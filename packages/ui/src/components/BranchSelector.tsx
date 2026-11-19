"use client";

import React, { useEffect, useRef, useState } from "react";
import { Building2, Check, ChevronDown, MapPin } from "lucide-react";
import { cn } from "../lib/cn";
import type { SyncStatus } from "./SyncStatusBadge";

export interface BranchOption {
  id: string;
  organizationName: string;
  branchName: string;
  code: string;
  city: string;
  syncStatus: SyncStatus;
  pendingSyncCount: number;
  lastHeartbeat: string;
}

export interface BranchSelectorProps {
  branches: BranchOption[];
  selectedBranchId: string;
  onSelectBranch: (branchId: string) => void;
  className?: string;
}

const HEARTBEAT_DOT: Record<SyncStatus, string> = {
  online: "bg-emerald-500",
  lan: "bg-amber-500",
  isolated: "bg-rose-600",
};

export function BranchSelector({
  branches,
  selectedBranchId,
  onSelectBranch,
  className,
}: BranchSelectorProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const activeBranch =
    branches.find((b) => b.id === selectedBranchId) ?? branches[0];

  useEffect(() => {
    function handleOutsideClick(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setOpen(false);
      }
    }
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleOutsideClick);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleOutsideClick);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, []);

  if (!activeBranch) {
    return null;
  }

  return (
    <div ref={containerRef} className={cn("relative", className)}>
      <button
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((prev) => !prev)}
        className="flex items-center gap-2.5 rounded-lg border border-[var(--surface-border)] bg-[var(--surface-card)] px-3 py-1.5 text-left shadow-card transition-colors hover:border-terracotta/50 focus:outline-none focus:ring-2 focus:ring-terracotta/40"
      >
        <span className="relative flex h-2.5 w-2.5 shrink-0">
          <span
            className={cn(
              "inline-flex h-2.5 w-2.5 rounded-full",
              HEARTBEAT_DOT[activeBranch.syncStatus]
            )}
          />
        </span>
        <div className="min-w-0">
          <div className="flex items-center gap-1.5">
            <span className="truncate text-xs font-semibold text-[var(--ink-primary)]">
              {activeBranch.branchName}
            </span>
            <span className="rounded bg-[var(--surface-subtle)] px-1.5 py-0.5 font-mono text-[10px] font-medium text-[var(--ink-muted)]">
              {activeBranch.code}
            </span>
          </div>
          <p className="truncate text-[11px] text-[var(--ink-muted)]">
            {activeBranch.organizationName}
          </p>
        </div>
        <ChevronDown
          className={cn(
            "ml-1 h-4 w-4 shrink-0 text-[var(--ink-muted)] transition-transform",
            open && "rotate-180"
          )}
        />
      </button>

      {open && (
        <div
          role="listbox"
          aria-label="Select organization branch"
          className="absolute left-0 z-50 mt-2 w-80 rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-1.5 shadow-elevated"
        >
          <div className="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-[var(--ink-muted)]">
            <Building2 className="h-3.5 w-3.5" />
            Multi-Tenant Branches
          </div>
          <div className="max-h-72 space-y-1 overflow-y-auto">
            {branches.map((branch) => {
              const isSelected = branch.id === activeBranch.id;
              return (
                <button
                  key={branch.id}
                  type="button"
                  role="option"
                  aria-selected={isSelected}
                  onClick={() => {
                    onSelectBranch(branch.id);
                    setOpen(false);
                  }}
                  className={cn(
                    "flex w-full items-start justify-between gap-3 rounded-lg px-2.5 py-2 text-left transition-colors",
                    isSelected
                      ? "bg-terracotta/10 text-[var(--ink-primary)]"
                      : "hover:bg-[var(--surface-subtle)]"
                  )}
                >
                  <div className="flex items-start gap-2.5 min-w-0">
                    <span
                      className={cn(
                        "mt-1.5 h-2 w-2 shrink-0 rounded-full",
                        HEARTBEAT_DOT[branch.syncStatus]
                      )}
                    />
                    <div className="min-w-0">
                      <div className="flex items-center gap-1.5">
                        <span className="truncate text-xs font-semibold text-[var(--ink-primary)]">
                          {branch.branchName}
                        </span>
                        <span className="font-mono text-[10px] text-[var(--ink-muted)]">
                          {branch.code}
                        </span>
                      </div>
                      <p className="truncate text-[11px] text-[var(--ink-muted)]">
                        {branch.organizationName}
                      </p>
                      <div className="mt-0.5 flex items-center gap-2 text-[10px] text-[var(--ink-muted)]">
                        <span className="inline-flex items-center gap-0.5">
                          <MapPin className="h-2.5 w-2.5" />
                          {branch.city}
                        </span>
                        <span>·</span>
                        <span>Heartbeat {branch.lastHeartbeat}</span>
                      </div>
                    </div>
                  </div>
                  {isSelected && (
                    <Check className="mt-1 h-4 w-4 shrink-0 text-terracotta" />
                  )}
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
