"use client";

import React, { useMemo, useState } from "react";
import { Boxes, ChefHat, ClipboardCheck, LayoutDashboard, Truck, X } from "lucide-react";
import {
  AppShell,
  DataTable,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
  type DataTableColumn,
} from "@hospitality/ui";

type TransferStatus =
  | "approved"
  | "reserved"
  | "in_transit"
  | "partially_received"
  | "received";

interface TransferRow {
  id: string;
  transferNumber: string;
  route: string;
  itemSummary: string;
  dispatchedQty: number;
  receivedQty: number;
  damagedQty: number;
  shortageQty: number;
  unit: string;
  status: TransferStatus;
  poDraftRef?: string;
}

const PILOT_TRANSFERS: TransferRow[] = [
  {
    id: "tr-1001",
    transferNumber: "TR-2026-1001",
    route: "Central Warehouse → F-7 Coffee Bar",
    itemSummary: "Barista Oat Milk (12 L)",
    dispatchedQty: 12,
    receivedQty: 0,
    damagedQty: 0,
    shortageQty: 0,
    unit: "L",
    status: "in_transit",
  },
  {
    id: "tr-1002",
    transferNumber: "TR-2026-1002",
    route: "Central Warehouse → G-9 Flagship Dine-In",
    itemSummary: "House Espresso Beans (10 kg)",
    dispatchedQty: 10,
    receivedQty: 0,
    damagedQty: 0,
    shortageQty: 0,
    unit: "kg",
    status: "reserved",
  },
  {
    id: "tr-1003",
    transferNumber: "TR-2026-1003",
    route: "Central Warehouse → F-7 Coffee Bar",
    itemSummary: "16oz Double-Wall Cups (2 Boxes)",
    dispatchedQty: 2,
    receivedQty: 2,
    damagedQty: 0,
    shortageQty: 0,
    unit: "box",
    status: "received",
  },
];

