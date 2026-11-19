"use client";

import React from "react";
import { OrderModeBadge } from "@hospitality/ui";
import type { KdsTicket } from "../lib/kds-client";

interface TicketCardProps {
  ticket: KdsTicket;
  nowMs: number;
  onToggleItem: (ticketId: string, itemId: string) => void;
  onStartPrep: (ticketId: string) => void;
  onBumpReady: (ticketId: string) => void;
}

function formatElapsed(ms: number): string {
  const totalSec = Math.max(0, Math.floor(ms / 1000));
  const mins = Math.floor(totalSec / 60);
  const secs = totalSec % 60;
  return `${mins}:${secs.toString().padStart(2, "0")}`;
}

export function TicketCard({
  ticket,
  nowMs,
  onToggleItem,
  onStartPrep,
  onBumpReady,
}: TicketCardProps) {
  const elapsedMs = nowMs - ticket.createdAtMs;
  const elapsedMin = elapsedMs / 60_000;

  // SLA color escalation: < 4m Emerald, 4-7m Amber, > 7m Rose pulsing
  const slaHeaderCls =
    elapsedMin >= 7
      ? "bg-rose-600 text-white animate-pulse"
      : elapsedMin >= 4
        ? "bg-amber-500 text-zinc-950"
        : "bg-emerald-600 text-white";

  const isPreparing = ticket.status === "preparing";

  return (
    <article className="flex flex-col overflow-hidden rounded-xl border border-zinc-800 bg-[#121215] shadow-lg">
      {/* SLA Header Bar */}
      <div className={`flex items-center justify-between px-3.5 py-2.5 ${slaHeaderCls}`}>
        <div className="flex items-center gap-2">
          <span className="font-mono text-base font-extrabold tracking-tight">
            {ticket.orderNumber}
          </span>
          {ticket.tableCode && (
            <span className="rounded bg-black/25 px-1.5 py-0.5 font-mono text-xs font-bold">
              TBL {ticket.tableCode}
            </span>
          )}
          <span className="rounded bg-black/20 px-1.5 py-0.5 font-mono text-[11px] font-semibold">
            R#{ticket.roundNumber}
          </span>
        </div>
        <span className="font-mono text-base font-extrabold tabular-nums">
          {formatElapsed(elapsedMs)}
        </span>
      </div>

      {/* Station & Mode Subheader */}
      <div className="flex items-center justify-between border-b border-zinc-800 bg-zinc-900/70 px-3.5 py-2">
        <OrderModeBadge mode={ticket.orderMode} size="sm" />
        <span className="font-mono text-[11px] font-bold uppercase tracking-wider text-zinc-400">
          {ticket.station.replace("_", " ")}
        </span>
      </div>

      {/* Items List (Tap to Strike-Through) */}
      <div className="flex-1 divide-y divide-zinc-800/80 px-3.5 py-2">
        {ticket.items.map((item) => (
          <button
            key={item.id}
            type="button"
            onClick={() => onToggleItem(ticket.id, item.id)}
            className={`flex w-full items-start gap-2.5 py-2.5 text-left transition-opacity ${
              item.completed ? "opacity-45 line-through" : "opacity-100"
            }`}
          >
            <span className="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-zinc-800 font-mono text-sm font-extrabold text-terracotta">
              {item.quantity}
            </span>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-bold text-zinc-100">{item.name}</p>
              {item.modifiers && item.modifiers.length > 0 && (
                <div className="mt-0.5 space-y-0.5">
                  {item.modifiers.map((mod) => (
                    <p key={mod} className="text-xs font-semibold text-amber-300">
                      + {mod}
                    </p>
                  ))}
                </div>
              )}
              {item.notes && (
                <p className="mt-1 rounded bg-rose-500/15 px-2 py-0.5 text-xs font-semibold text-rose-300">
                  NOTE: {item.notes}
                </p>
              )}
            </div>
          </button>
        ))}
      </div>

      {/* Action Footer: Start Prep (prep_confirmed_at) + Bump Ready */}
      <div className="grid grid-cols-2 gap-2 border-t border-zinc-800 bg-zinc-950/60 p-2.5">
        <button
          type="button"
          disabled={isPreparing}
          onClick={() => onStartPrep(ticket.id)}
          className={`rounded-lg py-2.5 text-xs font-bold uppercase tracking-wide transition-colors ${
            isPreparing
              ? "bg-emerald-500/15 text-emerald-400 cursor-default"
              : "bg-zinc-800 text-zinc-100 hover:bg-zinc-700"
          }`}
        >
          {isPreparing ? "Prep Confirmed ✓" : "Start Prep"}
        </button>
        <button
          type="button"
          onClick={() => onBumpReady(ticket.id)}
          className="rounded-lg bg-terracotta py-2.5 text-xs font-extrabold uppercase tracking-wide text-white transition-colors hover:bg-terracotta-700"
        >
          Bump Ready
        </button>
      </div>
    </article>
  );
}
