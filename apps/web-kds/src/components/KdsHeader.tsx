"use client";

import React from "react";
import type { KdsConnectionState, KdsStation } from "../lib/kds-client";

interface KdsHeaderProps {
  activeStation: KdsStation;
  onSelectStation: (station: KdsStation) => void;
  countsByStation: Record<KdsStation, number>;
  connectionState: KdsConnectionState;
  bumpedCount: number;
  onOpenRecall: () => void;
  onSimulateTicket: () => void;
}

const STATIONS: Array<{ id: KdsStation; label: string; keyHint: string }> = [
  { id: "all", label: "All Stations", keyHint: "1" },
  { id: "barista", label: "Barista", keyHint: "2" },
  { id: "hot_kitchen", label: "Hot Kitchen", keyHint: "3" },
  { id: "cold_prep", label: "Cold Prep", keyHint: "4" },
  { id: "expediter", label: "Expo / Pass", keyHint: "5" },
];

export function KdsHeader({
  activeStation,
  onSelectStation,
  countsByStation,
  connectionState,
  bumpedCount,
  onOpenRecall,
  onSimulateTicket,
}: KdsHeaderProps) {
  const connBadge =
    connectionState === "sse_live"
      ? {
          label: "LAN SSE Live (<1s)",
          cls: "border-emerald-500/30 bg-emerald-500/15 text-emerald-300",
          dot: "bg-emerald-400",
        }
      : connectionState === "polling_fallback"
        ? {
            label: "Reconnecting (Polling Fallback)",
            cls: "border-amber-500/35 bg-amber-500/15 text-amber-300",
            dot: "bg-amber-400",
          }
        : {
            label: "Live Pilot Demo Mode",
            cls: "border-sky-500/35 bg-sky-500/15 text-sky-300",
            dot: "bg-sky-400",
          };

  return (
    <header className="sticky top-0 z-30 border-b border-zinc-800 bg-zinc-950/95 px-4 py-3 backdrop-blur-md">
      <div className="flex flex-wrap items-center justify-between gap-3">
        {/* Brand + Station Tabs */}
        <div className="flex flex-wrap items-center gap-2">
          <div className="mr-2 flex items-center gap-2">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-terracotta font-mono text-xs font-extrabold text-white">
              KDS
            </span>
            <div>
              <p className="text-xs font-bold tracking-tight text-white">
                Kitchen Display
              </p>
              <p className="font-mono text-[10px] text-zinc-400">
                G-9 Flagship · LAN
              </p>
            </div>
          </div>

          {STATIONS.map((st) => {
            const active = activeStation === st.id;
            return (
              <button
                key={st.id}
                type="button"
                onClick={() => onSelectStation(st.id)}
                className={`inline-flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-semibold transition-colors ${
                  active
                    ? "border-terracotta bg-terracotta text-white"
                    : "border-zinc-800 bg-zinc-900 text-zinc-300 hover:border-zinc-700 hover:text-white"
                }`}
              >
                <span>{st.label}</span>
                <span
                  className={`rounded-full px-1.5 py-0.5 font-mono text-[10px] ${
                    active ? "bg-black/25 text-white" : "bg-zinc-800 text-zinc-400"
                  }`}
                >
                  {countsByStation[st.id] ?? 0}
                </span>
              </button>
            );
          })}
        </div>

        {/* Right Controls: Sync Pill, Simulate, Recall */}
        <div className="flex items-center gap-2">
          <span
            className={`inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-semibold ${connBadge.cls}`}
          >
            <span className={`h-2 w-2 rounded-full animate-pulse ${connBadge.dot}`} />
            {connBadge.label}
          </span>

          <button
            type="button"
            onClick={onSimulateTicket}
            className="rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-xs font-semibold text-zinc-200 hover:border-terracotta hover:text-white"
          >
            + Simulate Ticket
          </button>

          <button
            type="button"
            onClick={onOpenRecall}
            className="inline-flex items-center gap-1.5 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-xs font-semibold text-zinc-200 hover:border-amber-500 hover:text-amber-300"
          >
            <span>Recall (R)</span>
            <span className="rounded bg-zinc-800 px-1.5 py-0.5 font-mono text-[10px]">
              {bumpedCount}
            </span>
          </button>
        </div>
      </div>
    </header>
  );
}
