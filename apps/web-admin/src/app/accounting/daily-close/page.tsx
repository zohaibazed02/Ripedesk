"use client";

import React, { useState } from "react";
import { BookOpen, CheckCircle2, LayoutDashboard, Lock } from "lucide-react";
import {
  AppShell,
  DataTable,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
  type DataTableColumn,
} from "@hospitality/ui";

interface JournalFeedRow {
  id: string;
  entryNumber: string;
  source: string;
  debitSummary: string;
  creditSummary: string;
  amountPkr: string;
  balanced: boolean;
}

const PILOT_JOURNALS: JournalFeedRow[] = [
  {
    id: "je-1",
    entryNumber: "JE-2026-0412",
    source: "POS Sale (Split Cash/Card)",
    debitSummary: "DR 1010 Cash · DR 1030 Card",
    creditSummary: "CR 4010 Sales · CR 2100 GST",
    amountPkr: "PKR 1,357.20",
    balanced: true,
  },
  {
    id: "je-2",
    entryNumber: "JE-2026-0413",
    source: "Cash Shift Variance",
    debitSummary: "DR 5030 Cash Over/Short",
    creditSummary: "CR 1010 Cash on Hand",
    amountPkr: "PKR 50.00",
    balanced: true,
  },
  {
    id: "je-3",
    entryNumber: "JE-2026-0414",
    source: "Card Settlement Batch",
    debitSummary: "DR 1040 Bank · DR 5040 Fees",
    creditSummary: "CR 1030 Card Clearing",
    amountPkr: "PKR 48,200.00",
    balanced: true,
  },
];

export default function DailyClosePage() {
  const [fiscalConfirmed, setFiscalConfirmed] = useState(true);
  const [closed, setClosed] = useState(false);

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
      active: true,
    },
  ];

  const columns: DataTableColumn<JournalFeedRow>[] = [
    {
      key: "entryNumber",
      header: "Entry #",
      render: (r) => <span className="font-mono font-bold">{r.entryNumber}</span>,
    },
    { key: "source", header: "Source" },
    { key: "debitSummary", header: "Debits", render: (r) => <span className="font-mono text-[11px]">{r.debitSummary}</span> },
    { key: "creditSummary", header: "Credits", render: (r) => <span className="font-mono text-[11px]">{r.creditSummary}</span> },
    { key: "amountPkr", header: "Amount", align: "right" },
    {
      key: "balanced",
      header: "Check",
      align: "center",
      render: () => (
        <span className="inline-flex items-center gap-1 rounded bg-emerald-500/15 px-2 py-0.5 font-mono text-[10px] font-bold text-emerald-600 dark:text-emerald-400">
          <CheckCircle2 className="h-3 w-3" /> DR == CR
        </span>
      ),
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Double-Entry Accounting"
      navItems={navItems}
      topBarLeft={
        <h1 className="text-sm font-bold text-[var(--ink-primary)]">
          Daily Branch Reconciliation &amp; General Ledger Close
        </h1>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="0 Pending Offline Orders" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="Gross Sales (Today)"
            value="PKR 156,400.00"
            deltaPercent={11.2}
            comparisonLabel="F-7 Coffee Bar"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Net Revenue"
            value="PKR 148,200.00"
            deltaPercent={10.4}
            comparisonLabel="after discounts & refunds"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Sales Tax / GST Payable"
            value="PKR 23,712.00"
            deltaPercent={0}
            comparisonLabel="16% jurisdiction rate"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="POS vs GL Diff"
            value="PKR 0.00"
            deltaPercent={0}
            comparisonLabel="Balanced ✓"
            freshnessLabel="reconciled"
          />
        </div>

        {/* Daily Close Lock Card */}
        <section className="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5">
          <div className="space-y-1 text-xs">
            <p className="font-bold">Business Date: 2026-11-19 · F-7 Coffee Bar (ARB-F7)</p>
            <label className="flex items-center gap-2 text-[var(--ink-muted)]">
              <input
                type="checkbox"
                checked={fiscalConfirmed}
                onChange={(e) => setFiscalConfirmed(e.target.checked)}
              />
              Local fiscal/GST tax rules confirmed for automated daily close lock
            </label>
          </div>
          <button
            type="button"
            disabled={!fiscalConfirmed || closed}
            onClick={() => setClosed(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-terracotta px-4 py-2 text-xs font-bold text-white hover:bg-terracotta-700 disabled:opacity-40"
          >
            <Lock className="h-3.5 w-3.5" />
            {closed ? "Daily Close Locked ✓" : "Lock & Post Daily Close"}
          </button>
        </section>

        <DataTable
          columns={columns}
          data={PILOT_JOURNALS}
          caption="Automated double-entry General Ledger journals"
        />
      </div>
    </AppShell>
  );
}
