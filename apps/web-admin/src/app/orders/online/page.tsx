"use client";

import React, { useState } from "react";
import { Globe, LayoutDashboard, RefreshCw, RotateCcw } from "lucide-react";
import {
  AppShell,
  DataTable,
  KpiMetricCard,
  OrderModeBadge,
  SyncStatusBadge,
  type AppShellNavItem,
  type DataTableColumn,
  type OrderMode,
} from "@hospitality/ui";

interface OnlineOrderRow {
  id: string;
  orderNumber: string;
  branch: string;
  mode: OrderMode;
  totalPkr: string;
  handoffState: "pending_branch_acceptance" | "branch_accepted" | "cancelled_reversed";
  paymentState: "authorized" | "captured" | "reversed";
  slaCountdown: string;
}

const PILOT_ONLINE_ORDERS: OnlineOrderRow[] = [
  {
    id: "wo-1",
    orderNumber: "#ORD-4082",
    branch: "F-7 Coffee Bar",
    mode: "TAKEAWAY",
    totalPkr: "PKR 1,357.20",
    handoffState: "branch_accepted",
    paymentState: "captured",
    slaCountdown: "Accepted in 4s",
  },
  {
    id: "wo-2",
    orderNumber: "#ORD-4085",
    branch: "G-9 Flagship Dine-In",
    mode: "DELIVERY",
    totalPkr: "PKR 2,480.00",
    handoffState: "pending_branch_acceptance",
    paymentState: "authorized",
    slaCountdown: "03:42 left before auto-reverse",
  },
];

export default function OnlineOrdersHandoffPage() {
  const [orders, setOrders] = useState<OnlineOrderRow[]>(PILOT_ONLINE_ORDERS);

  function retryHandoff(id: string) {
    setOrders((prev) =>
      prev.map((o) =>
        o.id === id
          ? { ...o, handoffState: "branch_accepted", paymentState: "captured", slaCountdown: "Accepted via Retry" }
          : o
      )
    );
  }

  function cancelAndReverse(id: string) {
    setOrders((prev) =>
      prev.map((o) =>
        o.id === id
          ? { ...o, handoffState: "cancelled_reversed", paymentState: "reversed", slaCountdown: "Auth Reversed" }
          : o
      )
    );
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
      id: "online",
      label: "Online Orders & Handoff",
      icon: <Globe className="h-4 w-4" />,
      active: true,
    },
  ];

  const columns: DataTableColumn<OnlineOrderRow>[] = [
    {
      key: "orderNumber",
      header: "Order #",
      sortable: true,
      render: (r) => <span className="font-mono font-bold">{r.orderNumber}</span>,
    },
    { key: "branch", header: "Branch" },
    {
      key: "mode",
      header: "Mode",
      render: (r) => <OrderModeBadge mode={r.mode} size="sm" />,
    },
    { key: "totalPkr", header: "Total", align: "right" },
    {
      key: "handoffState",
      header: "Handoff / Payment",
      render: (r) => (
        <div className="space-y-0.5 font-mono text-[11px]">
          <p className="font-bold uppercase text-terracotta">{r.handoffState.replaceAll("_", " ")}</p>
          <p className="text-[var(--ink-muted)]">
            Payment: {r.paymentState} · {r.slaCountdown}
          </p>
        </div>
      ),
    },
    {
      key: "id",
      header: "Actions",
      align: "right",
      render: (r) =>
        r.handoffState === "pending_branch_acceptance" ? (
          <div className="flex justify-end gap-1.5">
            <button
              type="button"
              onClick={() => retryHandoff(r.id)}
              className="inline-flex items-center gap-1 rounded bg-emerald-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-emerald-700"
            >
              <RefreshCw className="h-3 w-3" />
              Retry Handoff
            </button>
            <button
              type="button"
              onClick={() => cancelAndReverse(r.id)}
              className="inline-flex items-center gap-1 rounded bg-rose-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-rose-700"
            >
              <RotateCcw className="h-3 w-3" />
              Cancel &amp; Reverse
            </button>
          </div>
        ) : (
          <span className="text-xs text-[var(--ink-muted)]">Settled</span>
        ),
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Online Orders & Handoff"
      navItems={navItems}
      topBarLeft={
        <h1 className="text-sm font-bold text-[var(--ink-primary)]">
          Online Orders &amp; Interrupted-Handoff Reconciliation
        </h1>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="HMAC Webhooks Active" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="Active Online Orders"
            value={String(orders.length)}
            deltaPercent={18.0}
            comparisonLabel="pickup, delivery & QR"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Awaiting Branch Handoff"
            value={String(orders.filter((o) => o.handoffState === "pending_branch_acceptance").length)}
            deltaPercent={0}
            comparisonLabel="SLA auto-reversal guarded"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Avg Handoff Latency"
            value="3.8s"
            deltaPercent={-12.0}
            comparisonLabel="cloud → branch KDS"
            freshnessLabel="today"
          />
          <KpiMetricCard
            title="Auth Reversals (24h)"
            value={String(orders.filter((o) => o.paymentState === "reversed").length)}
            deltaPercent={0}
            comparisonLabel="zero orphaned charges"
            freshnessLabel="live"
          />
        </div>

        <DataTable
          columns={columns}
          data={orders}
          caption="Online orders and branch handoff reconciliation"
        />
      </div>
    </AppShell>
  );
}
