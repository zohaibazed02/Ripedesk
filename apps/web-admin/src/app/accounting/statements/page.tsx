"use client";

import React, { useState } from "react";
import { BookOpen, Download, FileSpreadsheet, Landmark, LayoutDashboard } from "lucide-react";
import {
  AppShell,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
} from "@hospitality/ui";

type SuiteTab = "aging" | "bank" | "statements";
type StatementSubTab = "pnl" | "balance_sheet" | "trial_balance";

export default function FinancialStatementsPage() {
  const [tab, setTab] = useState<SuiteTab>("statements");
  const [stmtTab, setStmtTab] = useState<StatementSubTab>("pnl");
  const [bankFinalized, setBankFinalized] = useState(false);

  function exportCsv() {
    const csv = "Line,Amount_PKR\nNet Revenue,148200.00\nDirect Costs (COGS + Waste),40200.00\nGross Profit,108000.00\nOperating Expenses,4850.00\nNet Operating Income,103150.00\n";
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "artisan-roast-financial-statement.csv";
    a.click();
    URL.revokeObjectURL(url);
  }

  const navItems: AppShellNavItem[] = [
    {
      id: "home",
      label: "Design System & Health",
      icon: <LayoutDashboard className="h-4 w-4" />,
      onClick: () => {
        window.location.href = "/";
      },
    },
    {
      id: "daily-close",
      label: "Accounting · Daily Close & GL",
      icon: <BookOpen className="h-4 w-4" />,
      onClick: () => {
        window.location.href = "/accounting/daily-close";
      },
    },
    {
      id: "statements",
      label: "Accounting · Statements & Aging",
      icon: <FileSpreadsheet className="h-4 w-4" />,
      active: true,
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Financial Reporting Suite"
      navItems={navItems}
      topBarLeft={
        <h1 className="text-sm font-bold text-[var(--ink-primary)]">
          Financial Statements, Supplier Aging &amp; Bank Reconciliation
        </h1>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="Assets == L + E ✓" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="Total Accounts Payable"
            value="PKR 93,800"
            deltaPercent={-4.5}
            comparisonLabel="1 overdue (1-30d)"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Net Operating Margin"
            value="69.6%"
            deltaPercent={2.4}
            comparisonLabel="Food Cost 27.1%"
            freshnessLabel="MTD"
          />
          <KpiMetricCard
            title="Unreconciled Bank Diff"
            value="PKR 0.00"
            deltaPercent={0}
            comparisonLabel="1040 Main Bank"
            freshnessLabel="matched"
          />
          <KpiMetricCard
            title="Accounting Equation"
            value="Balanced ✓"
            deltaPercent={0}
            comparisonLabel="Assets == Liab + Equity"
            freshnessLabel="verified"
          />
        </div>

        {/* 3-Tab Switcher */}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex gap-2">
            {(
              [
                ["statements", "Financial Statements"],
                ["aging", "Supplier Aging & AP"],
                ["bank", "Bank Reconciliation"],
              ] as const
            ).map(([id, label]) => (
              <button
                key={id}
                type="button"
                onClick={() => setTab(id)}
                className={`rounded-lg border px-3.5 py-2 text-xs font-semibold ${
                  tab === id
                    ? "border-terracotta bg-terracotta text-white"
                    : "border-[var(--surface-border)] bg-[var(--surface-card)]"
                }`}
              >
                {label}
              </button>
            ))}
          </div>
          <button
            type="button"
            onClick={exportCsv}
            className="inline-flex items-center gap-1.5 rounded-lg border border-[var(--surface-border)] bg-[var(--surface-card)] px-3 py-2 text-xs font-semibold hover:border-terracotta"
          >
            <Download className="h-3.5 w-3.5" />
            Export CSV
          </button>
        </div>

        {/* Tab 1: Financial Statements */}
        {tab === "statements" && (
          <section className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5 space-y-4">
            <div className="flex gap-2 border-b border-[var(--surface-border)] pb-3">
              {(
                [
                  ["pnl", "Profit & Loss (P&L)"],
                  ["balance_sheet", "Balance Sheet"],
                  ["trial_balance", "Trial Balance"],
                ] as const
              ).map(([id, label]) => (
                <button
                  key={id}
                  type="button"
                  onClick={() => setStmtTab(id)}
                  className={`rounded-md px-3 py-1.5 text-xs font-semibold ${
                    stmtTab === id ? "bg-terracotta/15 text-terracotta" : "text-[var(--ink-muted)]"
                  }`}
                >
                  {label}
                </button>
              ))}
            </div>

            {stmtTab === "pnl" && (
              <div className="space-y-2 font-mono text-xs">
                <div className="flex justify-between"><span>4010 · Food &amp; Beverage Sales</span><span>PKR 156,400.00</span></div>
                <div className="flex justify-between text-[var(--ink-muted)]"><span>4090/4095 · Discounts &amp; Refunds</span><span>(PKR 8,200.00)</span></div>
                <div className="flex justify-between border-t border-[var(--surface-border)] pt-2 font-bold"><span>Net Revenue</span><span>PKR 148,200.00</span></div>
                <div className="flex justify-between text-[var(--ink-muted)]"><span>5010/5020 · Direct Costs (COGS + Waste)</span><span>(PKR 40,200.00)</span></div>
                <div className="flex justify-between border-t border-[var(--surface-border)] pt-2 font-bold"><span>Gross Profit (72.9% Margin)</span><span>PKR 108,000.00</span></div>
                <div className="flex justify-between text-[var(--ink-muted)]"><span>5030–5050 · Operating Expenses</span><span>(PKR 4,850.00)</span></div>
                <div className="flex justify-between border-t border-[var(--surface-border)] pt-2 text-sm font-extrabold text-emerald-600 dark:text-emerald-400"><span>Net Operating Income</span><span>PKR 103,150.00</span></div>
              </div>
            )}

            {stmtTab === "balance_sheet" && (
              <div className="space-y-2 font-mono text-xs">
                <div className="flex justify-between font-bold"><span>Total Assets (1xxx · Cash, Bank, Inventory)</span><span>PKR 420,662.00</span></div>
                <div className="flex justify-between"><span>Total Liabilities (2xxx · AP &amp; GST Payable)</span><span>PKR 117,512.00</span></div>
                <div className="flex justify-between"><span>Total Equity (3xxx + Current Net Income)</span><span>PKR 303,150.00</span></div>
                <div className="flex justify-between border-t border-[var(--surface-border)] pt-2 font-bold text-emerald-600 dark:text-emerald-400">
                  <span>Liabilities + Equity (Equation Balanced ✓)</span>
                  <span>PKR 420,662.00</span>
                </div>
              </div>
            )}

            {stmtTab === "trial_balance" && (
              <div className="space-y-2 font-mono text-xs">
                <div className="flex justify-between font-bold"><span>Total Trial Balance Debits</span><span>PKR 465,712.00</span></div>
                <div className="flex justify-between font-bold"><span>Total Trial Balance Credits</span><span>PKR 465,712.00</span></div>
                <p className="pt-2 text-emerald-600 dark:text-emerald-400">DR == CR · Trial Balance is strictly balanced.</p>
              </div>
            )}
          </section>
        )}

        {/* Tab 2: Supplier Aging */}
        {tab === "aging" && (
          <section className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5 text-xs">
            <h2 className="mb-3 text-sm font-bold">Accounts Payable Aging Buckets</h2>
            <div className="grid grid-cols-6 border-b border-[var(--surface-border)] pb-2 font-bold text-[var(--ink-muted)]">
              <span>Supplier</span>
              <span className="text-right">Current</span>
              <span className="text-right">1–30d</span>
              <span className="text-right">31–60d</span>
              <span className="text-right">61–90d</span>
              <span className="text-right">Total</span>
            </div>
            <div className="mt-2 grid grid-cols-6 font-mono">
              <span className="font-sans font-semibold">Alpine Dairy &amp; Oat</span>
              <span className="text-right">10,200</span>
              <span className="text-right text-amber-600">5,100</span>
              <span className="text-right">0</span>
              <span className="text-right">0</span>
              <span className="text-right font-bold">15,300</span>
            </div>
            <div className="mt-2 grid grid-cols-6 font-mono">
              <span className="font-sans font-semibold">Origin Roasters</span>
              <span className="text-right">78,500</span>
              <span className="text-right">0</span>
              <span className="text-right">0</span>
              <span className="text-right">0</span>
              <span className="text-right font-bold">78,500</span>
            </div>
          </section>
        )}

        {/* Tab 3: Bank Reconciliation */}
        {tab === "bank" && (
          <section className="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5 text-xs">
            <div className="flex items-center gap-3">
              <Landmark className="h-5 w-5 text-terracotta" />
              <div>
                <p className="font-bold">1040 Main Operating Bank — Statement vs. Book</p>
                <p className="text-[var(--ink-muted)]">
                  Statement Closing: <strong className="font-mono">PKR 248,500.00</strong> · Cleared Book: <strong className="font-mono">PKR 248,500.00</strong> · Diff: <strong className="font-mono text-emerald-600">PKR 0.00</strong>
                </p>
              </div>
            </div>
            <button
              type="button"
              disabled={bankFinalized}
              onClick={() => setBankFinalized(true)}
              className="rounded-lg bg-terracotta px-4 py-2 font-bold text-white hover:bg-terracotta-700 disabled:opacity-50"
            >
              {bankFinalized ? "Reconciliation Finalized ✓" : "Finalize Reconciliation"}
            </button>
          </section>
        )}
      </div>
    </AppShell>
  );
}
