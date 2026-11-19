"use client";

import React, { useState } from "react";
import Link from "next/link";
import { Boxes, ChefHat, ClipboardCheck, LayoutDashboard, Trash2 } from "lucide-react";
import {
  AppShell,
  DataTable,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
  type DataTableColumn,
} from "@hospitality/ui";

interface VarianceRow {
  id: string;
  ingredient: string;
  unit: string;
  theoretical: string;
  actual: string;
  varianceQty: string;
  variancePct: string;
  varianceValuePkr: string;
  severity: "ok" | "warn" | "high";
}

const PILOT_VARIANCE: VarianceRow[] = [
  {
    id: "v-1",
    ingredient: "House Espresso Beans",
    unit: "g",
    theoretical: "3,600 g",
    actual: "3,960 g",
    varianceQty: "+360 g",
    variancePct: "+10.0%",
    varianceValuePkr: "PKR 1,620.00",
    severity: "high",
  },
  {
    id: "v-2",
    ingredient: "Barista Oat Milk",
    unit: "ml",
    theoretical: "25,000 ml",
    actual: "25,750 ml",
    varianceQty: "+750 ml",
    variancePct: "+3.0%",
    varianceValuePkr: "PKR 637.50",
    severity: "warn",
  },
  {
    id: "v-3",
    ingredient: "16oz Double-Wall Paper Cup",
    unit: "pcs",
    theoretical: "140 pcs",
    actual: "141 pcs",
    varianceQty: "+1 pcs",
    variancePct: "+0.7%",
    varianceValuePkr: "PKR 22.00",
    severity: "ok",
  },
];

export default function InventoryLedgerPage() {
  const [rows] = useState<VarianceRow[]>(PILOT_VARIANCE);
  const [wasteLogged, setWasteLogged] = useState(2);

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
      id: "ingredients",
      label: "Inventory · Ingredients",
      icon: <Boxes className="h-4 w-4" />,
      onClick: () => {
        window.location.href = "/inventory/ingredients";
      },
    },
    {
      id: "recipes",
      label: "Inventory · Recipes & COGS",
      icon: <ChefHat className="h-4 w-4" />,
      onClick: () => {
        window.location.href = "/inventory/recipes";
      },
    },
    {
      id: "ledger",
      label: "Stock Ledger & Variance",
      icon: <ClipboardCheck className="h-4 w-4" />,
      active: true,
    },
  ];

  const columns: DataTableColumn<VarianceRow>[] = [
    { key: "ingredient", header: "Ingredient", sortable: true },
    { key: "theoretical", header: "Theoretical Usage", align: "right" },
    { key: "actual", header: "Actual Usage", align: "right" },
    { key: "varianceQty", header: "Variance Qty", align: "right" },
    {
      key: "variancePct",
      header: "Variance %",
      align: "right",
      render: (r) => (
        <span
          className={`rounded px-2 py-0.5 font-mono text-xs font-bold ${
            r.severity === "high"
              ? "bg-rose-500/15 text-rose-600 dark:text-rose-400"
              : r.severity === "warn"
                ? "bg-amber-500/15 text-amber-600 dark:text-amber-400"
                : "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"
          }`}
        >
          {r.variancePct}
        </span>
      ),
    },
    { key: "varianceValuePkr", header: "Variance Impact", align: "right", sortable: true },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Stock Ledger & Variance"
      navItems={navItems}
      topBarLeft={
        <div className="flex items-center gap-3">
          <h1 className="text-sm font-bold text-[var(--ink-primary)]">
            Append-Only Stock Ledger &amp; Theoretical vs. Actual Variance
          </h1>
          <Link
            href="/inventory/recipes"
            className="rounded-md border border-[var(--surface-border)] px-2.5 py-1 text-xs font-medium text-[var(--ink-secondary)] hover:border-terracotta"
          >
            ← Recipes
          </Link>
        </div>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="Prep-Point Sync" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="Prep-Point Deductions (Today)"
            value="418"
            deltaPercent={6.4}
            comparisonLabel="idempotent ledger rows"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Theoretical vs Actual Variance"
            value="PKR 2,279.50"
            deltaPercent={-4.1}
            comparisonLabel="shrinkage this week"
            freshnessLabel="5m ago"
          />
          <KpiMetricCard
            title="Waste Events Logged"
            value={String(wasteLogged)}
            deltaPercent={0}
            comparisonLabel="spillage & post-prep"
            freshnessLabel="today"
          />
          <KpiMetricCard
            title="Production Batches"
            value="6"
            deltaPercent={12.0}
            comparisonLabel="avg yield 96.4%"
            freshnessLabel="today"
          />
        </div>

        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-sm font-bold">Theoretical vs. Actual Usage Reconciliation</h2>
            <p className="text-xs text-[var(--ink-muted)]">
              Compares recipe-exploded prep consumption against approved physical stock counts.
            </p>
          </div>
          <button
            type="button"
            onClick={() => setWasteLogged((c) => c + 1)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-rose-600 px-3.5 py-2 text-xs font-semibold text-white hover:bg-rose-700"
          >
            <Trash2 className="h-3.5 w-3.5" />
            Log Kitchen Waste
          </button>
        </div>

        <DataTable
          columns={columns}
          data={rows}
          caption="Theoretical vs actual ingredient usage variance"
        />
      </div>
    </AppShell>
  );
}
