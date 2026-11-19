"use client";

import React from "react";
import { ArrowDownRight, ArrowUpRight, Clock } from "lucide-react";
import { cn } from "../lib/cn";

export interface KpiMetricCardProps {
  title: string;
  value: string;
  deltaPercent: number;
  comparisonLabel: string;
  freshnessLabel: string;
  sparkline?: number[];
  className?: string;
}

function buildSparklinePoints(values: number[]): string {
  if (values.length < 2) {
    return "0,24 100,24";
  }
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  return values
    .map((val, idx) => {
      const x = (idx / (values.length - 1)) * 100;
      const y = 26 - ((val - min) / range) * 22;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
}

export function KpiMetricCard({
  title,
  value,
  deltaPercent,
  comparisonLabel,
  freshnessLabel,
  sparkline = [42, 48, 45, 53, 58, 54, 67, 72],
  className,
}: KpiMetricCardProps) {
  const isPositive = deltaPercent >= 0;
  const strokeColor = isPositive ? "#10B981" : "#E11D48";
  const points = buildSparklinePoints(sparkline);

  return (
    <div
      className={cn(
        "rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-4 shadow-card transition-colors hover:border-terracotta/40",
        className
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <p className="text-xs font-medium text-[var(--ink-muted)]">{title}</p>
        <span
          className={cn(
            "inline-flex items-center gap-0.5 rounded-full px-2 py-0.5 font-mono text-[11px] font-semibold",
            isPositive
              ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
              : "bg-rose-500/10 text-rose-700 dark:text-rose-300"
          )}
        >
          {isPositive ? (
            <ArrowUpRight className="h-3 w-3" />
          ) : (
            <ArrowDownRight className="h-3 w-3" />
          )}
          {isPositive ? "+" : ""}
          {deltaPercent.toFixed(1)}%
        </span>
      </div>

      <div className="mt-2 flex items-end justify-between gap-3">
        <div>
          <p className="font-mono text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
            {value}
          </p>
          <p className="mt-0.5 text-[11px] text-[var(--ink-muted)]">
            {comparisonLabel}
          </p>
        </div>

        <svg
          viewBox="0 0 100 30"
          className="h-8 w-24 shrink-0 overflow-visible"
          aria-hidden="true"
        >
          <polyline
            fill="none"
            stroke={strokeColor}
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            points={points}
          />
        </svg>
      </div>

      <div className="mt-3 flex items-center gap-1 border-t border-[var(--surface-border)] pt-2 text-[11px] text-[var(--ink-muted)]">
        <Clock className="h-3 w-3" />
        <span>Updated {freshnessLabel}</span>
      </div>
    </div>
  );
}