export default function WarehouseTransfersPage() {
  const [transfers, setTransfers] = useState<TransferRow[]>(PILOT_TRANSFERS);
  const [filter, setFilter] = useState<"all" | TransferStatus>("all");
  const [receivingRow, setReceivingRow] = useState<TransferRow | null>(null);
  const [goodQty, setGoodQty] = useState<number>(8);
  const [damagedQty, setDamagedQty] = useState<number>(1);
  const [autoDraftPO, setAutoDraftPO] = useState<boolean>(true);

  const filtered = useMemo(
    () => (filter === "all" ? transfers : transfers.filter((t) => t.status === filter)),
    [transfers, filter]
  );

  function advanceStatus(id: string, next: TransferStatus) {
    setTransfers((prev) => prev.map((t) => (t.id === id ? { ...t, status: next } : t)));
  }

  function confirmReceive(e: React.FormEvent) {
    e.preventDefault();
    if (!receivingRow) return;
    const shortage = Math.max(0, receivingRow.dispatchedQty - goodQty - damagedQty);
    const poRef = autoDraftPO && shortage > 0 ? `PO-DRAFT-${Date.now().toString().slice(-4)}` : undefined;
    setTransfers((prev) =>
      prev.map((t) =>
        t.id === receivingRow.id
          ? {
              ...t,
              receivedQty: goodQty,
              damagedQty,
              shortageQty: shortage,
              status: shortage > 0 && !autoDraftPO ? "partially_received" : "received",
              poDraftRef: poRef,
            }
          : t
      )
    );
    setReceivingRow(null);
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
      onClick: () => {
        window.location.href = "/inventory/ledger";
      },
    },
    {
      id: "transfers",
      label: "Warehouse Transfers",
      icon: <Truck className="h-4 w-4" />,
      active: true,
    },
  ];

  const columns: DataTableColumn<TransferRow>[] = [
    {
      key: "transferNumber",
      header: "Transfer #",
      sortable: true,
      render: (r) => <span className="font-mono font-bold">{r.transferNumber}</span>,
    },
    { key: "route", header: "Route", sortable: true },
    { key: "itemSummary", header: "Items" },
    {
      key: "status",
      header: "Status",
      render: (r) => (
        <span className="rounded-full bg-terracotta/15 px-2.5 py-0.5 font-mono text-[11px] font-bold uppercase text-terracotta">
          {r.status.replace("_", " ")}
        </span>
      ),
    },
    {
      key: "receivedQty",
      header: "Received / Dispatched",
      render: (r) => (
        <span className="font-mono text-xs">
          {r.receivedQty}/{r.dispatchedQty} {r.unit}
          {r.shortageQty > 0 && (
            <span className="ml-1.5 text-rose-500">(-{r.shortageQty} short)</span>
          )}
          {r.poDraftRef && (
            <span className="ml-1.5 rounded bg-emerald-500/15 px-1.5 py-0.5 text-[10px] font-bold text-emerald-600 dark:text-emerald-400">
              {r.poDraftRef}
            </span>
          )}
        </span>
      ),
    },
    {
      key: "id",
      header: "Action",
      align: "right",
      render: (r) => {
        if (r.status === "approved") {
          return (
            <button
              type="button"
              onClick={() => advanceStatus(r.id, "reserved")}
              className="rounded bg-zinc-800 px-2.5 py-1 text-xs font-semibold text-white hover:bg-zinc-700"
            >
              Reserve
            </button>
          );
        }
        if (r.status === "reserved") {
          return (
            <button
              type="button"
              onClick={() => advanceStatus(r.id, "in_transit")}
              className="rounded bg-amber-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-amber-700"
            >
              Dispatch
            </button>
          );
        }
        if (r.status === "in_transit" || r.status === "partially_received") {
          return (
            <button
              type="button"
              onClick={() => {
                setReceivingRow(r);
                setGoodQty(Math.max(0, r.dispatchedQty - 4));
                setDamagedQty(1);
                setAutoDraftPO(true);
              }}
              className="rounded bg-terracotta px-2.5 py-1 text-xs font-semibold text-white hover:bg-terracotta-700"
            >
              Receive Stock
            </button>
          );
        }
        return <span className="text-xs text-[var(--ink-muted)]">Reconciled</span>;
      },
    },
  ];

  const modalShortage = receivingRow
    ? Math.max(0, receivingRow.dispatchedQty - goodQty - damagedQty)
    : 0;

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Warehouse & Supply Chain"
      navItems={navItems}
      topBarLeft={
        <h1 className="text-sm font-bold text-[var(--ink-primary)]">
          Warehouse Transfers &amp; Branch Receiving Console
        </h1>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="Warehouse Module" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="In-Transit Shipments"
            value={String(transfers.filter((t) => t.status === "in_transit").length)}
            deltaPercent={0}
            comparisonLabel="awaiting branch receipt"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Pending Dispatch"
            value={String(transfers.filter((t) => t.status === "reserved" || t.status === "approved").length)}
            deltaPercent={-8.0}
            comparisonLabel="reserved at warehouse"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="7-Day Transit Damages"
            value="PKR 850.00"
            deltaPercent={-14.2}
            comparisonLabel="logged to waste ledger"
            freshnessLabel="today"
          />
          <KpiMetricCard
            title="Branch Fulfillment Rate"
            value="96.8%"
            deltaPercent={2.4}
            comparisonLabel="on-time & complete"
            freshnessLabel="7d avg"
          />
        </div>

        <div className="flex flex-wrap gap-2">
          {(["all", "approved", "reserved", "in_transit", "partially_received", "received"] as const).map(
            (st) => (
              <button
                key={st}
                type="button"
                onClick={() => setFilter(st)}
                className={`rounded-lg border px-3 py-1.5 text-xs font-semibold capitalize ${
                  filter === st
                    ? "border-terracotta bg-terracotta text-white"
                    : "border-[var(--surface-border)] bg-[var(--surface-card)] text-[var(--ink-secondary)]"
                }`}
              >
                {st.replace("_", " ")}
              </button>
            )
          )}
        </div>

        <DataTable
          columns={columns}
          data={filtered}
          caption="Warehouse to branch transfers"
        />
      </div>

      {/* Branch Receiving & Discrepancy Modal */}
      {receivingRow && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-xs"
          onClick={() => setReceivingRow(null)}
        >
          <div
            className="w-full max-w-md rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-6 shadow-elevated"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-[var(--surface-border)] pb-3">
              <h2 className="text-sm font-bold">
                Receive {receivingRow.transferNumber}
              </h2>
              <button type="button" onClick={() => setReceivingRow(null)}>
                <X className="h-4 w-4" />
              </button>
            </div>
            <form onSubmit={confirmReceive} className="mt-4 space-y-4 text-xs">
              <p className="text-[var(--ink-muted)]">
                {receivingRow.itemSummary} · Dispatched:{" "}
                <strong className="font-mono">
                  {receivingRow.dispatchedQty} {receivingRow.unit}
                </strong>
              </p>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block font-semibold">Received Good Qty</label>
                  <input
                    type="number"
                    min={0}
                    max={receivingRow.dispatchedQty}
                    value={goodQty}
                    onChange={(e) => setGoodQty(Number(e.target.value))}
                    className="mt-1 w-full rounded-lg border border-[var(--surface-border)] bg-transparent px-3 py-2 font-mono"
                  />
                </div>
                <div>
                  <label className="block font-semibold">Damaged in Transit</label>
                  <input
                    type="number"
                    min={0}
                    max={receivingRow.dispatchedQty}
                    value={damagedQty}
                    onChange={(e) => setDamagedQty(Number(e.target.value))}
                    className="mt-1 w-full rounded-lg border border-[var(--surface-border)] bg-transparent px-3 py-2 font-mono"
                  />
                </div>
              </div>
              <div className="rounded-lg border border-[var(--surface-border)] bg-[var(--surface-subtle)] p-3 font-mono">
                Auto-Calculated Shortage: <strong>{modalShortage} {receivingRow.unit}</strong>
              </div>
              <label className="flex items-center gap-2 font-semibold">
                <input
                  type="checkbox"
                  checked={autoDraftPO}
                  onChange={(e) => setAutoDraftPO(e.target.checked)}
                />
                Auto-Draft Supplier PO for Remaining Shortage ({modalShortage} {receivingRow.unit})
              </label>
              <button
                type="submit"
                className="w-full rounded-lg bg-terracotta py-2.5 font-bold text-white hover:bg-terracotta-700"
              >
                Confirm Branch Receipt
              </button>
            </form>
          </div>
        </div>
      )}
    </AppShell>
  );
}
