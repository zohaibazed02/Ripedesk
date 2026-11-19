"use client";

import React from "react";
import { Cloud, WifiOff, AlertTriangle, RefreshCw } from "lucide-react";
import { cn } from "../lib/cn";

export type SyncStatus = "online" | "lan" | "isolated";

export interface SyncStatusBadgeProps {
  status: SyncStatus;
  pendingSyncCount?: number;
  lastSyncedLabel?: string;
  compact?: boolean;
  className?: string;
}

const STATUS_META: Record<
  SyncStatus,
  {
    label: string;
    shortLabel: string;
    dotClass: string;
    badgeClass: string;
    icon: React.ComponentType<{ className?: string }>;
  }
> = {
  online: {
    label: "Online (Synced)",
    shortLabel: "Online",
    dotClass: "bg-emerald-500",
    badgeClass:
      "border-emerald-500/25 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
    icon: Cloud,
  },
  lan: {
    label: "LAN Mode (Offline Internet)",
    shortLabel: "LAN Mode",
    dotClass: "bg-amber-500",
    badgeClass:
      "border-amber-500/30 bg-amber-500/10 text-amber-800 dark:text-amber-300",
    icon: WifiOff,
  },
  isolated: {
    label: "Isolated Terminal",
    shortLabel: "Isolated",
    dotClass: "bg-rose-600",
    badgeClass:
      "border-rose-500/30 bg-rose-500/10 text-rose-700 dark:text-rose-300",
    icon: AlertTriangle,
  },
};

export function SyncStatusBadge({
  status,
  pendingSyncCount = 0,
  lastSyncedLabel,
  compact = false,
  className,
}: SyncStatusBadgeProps) {
  const meta = STATUS_META[status];
  const Icon = meta.icon;

  return (
    <div
      role="status"
      aria-live="polite"
      className={cn(
        "inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-medium transition-colors",
        meta.badgeClass,
        className
      )}
    >
      <span className="relative flex h-2 w-2 shrink-0">
        <span
          className={cn(
            "absolute inline-flex h-full w-full rounded-full opacity-75 animate-ping",
            meta.dotClass
          )}
        />
        <span
          className={cn(
            "relative inline-flex h-2 w-2 rounded-full",
            meta.dotClass
          )}
        />
      </span>

      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className="whitespace-nowrap">
        {compact ? meta.shortLabel : meta.label}
      </span>

      {pendingSyncCount > 0 && (
        <span className="inline-flex items-center gap-1 rounded-full bg-espresso-900/10 px-2 py-0.5 font-mono text-[11px] font-semibold dark:bg-white/10">
          <RefreshCw className="h-2.5 w-2.5 animate-spin" />
          {pendingSyncCount} pending
        </span>
      )}

      {!compact && lastSyncedLabel && (
        <span className="hidden text-[11px] opacity-75 sm:inline">
          · {lastSyncedLabel}
        </span>
      )}
    </div>
  );
}
