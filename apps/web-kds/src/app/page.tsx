"use client";

import React, { useEffect, useMemo, useRef, useState } from "react";
import { KdsHeader } from "../components/KdsHeader";
import { TicketCard } from "../components/TicketCard";
import {
  DEFAULT_LAN_URL,
  KdsLanClient,
  createPilotTickets,
  type KdsConnectionState,
  type KdsStation,
  type KdsTicket,
} from "../lib/kds-client";

export default function KdsBoardPage() {
  const [nowMs, setNowMs] = useState<number>(() => Date.now());
  const [tickets, setTickets] = useState<KdsTicket[]>([]);
  const [branchLabel, setBranchLabel] = useState<string>(
    process.env.NEXT_PUBLIC_BRANCH_NAME || "Branch LAN"
  );
  const [activeStation, setActiveStation] = useState<KdsStation>("all");
  const [connectionState, setConnectionState] =
    useState<KdsConnectionState>("demo_mode");
  const [recallOpen, setRecallOpen] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const clientRef = useRef<KdsLanClient | null>(null);
  const simCounterRef = useRef(500);

  // Opt-in demo tickets ONLY when ?demo=1 is explicitly in the URL
  useEffect(() => {
    if (typeof window !== "undefined" && new URLSearchParams(window.location.search).get("demo") === "1") {
      setTickets(createPilotTickets(Date.now()));
    }
  }, []);

  // 1-second SLA timer tick
  useEffect(() => {
    const id = setInterval(() => setNowMs(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  // Connect to Branch LAN SSE with automatic 2s safety polling + in-flight guard
  useEffect(() => {
    let isFetching = false;
    const fetchLiveTickets = async () => {
      if (isFetching) return;
      isFetching = true;
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 4000);
      try {
        const res = await fetch(`${DEFAULT_LAN_URL}/lan/kds/tickets`, {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!res.ok) return;
        const data = (await res.json()) as {
          tickets?: Array<{
            id: string;
            order_number?: string;
            station?: string;
            status?: string;
            created_at?: string;
            items_json?: string;
          }>;
        };
        if (Array.isArray(data.tickets)) {
          const mapped: KdsTicket[] = data.tickets.map((t) => {
            let parsedItems: Array<{ item_name?: string; name?: string; quantity?: string | number }> = [];
            try {
              parsedItems = t.items_json ? JSON.parse(t.items_json) : [];
            } catch {
              parsedItems = [];
            }
            const rawStation = (t.station || "barista").toLowerCase();
            const normalizedStation: KdsTicket["station"] =
              rawStation === "pastry"
                ? "cold_prep"
                : ["barista", "hot_kitchen", "cold_prep", "expediter"].includes(rawStation)
                  ? (rawStation as KdsTicket["station"])
                  : "barista";
            return {
              id: t.id,
              orderNumber: t.order_number || t.id.slice(0, 8),
              orderMode: "QUICK_SALE",
              roundNumber: 1,
              station: normalizedStation,
              status: (t.status as KdsTicket["status"]) || "queued",
              createdAtMs: t.created_at ? Date.parse(t.created_at) || Date.now() : Date.now(),
              items: parsedItems.map((it, i) => ({
                id: `${t.id}-${i}`,
                name: it.item_name || it.name || "Item",
                quantity: Number(it.quantity || 1),
              })),
            };
          });
          setTickets(mapped);
        }
      } catch {
        // Branch service unreachable or aborted — keep current state without injecting mock tickets
      } finally {
        clearTimeout(timeoutId);
        isFetching = false;
      }
    };

    const fetchBranchInfo = async () => {
      try {
        const res = await fetch(`${DEFAULT_LAN_URL}/healthz`, { cache: "no-store" });
        if (!res.ok) return;
        const h = (await res.json()) as { branch_id?: string };
        if (h.branch_id && !process.env.NEXT_PUBLIC_BRANCH_NAME) {
          setBranchLabel(`Branch ${h.branch_id.slice(0, 8)}`);
        }
      } catch {
        // Ignore if unreachable
      }
    };

    void fetchBranchInfo();
    void fetchLiveTickets();
    const pollId = setInterval(() => void fetchLiveTickets(), 2000);

    const client = new KdsLanClient();
    clientRef.current = client;
    const disconnect = client.connect({
      onStateChange: setConnectionState,
      onLanEvent: (_seq, eventType, payload) => {
        if (eventType === "order.upserted" || eventType === "kds.ticket_updated") {
          void fetchLiveTickets();
        }
        if (eventType === "kds.ticket_updated" && typeof payload.ticket_id === "string") {
          setTickets((prev) =>
            prev.map((t) =>
              t.id === payload.ticket_id
                ? {
                    ...t,
                    status: (payload.status as KdsTicket["status"]) ?? t.status,
                  }
                : t
            )
          );
        }
      },
    });
    const onFocusOrVisible = () => {
      if (document.visibilityState === "visible") {
        void fetchLiveTickets();
      }
    };
    window.addEventListener("focus", onFocusOrVisible);
    document.addEventListener("visibilitychange", onFocusOrVisible);

    return () => {
      window.removeEventListener("focus", onFocusOrVisible);
      document.removeEventListener("visibilitychange", onFocusOrVisible);
      clearInterval(pollId);
      disconnect();
    };
  }, []);

  // Keyboard shortcuts (1-5 station tabs, R for recall drawer)
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "1") setActiveStation("all");
      else if (e.key === "2") setActiveStation("barista");
      else if (e.key === "3") setActiveStation("hot_kitchen");
      else if (e.key === "4") setActiveStation("cold_prep");
      else if (e.key === "5") setActiveStation("expediter");
      else if (e.key.toLowerCase() === "r") setRecallOpen((prev) => !prev);
      else if (e.key === "Escape") setRecallOpen(false);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  const activeTickets = useMemo(
    () => tickets.filter((t) => t.status !== "bumped"),
    [tickets]
  );

  const bumpedTickets = useMemo(
    () => tickets.filter((t) => t.status === "bumped"),
    [tickets]
  );

  const countsByStation = useMemo<Record<KdsStation, number>>(() => {
    const counts: Record<KdsStation, number> = {
      all: activeTickets.length,
      barista: 0,
      hot_kitchen: 0,
      cold_prep: 0,
      expediter: 0,
    };
    for (const t of activeTickets) {
      counts[t.station] += 1;
    }
    return counts;
  }, [activeTickets]);

  const visibleTickets = useMemo(() => {
    const filtered =
      activeStation === "all"
        ? activeTickets
        : activeTickets.filter((t) => t.station === activeStation);
    return [...filtered].sort((a, b) => a.createdAtMs - b.createdAtMs);
  }, [activeTickets, activeStation]);

  function handleToggleItem(ticketId: string, itemId: string) {
    setTickets((prev) =>
      prev.map((t) =>
        t.id === ticketId
          ? {
              ...t,
              items: t.items.map((it) =>
                it.id === itemId ? { ...it, completed: !it.completed } : it
              ),
            }
          : t
      )
    );
  }

  async function persistTicketStatus(
    ticketId: string,
    status: "preparing" | "bumped" | "recalled"
  ) {
    setActionError(null);
    const res = await clientRef.current?.bumpTicket(ticketId, status);
    if (res && !res.ok) {
      setActionError(res.error || "Failed to update ticket on Branch LAN");
    }
  }

  function handleStartPrep(ticketId: string) {
    setTickets((prev) =>
      prev.map((t) =>
        t.id === ticketId
          ? { ...t, status: "preparing", prepConfirmedAtMs: Date.now() }
          : t
      )
    );
    void persistTicketStatus(ticketId, "preparing");
  }

  function handleBumpReady(ticketId: string) {
    setTickets((prev) =>
      prev.map((t) =>
        t.id === ticketId
          ? { ...t, status: "bumped", bumpedAtMs: Date.now() }
          : t
      )
    );
    void persistTicketStatus(ticketId, "bumped");
  }

  function handleRecall(ticketId: string) {
    setTickets((prev) =>
      prev.map((t) =>
        t.id === ticketId ? { ...t, status: "recalled" } : t
      )
    );
    void persistTicketStatus(ticketId, "recalled");
  }

  function handleSimulateTicket() {
    const num = ++simCounterRef.current;
    const newTicket: KdsTicket = {
      id: `tk-sim-${num}`,
      orderNumber: `F7-${num}`,
      orderMode: num % 2 === 0 ? "QR_ORDER" : "DRIVE_THRU",
      roundNumber: 1,
      station: num % 2 === 0 ? "barista" : "hot_kitchen",
      status: "queued",
      createdAtMs: Date.now(),
      items: [
        {
          id: `sim-it-${num}`,
          name: num % 2 === 0 ? "Large Oat Latte" : "Wood-Fired Margherita",
          quantity: 1,
          modifiers: num % 2 === 0 ? ["Extra Shot"] : ["Extra Basil"],
        },
      ],
    };
    setTickets((prev) => [...prev, newTicket]);
  }

  return (
    <div className="min-h-screen bg-[#09090B] text-zinc-100">
      <KdsHeader
        branchLabel={branchLabel}
        activeStation={activeStation}
        onSelectStation={setActiveStation}
        countsByStation={countsByStation}
        connectionState={connectionState}
        bumpedCount={bumpedTickets.length}
        onOpenRecall={() => setRecallOpen(true)}
        onSimulateTicket={handleSimulateTicket}
      />

      {actionError && (
        <div className="mx-4 mt-3 flex items-center justify-between rounded-lg border border-rose-500/40 bg-rose-500/15 px-3.5 py-2 text-xs text-rose-200">
          <span>{actionError}</span>
          <button
            type="button"
            onClick={() => setActionError(null)}
            className="ml-3 rounded bg-rose-500/20 px-2 py-0.5 font-semibold hover:bg-rose-500/30"
          >
            Dismiss
          </button>
        </div>
      )}

      <main className="p-4">
        {visibleTickets.length === 0 ? (
          <div className="flex h-[65vh] flex-col items-center justify-center rounded-xl border border-dashed border-zinc-800 text-center">
            <p className="text-lg font-bold text-zinc-300">All Caught Up</p>
            <p className="mt-1 text-xs text-zinc-500">
              No active tickets in this station. New LAN orders appear in &lt; 1s.
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {visibleTickets.map((ticket) => (
              <TicketCard
                key={ticket.id}
                ticket={ticket}
                nowMs={nowMs}
                onToggleItem={handleToggleItem}
                onStartPrep={handleStartPrep}
                onBumpReady={handleBumpReady}
              />
            ))}
          </div>
        )}
      </main>

      {/* Recall Drawer */}
      {recallOpen && (
        <div
          className="fixed inset-0 z-50 flex justify-end bg-black/60 backdrop-blur-xs"
          onClick={() => setRecallOpen(false)}
        >
          <aside
            className="flex h-full w-full max-w-md flex-col border-l border-zinc-800 bg-[#121215] p-5"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-zinc-800 pb-3">
              <h2 className="text-sm font-bold text-white">
                Recently Bumped Tickets ({bumpedTickets.length})
              </h2>
              <button
                type="button"
                onClick={() => setRecallOpen(false)}
                className="rounded px-2 py-1 font-mono text-xs text-zinc-400 hover:bg-zinc-800 hover:text-white"
              >
                ESC
              </button>
            </div>
            <div className="mt-4 flex-1 space-y-3 overflow-y-auto">
              {bumpedTickets.length === 0 ? (
                <p className="text-xs text-zinc-500">
                  No bumped tickets available to recall yet.
                </p>
              ) : (
                bumpedTickets.map((tk) => (
                  <div
                    key={tk.id}
                    className="flex items-center justify-between rounded-lg border border-zinc-800 bg-zinc-900/70 p-3"
                  >
                    <div>
                      <p className="font-mono text-sm font-bold text-white">
                        {tk.orderNumber}
                        {tk.tableCode ? ` · TBL ${tk.tableCode}` : ""}
                      </p>
                      <p className="text-xs text-zinc-400">
                        {tk.station.toUpperCase()} · {tk.items.length} items
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={() => handleRecall(tk.id)}
                      className="rounded-lg bg-amber-500 px-3 py-1.5 text-xs font-bold text-zinc-950 hover:bg-amber-400"
                    >
                      Recall
                    </button>
                  </div>
                ))
              )}
            </div>
          </aside>
        </div>
      )}
    </div>
  );
}
