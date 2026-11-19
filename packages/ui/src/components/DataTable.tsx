"use client";

import React, { useMemo, useState } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { cn } from "../lib/cn";

export interface DataTableColumn<T> {
  key: keyof T & string;
  header: string;
  align?: "left" | "right" | "center";
  sortable?: boolean;
  render?: (row: T) => React.ReactNode;
}

export interface DataTableProps<T extends { id: string }> {
  columns: DataTableColumn<T>[];
  data: T[];
  caption?: string;
  onRowActivate?: (row: T) => void;
  className?: string;
}

export function DataTable<T extends { id: string }>({
  columns,
  data,
  caption,
  onRowActivate,
  className,
}: DataTableProps<T>) {
  const [sortKey, setSortKey] = useState<(keyof T & string) | null>(null);
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");
  const [focusedIndex, setFocusedIndex] = useState<number>(0);

  const sortedData = useMemo(() => {
    if (!sortKey) return data;
    return [...data].sort((a, b) => {
      const aVal = a[sortKey];
      const bVal = b[sortKey];
      if (aVal === bVal) return 0;
      const comparison = String(aVal).localeCompare(String(bVal), undefined, {
        numeric: true,
      });
      return sortDir === "asc" ? comparison : -comparison;
    });
  }, [data, sortKey, sortDir]);

  function toggleSort(key: keyof T & string) {
    if (sortKey === key) {
      setSortDir((prev) => (prev === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLTableSectionElement>) {
    if (sortedData.length === 0) return;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setFocusedIndex((prev) => Math.min(prev + 1, sortedData.length - 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setFocusedIndex((prev) => Math.max(prev - 1, 0));
    } else if (event.key === "Enter" && onRowActivate) {
      event.preventDefault();
      const active = sortedData[focusedIndex];
      if (active) onRowActivate(active);
    }
  }

  return (
    <div
      className={cn(
        "overflow-hidden rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] shadow-card",
        className
      )}
    >
      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-left text-xs">
          {caption && (
            <caption className="sr-only">{caption}</caption>
          )}
          <thead>
            <tr className="border-b border-[var(--surface-border)] bg-[var(--surface-subtle)] text-[11px] font-semibold uppercase tracking-wider text-[var(--ink-muted)]">
              {columns.map((col) => (
                <th
                  key={col.key}
                  scope="col"
                  className={cn(
                    "px-3.5 py-2.5 whitespace-nowrap",
                    col.align === "right" && "text-right",
                    col.align === "center" && "text-center"
                  )}
                >
                  {col.sortable ? (
                    <button
                      type="button"
                      onClick={() => toggleSort(col.key)}
                      className={cn(
                        "inline-flex items-center gap-1 hover:text-[var(--ink-primary)]",
                        col.align === "right" && "flex-row-reverse"
                      )}
                    >
                      <span>{col.header}</span>
                      {sortKey === col.key ? (
                        sortDir === "asc" ? (
                          <ArrowUp className="h-3 w-3 text-terracotta" />
                        ) : (
                          <ArrowDown className="h-3 w-3 text-terracotta" />
                        )
                      ) : (
                        <ArrowUpDown className="h-3 w-3 opacity-50" />
                      )}
                    </button>
                  ) : (
                    col.header
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody
            tabIndex={0}
            onKeyDown={handleKeyDown}
            aria-label="Keyboard navigable ledger rows"
            className="divide-y divide-[var(--surface-border)] focus:outline-none"
          >
            {sortedData.map((row, idx) => {
              const isFocused = idx === focusedIndex;
              return (
                <tr
                  key={row.id}
                  onClick={() => {
                    setFocusedIndex(idx);
                    onRowActivate?.(row);
                  }}
                  className={cn(
                    "cursor-pointer transition-colors",
                    isFocused
                      ? "bg-terracotta/10"
                      : "hover:bg-[var(--surface-subtle)]/70"
                  )}
                >
                  {columns.map((col) => (
                    <td
                      key={col.key}
                      className={cn(
                        "px-3.5 py-2.5 text-[var(--ink-primary)] whitespace-nowrap",
                        col.align === "right" && "text-right font-mono",
                        col.align === "center" && "text-center"
                      )}
                    >
                      {col.render ? col.render(row) : String(row[col.key])}
                    </td>
                  ))}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <div className="flex items-center justify-between border-t border-[var(--surface-border)] bg-[var(--surface-subtle)]/50 px-3.5 py-1.5 text-[11px] text-[var(--ink-muted)]">
        <span>{sortedData.length} records</span>
        <span className="font-mono">↑/↓ Navigate · Enter Inspect</span>
      </div>
    </div>
  );
}
