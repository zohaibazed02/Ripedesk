"use client";

import React, { useState } from "react";
import Link from "next/link";
import { ClipboardList, LayoutDashboard, ShoppingCart, Truck, X } from "lucide-react";
import {
  AppShell,
  DataTable,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
  type DataTableColumn,
} from "@hospitality/ui";

interface RequisitionRow {
  id: string;
  reqNumber: string;
  branch: string;
  template: string;
  neededBy: string;
  itemSummary: string;
  status: "submitted" | "approved" | "fulfilled";
  transferRef?: string;
  poRef?: string;
}

const PILOT_REQUISITIONS: RequisitionRow[] = [
  {
    id: "req-101",
    reqNumber: "REQ-F7-101",
    branch: "F-7 Coffee Bar",
    template: "Daily Barista Prep",
    neededBy: "Tomorrow 07:00",
    itemSummary: "Barista Oat Milk (12 L)",
    status: "submitted",
  },
  {
    id: "req-102",
    reqNumber: "REQ-G9-102",
    branch: "G-9 Flagship Dine-In",
    template: "Weekend Hot Kitchen",
    neededBy: "Tomorrow 09:00",
    itemSummary: "Pugliese Burrata (24 pcs)",
    status: "approved",
    transferRef: "TR-2026-1004",
    poRef: "PO-2026-088",
  },
];

export default function RequisitionsPage() {
  const [rows, setRows] = useState<RequisitionRow[]>(PILOT_REQUISITIONS);
  const [previewRow, setPreviewRow] = useState<RequisitionRow | null>(null);

  function approveAndSplitSource(row: RequisitionRow) {
    setRows((prev) =>
      prev.map((r) =>
        r.id === row.id
          ? {
              ...r,
              status: "approved",
              transferRef: "TR-2026-1009",
              poRef: "PO-DRAFT-091",
            }
          : r
      )
    );
    setPreviewRow(null);
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
      id: "transfers",
      label: "Warehouse Transfers",
      icon: <Truck className="h-4 w-4" />,
      onClick: () => {
        window.location.href = "/inventory/transfers";
      },
    },
    {
      id: "requisitions",
      label: "Procurement · Requisitions",
      icon: <ClipboardList className="h-4 w-4" />,
      active: true,
    },
    {
      id: "pos",
      label: "Procurement · Purchase Orders",
      icon: <ShoppingCart className="h-4 w-4" />,
      onClick: () => {
        window.location.href = "/procurement/purchase-orders";
      },
    },
  ];

  const columns: DataTableColumn<RequisitionRow>[] = [
    {
      key: "reqNumber",
      header: "Requisition #",
      sortable: true,
      render: (r) => <span className="font-mono font-bold">{r.reqNumber}</span>,
    },
    { key: "branch", header: "Branch", sortable: true },
    { key: "template", header: "Prep Template" },
    { key: "itemSummary", header: "Requested Items" },
    { key: "neededBy", header: "Needed By" },
    {
      key: "status",
      header: "Status / Sourcing",
      render: (r) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="rounded-full bg-terracotta/15 px-2 py-0.5 font-mono text-[10px] font-bold uppercase text-terracotta">
            {r.status}
          </span>
          {r.transferRef && (
            <span className="rounded bg-emerald-500/15 px-1.5 py-0.5 font-mono text-[10px] font-bold text-emerald-600 dark:text-emerald-400">
              {r.transferRef}
            </span>
          )}
          {r.poRef && (
            <span className="rounded bg-sky-500/15 px-1.5 py-0.5 font-mono text-[10px] font-bold text-sky-600 dark:text-sky-400">
              {r.poRef}
            </span>
          )}
        </div>
      ),
    },
    {
      key: "id",
      header: "Action",
      align: "right",
      render: (r) =>
        r.status === "submitted" ? (
          <button
            type="button"
            onClick={() => setPreviewRow(r)}
            className="rounded bg-terracotta px-2.5 py-1 text-xs font-semibold text-white hover:bg-terracotta-700"
          >
            Approve &amp; Split-Source
          </button>
        ) : (
          <span className="text-xs text-[var(--ink-muted)]">Sourced</span>
        ),
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Kitchen Requisitions"
      navItems={navItems}
      topBarLeft={
        <div className="flex items-center gap-3">
          <h1 className="text-sm font-bold text-[var(--ink-primary)]">
            Kitchen Requisitions &amp; Split-Sourcing
          </h1>
          <Link
            href="/procurement/purchase-orders"
            className="rounded-md border border-[var(--surface-border)] px-2.5 py-1 text-xs font-medium text-[var(--ink-secondary)] hover:border-terracotta"
          >
            Purchase Orders →
          </Link>
        </div>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="Split-Sourcing Active" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="Open Requisitions"
            value={String(rows.length)}
            deltaPercent={5.0}
            comparisonLabel="across 2 pilot branches"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Pending Approval"
            value={String(rows.filter((r) => r.status === "submitted").length)}
            deltaPercent={0}
            comparisonLabel="awaiting split-source"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Warehouse Fulfillment"
            value="66.7%"
            deltaPercent={4.2}
            comparisonLabel="served from Central WH"
            freshnessLabel="7d avg"
          />
          <KpiMetricCard
            title="Linked Auto-PO Drafts"
            value={String(rows.filter((r) => Boolean(r.poRef)).length)}
            deltaPercent={0}
            comparisonLabel="MOQ & pack-size rounded"
            freshnessLabel="live"
          />
        </div>

        <DataTable
          columns={columns}
          data={rows}
          caption="Kitchen requisitions with warehouse transfer and PO links"
        />
      </div>

      {/* Split-Source Preview Drawer */}
      {previewRow && (
        <div
          className="fixed inset-0 z-50 flex justify-end bg-black/50 backdrop-blur-xs"
          onClick={() => setPreviewRow(null)}
        >
          <aside
            className="flex h-full w-full max-w-md flex-col border-l border-[var(--surface-border)] bg-[var(--surface-card)] p-6 shadow-elevated"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-[var(--surface-border)] pb-3">
              <h2 className="text-sm font-bold">
                Split-Source Preview · {previewRow.reqNumber}
              </h2>
              <button type="button" onClick={() => setPreviewRow(null)}>
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="mt-4 space-y-4 text-xs">
              <div className="rounded-lg border border-[var(--surface-border)] bg-[var(--surface-subtle)] p-3.5">
                <p className="font-bold">Barista Oat Milk — Requested: 12 L</p>
                <ul className="mt-2 space-y-1 font-mono text-[11px]">
                  <li className="text-emerald-600 dark:text-emerald-400">
                    • Central Warehouse Available: 8 L → Reserve Transfer
                  </li>
                  <li className="text-amber-600 dark:text-amber-400">
                    • Uncovered Shortage: 4 L (Confirmed Inbound: 0 L)
                  </li>
                  <li className="text-sky-600 dark:text-sky-400">
                    • Supplier Pack (6 L) &amp; MOQ (6 L) → Draft PO for 6 L
                  </li>
                </ul>
              </div>
              <button
                type="button"
                onClick={() => approveAndSplitSource(previewRow)}
                className="w-full rounded-lg bg-terracotta py-2.5 font-bold text-white hover:bg-terracotta-700"
              >
                Confirm Approval, Reserve 8 L &amp; Draft 6 L PO
              </button>
            </div>
          </aside>
        </div>
      )}
    </AppShell>
  );
}
