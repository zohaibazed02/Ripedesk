import type { OrderMode } from "@hospitality/ui";

export type KdsStation = "all" | "barista" | "hot_kitchen" | "cold_prep" | "expediter";
export type TicketStatus = "queued" | "preparing" | "bumped" | "recalled";
export type KdsConnectionState = "sse_live" | "polling_fallback" | "demo_mode";

export interface KdsTicketItem {
  id: string;
  name: string;
  quantity: number;
  modifiers?: string[];
  notes?: string;
  completed?: boolean;
}

export interface KdsTicket {
  id: string;
  orderNumber: string;
  orderMode: OrderMode;
  tableCode?: string;
  roundNumber: number;
  station: Exclude<KdsStation, "all">;
  status: TicketStatus;
  createdAtMs: number;
  prepConfirmedAtMs?: number;
  bumpedAtMs?: number;
  items: KdsTicketItem[];
}

export const DEFAULT_LAN_URL =
  process.env.NEXT_PUBLIC_BRANCH_LAN_URL || "http://localhost:8090";

export function createPilotTickets(nowMs: number): KdsTicket[] {
  return [
    {
      id: "tk-bar-101",
      orderNumber: "F7-1042",
      orderMode: "QUICK_SALE",
      roundNumber: 1,
      station: "barista",
      status: "preparing",
      createdAtMs: nowMs - 145_000, // 2m 25s (Emerald)
      prepConfirmedAtMs: nowMs - 110_000,
      items: [
        {
          id: "it-1",
          name: "Large Oat Latte",
          quantity: 2,
          modifiers: ["Extra Shot", "Iced"],
          notes: "Light ice",
        },
        {
          id: "it-2",
          name: "Flat White",
          quantity: 1,
          modifiers: ["Almond Milk"],
        },
      ],
    },
    {
      id: "tk-hot-202",
      orderNumber: "G9-208",
      orderMode: "DINE_IN",
      tableCode: "T2",
      roundNumber: 2,
      station: "hot_kitchen",
      status: "queued",
      createdAtMs: nowMs - 310_000, // 5m 10s (Amber warning)
      items: [
        {
          id: "it-3",
          name: "Truffle Mushroom Pasta",
          quantity: 1,
          notes: "No parmesan on top",
        },
        {
          id: "it-4",
          name: "Wood-Fired Margherita",
          quantity: 2,
          modifiers: ["Extra Basil"],
        },
      ],
    },
    {
      id: "tk-cold-303",
      orderNumber: "G9-205",
      orderMode: "DELIVERY",
      roundNumber: 1,
      station: "cold_prep",
      status: "queued",
      createdAtMs: nowMs - 475_000, // 7m 55s (Rose overdue!)
      items: [
        {
          id: "it-5",
          name: "Burrata Salad",
          quantity: 1,
          notes: "Dressing on side",
        },
        {
          id: "it-6",
          name: "Classic Espresso Tiramisu",
          quantity: 2,
        },
      ],
    },
    {
      id: "tk-expo-404",
      orderNumber: "G9-209",
      orderMode: "TAKEAWAY",
      roundNumber: 1,
      station: "expediter",
      status: "preparing",
      createdAtMs: nowMs - 90_000, // 1m 30s (Emerald)
      prepConfirmedAtMs: nowMs - 60_000,
      items: [
        {
          id: "it-7",
          name: "Butter Croissant",
          quantity: 3,
          notes: "Warm slightly",
        },
      ],
    },
  ];
}

export class KdsLanClient {
  private baseUrl: string;
  private eventSource: EventSource | null = null;
  private pollTimer: ReturnType<typeof setInterval> | null = null;
  private latestLanSeq = 0;

  constructor(baseUrl: string = DEFAULT_LAN_URL) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
  }

  connect(callbacks: {
    onStateChange: (state: KdsConnectionState) => void;
    onLanEvent: (seqId: number, eventType: string, payload: Record<string, unknown>) => void;
  }): () => void {
    if (typeof window === "undefined" || typeof EventSource === "undefined") {
      callbacks.onStateChange("demo_mode");
      return () => {};
    }

    const startPolling = () => {
      if (this.pollTimer) return;
      callbacks.onStateChange("polling_fallback");
      this.pollTimer = setInterval(async () => {
        try {
          const resp = await fetch(`${this.baseUrl}/lan/events?since_seq=${this.latestLanSeq}`);
          if (!resp.ok) {
            callbacks.onStateChange("demo_mode");
            return;
          }
          const body = (await resp.json()) as {
            events?: Array<{ seq_id: number; event_type: string; payload_json: string }>;
          };
          for (const ev of body.events ?? []) {
            if (ev.seq_id > this.latestLanSeq) {
              this.latestLanSeq = ev.seq_id;
            }
            try {
              callbacks.onLanEvent(ev.seq_id, ev.event_type, JSON.parse(ev.payload_json));
            } catch {
              // Ignore malformed event payload
            }
          }
        } catch {
          callbacks.onStateChange("demo_mode");
        }
      }, 3000);
    };

    try {
      const es = new EventSource(`${this.baseUrl}/lan/stream`);
      this.eventSource = es;

      es.onopen = () => {
        if (this.pollTimer) {
          clearInterval(this.pollTimer);
          this.pollTimer = null;
        }
        callbacks.onStateChange("sse_live");
      };

      es.onerror = () => {
        startPolling();
      };

      es.onmessage = (msg) => {
        try {
          const ev = JSON.parse(msg.data) as {
            seq_id?: number;
            event_type?: string;
            payload_json?: string;
          };
          if (ev.seq_id && ev.seq_id > this.latestLanSeq) {
            this.latestLanSeq = ev.seq_id;
          }
          if (ev.event_type && ev.payload_json) {
            callbacks.onLanEvent(ev.seq_id ?? 0, ev.event_type, JSON.parse(ev.payload_json));
          }
        } catch {
          // Keep-alive or non-JSON frame
        }
      };
    } catch {
      callbacks.onStateChange("demo_mode");
    }

    return () => {
      if (this.eventSource) {
        this.eventSource.close();
        this.eventSource = null;
      }
      if (this.pollTimer) {
        clearInterval(this.pollTimer);
        this.pollTimer = null;
      }
    };
  }

  async bumpTicket(ticketId: string, status: Exclude<TicketStatus, "queued">): Promise<void> {
    try {
      await fetch(`${this.baseUrl}/lan/commands/kds/bump`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          ticket_id: ticketId,
          status,
          idempotency_key: `kds-${ticketId}-${status}-${Date.now()}`,
        }),
      });
    } catch {
      // Demo / offline local state handles optimistic update cleanly
    }
  }
}
