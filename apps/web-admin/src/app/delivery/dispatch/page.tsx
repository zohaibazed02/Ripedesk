"use client";

import React, { useState } from "react";
import { Bike, CheckCircle2, LayoutDashboard, Wallet } from "lucide-react";
import {
  AppShell,
  DataTable,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
  type DataTableColumn,
} from "@hospitality/ui";

interface DispatchTripRow {
  id: string;
  orderNumber: string;
  rider: string;
  status: "unassigned" | "out_for_delivery" | "delivered";
  paymentType: "PREPAID" | "COD";
  codAmount: number;
  podVerified: boolean;
  codReconciled: boolean;
}

const PILOT_TRIPS: DispatchTripRow[] = [
  {
    id: "dt-1",
    orderNumber: "#ORD-4085",
    rider: "Kamran Ali",
    status: "delivered",
    paymentType: "COD",
    codAmount: 2480,
    podVerified: true,
    codReconciled: false,
  },
  {
    id: "dt-2",
    orderNumber: "#ORD-4089",
    rider: "Kamran Ali",
    status: "delivered",
    paymentType: "COD",
    codAmount: 1870,
    podVerified: true,
    codReconciled: false,
  },
  {
    id: "dt-3",
    orderNumber: "#ORD-4091",
    rider: "Unassigned",
    status: "unassigned",
    paymentType: "PREPAID",
    codAmount: 0,
    podVerified: false,
    codReconciled: true,
  },
];

export default function DeliveryDispatchPage() {
  const [trips, setTrips] = useState<DispatchTripRow[]>(PILOT_TRIPS);
  const [handedOver, setHandedOver] = useState<number>(4350);

  const unreconciledCod = trips
    .filter((t) => t.paymentType === "COD" && t.status === "delivered" && !t.codReconciled)
    .reduce((sum, t) => sum + t.codAmount, 0);

  const variance = handedOver - unreconciledCod;

  function assignRider(id: string) {
    setTrips((prev) =>
      prev.map((t) => (t.id === id ? { ...t, rider: "Salman Rider", status: "out_for_delivery" } : t))
    );
  }

  function reconcileAllCod() {
    setTrips((prev) => prev.map((t) => (t.status === "delivered" ? { ...t, codReconciled: true } : t)));
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
      id: "dispatch",
      label: "Delivery · Dispatch & COD",
      icon: <Bike className="h-4 w-4" />,
      active: true,
    },
  ];

  const columns: DataTableColumn<DispatchTripRow>[] = [
    {
      key: "orderNumber",
      header: "Order #",
      render: (r) => <span className="font-mono font-bold">{r.orderNumber}</span>,
    },
    { key: "rider", header: "Assigned Rider" },
    {
      key: "status",
      header: "Status",
      render: (r) => (
        <span className="rounded-full bg-terracotta/15 px-2 py-0.5 font-mono text-[10px] font-bold uppercase text-terracotta">
          {r.status.replaceAll("_", " ")}
        </span>
      ),
    },
    {
      key: "paymentType",
      header: "Payment / COD",
      render: (r) =>
        r.paymentType === "COD" ? (
          <span className="font-mono text-xs font-bold text-amber-600 dark:text-amber-400">
            COD PKR {r.codAmount.toLocaleString()}
          </span>
        ) : (
          <span className="font-mono text-xs font-bold text-emerald-600 dark:text-emerald-400">
            PREPAID
          </span>
        ),
    },
    {
      key: "podVerified",
      header: "POD",
      render: (r) =>
        r.podVerified ? (
          <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="h-3.5 w-3.5" /> Photo + Recipient
          </span>
        ) : (
          <span className="text-xs text-[var(--ink-muted)]">—</span>
        ),
    },
    {
      key: "id",
      header: "Action",
      align: "right",
      render: (r) =>
        r.status === "unassigned" ? (
          <button
            type="button"
            onClick={() => assignRider(r.id)}
            className="rounded bg-terracotta px-2.5 py-1 text-xs font-semibold text-white hover:bg-terracotta-700"
          >
            Assign Rider
          </button>
        ) : r.codReconciled ? (
          <span className="text-xs text-emerald-600 dark:text-emerald-400">Reconciled</span>
        ) : (
          <span className="text-xs text-amber-600 dark:text-amber-400">Pending COD</span>
        ),
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Delivery & Rider COD"
      navItems={navItems}
      topBarLeft={
        <h1 className="text-sm font-bold text-[var(--ink-primary)]">
          Delivery Dispatch Console &amp; Rider COD Reconciliation
        </h1>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="Rider App Sync" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="Awaiting Assignment"
            value={String(trips.filter((t) => t.status === "unassigned").length)}
            deltaPercent={0}
            comparisonLabel="ready at pass"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Out for Delivery"
            value={String(trips.filter((t) => t.status === "out_for_delivery").length)}
            deltaPercent={0}
            comparisonLabel="active riders"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Unreconciled Rider COD"
            value={`PKR ${unreconciledCod.toLocaleString()}`}
            deltaPercent={0}
            comparisonLabel="cash-in-hand"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="On-Time Delivery Rate"
            value="95.4%"
            deltaPercent={2.1}
            comparisonLabel="<= 35m SLA"
            freshnessLabel="today"
          />
        </div>

        <DataTable columns={columns} data={trips} caption="Delivery trips and COD status" />

        {/* Rider COD Settlement Panel */}
        {unreconciledCod > 0 && (
          <section className="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5">
            <div className="flex items-center gap-3">
              <Wallet className="h-5 w-5 text-terracotta" />
              <div className="text-xs">
                <p className="font-bold">Rider COD Settlement · Kamran Ali</p>
                <p className="text-[var(--ink-muted)]">
                  Expected COD: <strong className="font-mono">PKR {unreconciledCod.toLocaleString()}</strong> · Variance:{" "}
                  <strong className={`font-mono ${variance === 0 ? "text-emerald-600" : "text-rose-600"}`}>
                    PKR {variance}
                  </strong>
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <input
                type="number"
                value={handedOver}
                onChange={(e) => setHandedOver(Number(e.target.value))}
                className="w-32 rounded-lg border border-[var(--surface-border)] bg-transparent px-3 py-2 font-mono text-xs"
              />
              <button
                type="button"
                onClick={reconcileAllCod}
                className="rounded-lg bg-terracotta px-4 py-2 text-xs font-bold text-white hover:bg-terracotta-700"
              >
                Reconcile &amp; Post Rider COD
              </button>
            </div>
          </section>
        )}
      </div>
    </AppShell>
  );
}
