"use client";

import React, { useState } from "react";
import { AlertTriangle, ClipboardList, LayoutDashboard, Send, ShoppingCart, Sparkles } from "lucide-react";
import {
  AppShell,
  DataTable,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
  type DataTableColumn,
} from "@hospitality/ui";

interface PoRow {
  id: string;
  poNumber: string;
  supplier: string;
  destination: string;
  totalPkr: string;
  status: "draft" | "pending_review" | "approved" | "sent";
  varianceAlert?: string;
  dispatchAttempts: number;
}

const PILOT_POS: PoRow[] = [
  {
    id: "po-1",
    poNumber: "PO-2026-091",
    supplier: "Alpine Dairy & Oat Distributors",
    destination: "F-7 Coffee Bar",
    totalPkr: "PKR 5,100.00",
    status: "approved",
    dispatchAttempts: 0,
  },
  {
    id: "po-2",
    poNumber: "PO-2026-092",
    supplier: "Origin Roasters Wholesale",
    destination: "Central Warehouse",
    totalPkr: "PKR 78,500.00",
    status: "pending_review",
    varianceAlert: "+12.5% vs last bill · Exceeds 50k auto-send cap",
    dispatchAttempts: 0,
  },
];

export default function PurchaseOrdersPage() {
  const [pos, setPos] = useState<PoRow[]>(PILOT_POS);

  function approvePO(id: string) {
    setPos((prev) => prev.map((p) => (p.id === id ? { ...p, status: "approved", varianceAlert: undefined } : p)));
  }

  function dispatchPO(id: string) {
    setPos((prev) =>
      prev.map((p) =>
        p.id === id && p.status !== "sent"
          ? { ...p, status: "sent", dispatchAttempts: p.dispatchAttempts + 1 }
          : p
      )
    );
  }

  function runReplenishment() {
    setPos((prev) => [
      {
        id: `po-auto-${Date.now()}`,
        poNumber: `PO-AUTO-${Date.now().toString().slice(-3)}`,
        supplier: "Alpine Dairy & Oat Distributors",
        destination: "G-9 Flagship Dine-In",
        totalPkr: "PKR 15,300.00",
        status: "draft",
        dispatchAttempts: 0,
      },
      ...prev,
    ]);
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
      id: "requisitions",
      label: "Procurement · Requisitions",
      icon: <ClipboardList className="h-4 w-4" />,
      onClick: () => {
        window.location.href = "/procurement/requisitions";
      },
    },
    {
      id: "pos",
      label: "Procurement · Purchase Orders",
      icon: <ShoppingCart className="h-4 w-4" />,
      active: true,
    },
  ];

  const columns: DataTableColumn<PoRow>[] = [
    {
      key: "poNumber",
      header: "PO #",
      sortable: true,
      render: (r) => <span className="font-mono font-bold">{r.poNumber}</span>,
    },
    { key: "supplier", header: "Supplier", sortable: true },
    { key: "destination", header: "Destination" },
    { key: "totalPkr", header: "Total", align: "right", sortable: true },
    {
      key: "status",
      header: "Status & Guardrails",
      render: (r) => (
        <div className="space-y-1">
          <span className="rounded-full bg-terracotta/15 px-2 py-0.5 font-mono text-[10px] font-bold uppercase text-terracotta">
            {r.status.replace("_", " ")}
          </span>
          {r.varianceAlert && (
            <div className="flex items-center gap-1 text-[11px] font-semibold text-amber-600 dark:text-amber-400">
              <AlertTriangle className="h-3 w-3" />
              {r.varianceAlert}
            </div>
          )}
        </div>
      ),
    },
    {
      key: "id",
      header: "Action",
      align: "right",
      render: (r) => {
        if (r.status === "pending_review" || r.status === "draft") {
          return (
            <button
              type="button"
              onClick={() => approvePO(r.id)}
              className="rounded bg-zinc-800 px-2.5 py-1 text-xs font-semibold text-white hover:bg-zinc-700"
            >
              Approve PO
            </button>
          );
        }
        if (r.status === "approved") {
          return (
            <button
              type="button"
              onClick={() => dispatchPO(r.id)}
              className="inline-flex items-center gap-1 rounded bg-terracotta px-2.5 py-1 text-xs font-semibold text-white hover:bg-terracotta-700"
            >
              <Send className="h-3 w-3" />
              Dispatch to Supplier
            </button>
          );
        }
        return (
          <span className="font-mono text-xs text-emerald-600 dark:text-emerald-400">
            Sent ({r.dispatchAttempts}x)
          </span>
        );
      },
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Automated Purchasing"
      navItems={navItems}
      topBarLeft={
        <h1 className="text-sm font-bold text-[var(--ink-primary)]">
          Automated PO Drafts &amp; Buyer Dispatch
        </h1>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="Guardrails Active" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="Auto-Drafted POs"
            value={String(pos.length)}
            deltaPercent={10.0}
            comparisonLabel="MOQ & pack-size aligned"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Flagged for Review"
            value={String(pos.filter((p) => p.status === "pending_review").length)}
            deltaPercent={0}
            comparisonLabel="price/budget guardrails"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Open Inbound Value"
            value="PKR 83,600"
            deltaPercent={6.5}
            comparisonLabel="confirmed & drafted"
            freshnessLabel="today"
          />
          <KpiMetricCard
            title="On-Time Supplier Rate"
            value="94.2%"
            deltaPercent={1.8}
            comparisonLabel="30-day rolling"
            freshnessLabel="synced"
          />
        </div>

        <div className="flex justify-end">
          <button
            type="button"
            onClick={runReplenishment}
            className="inline-flex items-center gap-1.5 rounded-lg bg-terracotta px-3.5 py-2 text-xs font-semibold text-white hover:bg-terracotta-700"
          >
            <Sparkles className="h-3.5 w-3.5" />
            Run Replenishment Planner
          </button>
        </div>

        <DataTable
          columns={columns}
          data={pos}
          caption="Purchase orders with guardrail review and idempotent supplier dispatch"
        />
      </div>
    </AppShell>
  );
}
