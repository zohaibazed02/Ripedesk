"use client";

import React, { useEffect, useState } from "react";
import { Command, Menu, Search, X } from "lucide-react";
import { cn } from "../lib/cn";

export interface AppShellNavItem {
  id: string;
  label: string;
  icon: React.ReactNode;
  badge?: string;
  active?: boolean;
  onClick?: () => void;
}

export interface CommandPaletteAction {
  id: string;
  label: string;
  group: string;
  shortcut?: string;
  onSelect: () => void;
}

export interface AppShellProps {
  brandName: string;
  brandSubtitle?: string;
  navItems: AppShellNavItem[];
  topBarLeft?: React.ReactNode;
  topBarRight?: React.ReactNode;
  commandActions?: CommandPaletteAction[];
  children: React.ReactNode;
}

export function AppShell({
  brandName,
  brandSubtitle = "Enterprise POS & ERP",
  navItems,
  topBarLeft,
  topBarRight,
  commandActions = [],
  children,
}: AppShellProps) {
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [query, setQuery] = useState("");

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setPaletteOpen((prev) => !prev);
      } else if (event.key === "Escape") {
        setPaletteOpen(false);
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  const filteredActions = commandActions.filter((action) =>
    `${action.label} ${action.group}`.toLowerCase().includes(query.toLowerCase())
  );

  return (
    <div className="flex min-h-screen bg-[var(--surface-canvas)] text-[var(--ink-primary)]">
      {/* Desktop Sidebar */}
      <aside className="hidden w-64 shrink-0 flex-col border-r border-[var(--surface-border)] bg-[var(--surface-card)] lg:flex">
        <div className="flex h-16 items-center gap-2.5 border-b border-[var(--surface-border)] px-5">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-espresso-900 text-white dark:bg-terracotta">
            <span className="font-mono text-sm font-bold">HO</span>
          </div>
          <div className="min-w-0">
            <p className="truncate text-sm font-bold tracking-tight text-[var(--ink-primary)]">
              {brandName}
            </p>
            <p className="truncate text-[11px] text-[var(--ink-muted)]">
              {brandSubtitle}
            </p>
          </div>
        </div>

        <nav className="flex-1 space-y-1 p-3" aria-label="Primary Navigation">
          {navItems.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={item.onClick}
              className={cn(
                "flex w-full items-center justify-between rounded-lg px-3 py-2 text-xs font-medium transition-colors",
                item.active
                  ? "bg-espresso-900 text-white dark:bg-terracotta dark:text-white"
                  : "text-[var(--ink-secondary)] hover:bg-[var(--surface-subtle)] hover:text-[var(--ink-primary)]"
              )}
            >
              <span className="flex items-center gap-2.5">
                {item.icon}
                <span>{item.label}</span>
              </span>
              {item.badge && (
                <span
                  className={cn(
                    "rounded-full px-1.5 py-0.5 font-mono text-[10px]",
                    item.active
                      ? "bg-white/20 text-white"
                      : "bg-[var(--surface-subtle)] text-[var(--ink-muted)]"
                  )}
                >
                  {item.badge}
                </span>
              )}
            </button>
          ))}
        </nav>

        <div className="border-t border-[var(--surface-border)] p-3">
          <button
            type="button"
            onClick={() => setPaletteOpen(true)}
            className="flex w-full items-center justify-between rounded-lg border border-[var(--surface-border)] bg-[var(--surface-subtle)] px-3 py-2 text-xs text-[var(--ink-muted)] transition-colors hover:border-terracotta/50 hover:text-[var(--ink-primary)]"
          >
            <span className="flex items-center gap-2">
              <Search className="h-3.5 w-3.5" />
              <span>Quick Command...</span>
            </span>
            <kbd className="inline-flex items-center gap-0.5 rounded border border-[var(--surface-border)] bg-[var(--surface-card)] px-1.5 py-0.5 font-mono text-[10px]">
              <Command className="h-2.5 w-2.5" />K
            </kbd>
          </button>
        </div>
      </aside>

      {/* Mobile Drawer */}
      {mobileNavOpen && (
        <div className="fixed inset-0 z-50 flex lg:hidden">
          <div
            className="fixed inset-0 bg-black/50 backdrop-blur-xs"
            onClick={() => setMobileNavOpen(false)}
          />
          <aside className="relative z-10 flex w-64 flex-col bg-[var(--surface-card)] shadow-elevated">
            <div className="flex h-16 items-center justify-between border-b border-[var(--surface-border)] px-4">
              <span className="text-sm font-bold">{brandName}</span>
              <button
                type="button"
                aria-label="Close navigation"
                onClick={() => setMobileNavOpen(false)}
                className="rounded-md p-1.5 text-[var(--ink-muted)] hover:bg-[var(--surface-subtle)]"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <nav className="flex-1 space-y-1 p-3">
              {navItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => {
                    item.onClick?.();
                    setMobileNavOpen(false);
                  }}
                  className={cn(
                    "flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-xs font-medium",
                    item.active
                      ? "bg-espresso-900 text-white dark:bg-terracotta"
                      : "text-[var(--ink-secondary)] hover:bg-[var(--surface-subtle)]"
                  )}
                >
                  {item.icon}
                  <span>{item.label}</span>
                </button>
              ))}
            </nav>
          </aside>
        </div>
      )}

      {/* Main Content Area */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-16 items-center justify-between gap-3 border-b border-[var(--surface-border)] bg-[var(--surface-card)]/90 px-4 backdrop-blur-md lg:px-6">
          <div className="flex items-center gap-3 min-w-0">
            <button
              type="button"
              aria-label="Open navigation menu"
              onClick={() => setMobileNavOpen(true)}
              className="rounded-lg border border-[var(--surface-border)] p-2 text-[var(--ink-secondary)] hover:bg-[var(--surface-subtle)] lg:hidden"
            >
              <Menu className="h-4 w-4" />
            </button>
            {topBarLeft}
          </div>

          <div className="flex items-center gap-2.5">
            <button
              type="button"
              onClick={() => setPaletteOpen(true)}
              className="hidden items-center gap-2 rounded-lg border border-[var(--surface-border)] bg-[var(--surface-subtle)] px-2.5 py-1.5 text-xs text-[var(--ink-muted)] transition-colors hover:border-terracotta/50 sm:inline-flex"
            >
              <Search className="h-3.5 w-3.5" />
              <span className="hidden md:inline">Search</span>
              <kbd className="rounded bg-[var(--surface-card)] px-1 font-mono text-[10px]">
                ⌘K
              </kbd>
            </button>
            {topBarRight}
          </div>
        </header>

        <main className="flex-1 p-4 lg:p-6">{children}</main>
      </div>

      {/* Command Palette Modal */}
      {paletteOpen && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Command palette"
          className="fixed inset-0 z-50 flex items-start justify-center bg-black/50 p-4 pt-24 backdrop-blur-xs"
          onClick={() => setPaletteOpen(false)}
        >
          <div
            className="w-full max-w-lg overflow-hidden rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] shadow-elevated"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center gap-2.5 border-b border-[var(--surface-border)] px-4 py-3">
              <Search className="h-4 w-4 text-[var(--ink-muted)]" />
              <input
                type="text"
                autoFocus
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Jump to branch, toggle theme, or simulate LAN mode..."
                className="w-full bg-transparent text-sm text-[var(--ink-primary)] placeholder:text-[var(--ink-muted)] focus:outline-none"
              />
              <button
                type="button"
                onClick={() => setPaletteOpen(false)}
                className="rounded px-1.5 py-0.5 font-mono text-[10px] text-[var(--ink-muted)] hover:bg-[var(--surface-subtle)]"
              >
                ESC
              </button>
            </div>
            <div className="max-h-72 overflow-y-auto p-2">
              {filteredActions.length === 0 ? (
                <p className="px-3 py-6 text-center text-xs text-[var(--ink-muted)]">
                  No matching commands found.
                </p>
              ) : (
                filteredActions.map((action) => (
                  <button
                    key={action.id}
                    type="button"
                    onClick={() => {
                      action.onSelect();
                      setPaletteOpen(false);
                      setQuery("");
                    }}
                    className="flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-xs transition-colors hover:bg-terracotta/10"
                  >
                    <div>
                      <p className="font-medium text-[var(--ink-primary)]">
                        {action.label}
                      </p>
                      <p className="text-[11px] text-[var(--ink-muted)]">
                        {action.group}
                      </p>
                    </div>
                    {action.shortcut && (
                      <kbd className="rounded border border-[var(--surface-border)] bg-[var(--surface-subtle)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--ink-muted)]">
                        {action.shortcut}
                      </kbd>
                    )}
                  </button>
                ))
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
