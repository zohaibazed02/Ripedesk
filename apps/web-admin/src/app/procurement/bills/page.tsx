"use client";

import React, { useState } from "react";
import { AlertTriangle, Camera, ClipboardList, FileText, LayoutDashboard, ShoppingCart, Upload, X } from "lucide-react";
import {
  AppShell,
  DataTable,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
  type DataTableColumn,
} from "@hospitality/ui";

interface VendorBillRow {
  id: string;
  invoiceNumber: string;
  supplier: string;
  date: string;
  totalPkr: string;
  confidencePct: number;
  modelVersion: string;
  status: "extracted_needs_review" | "duplicate_flagged" | "posted";
  sampleUrduLine: string;
}

const PILOT_BILLS: VendorBillRow[] = [
  {
    id: "vb-1",
    invoiceNumber: "INV-2026-0042",
    supplier: "Alpine Dairy & Oat Distributors",
    date: "2026-11-19",
    totalPkr: "PKR 10,200.00",
    confidencePct: 97,
    modelVersion: "v1.0-urdu-en",
    status: "extracted_needs_review",
    sampleUrduLine: "Barista Oat Milk 1L / اوٹ ملک (12 Pack) · 12 L × 850.00",
  },
  {
    id: "vb-2",
    invoiceNumber: "INV-2026-0042",
    supplier: "Alpine Dairy & Oat Distributors",
    date: "2026-11-19",
    totalPkr: "PKR 10,200.00",
    confidencePct: 97,
    modelVersion: "v1.0-urdu-en",
    status: "duplicate_flagged",
    sampleUrduLine: "Blocked: SHA-256 & normalized invoice number INV20260042 already uploaded",
  },
];

export default function VendorBillsOcrPage() {
  const [bills] = useState<VendorBillRow[]>(PILOT_BILLS);
  const [blurWarning, setBlurWarning] = useState(false);
  const [offlineQueued, setOfflineQueued] = useState(0);
  const [inspecting, setInspecting] = useState<VendorBillRow | null>(null);

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
      onClick: () => {
        window.location.href = "/procurement/purchase-orders";
      },
    },
    {
      id: "bills",
      label: "Procurement · Vendor Bills (OCR)",
      icon: <FileText className="h-4 w-4" />,
      active: true,
    },
  ];

  const columns: DataTableColumn<VendorBillRow>[] = [
    {
      key: "invoiceNumber",
      header: "Invoice #",
      sortable: true,
      render: (r) => (
        <div className="flex items-center gap-1.5">
          <span className="font-mono font-bold">{r.invoiceNumber}</span>
          {r.status === "duplicate_flagged" && (
            <span className="rounded bg-rose-500/15 px-1.5 py-0.5 text-[10px] font-bold text-rose-600 dark:text-rose-400">
              DUPLICATE
            </span>
          )}
        </div>
      ),
    },
    { key: "supplier", header: "Supplier", sortable: true },
    { key: "date", header: "Date" },
    { key: "totalPkr", header: "Extracted Total", align: "right", sortable: true },
    {
      key: "confidencePct",
      header: "OCR Confidence",
      render: (r) => (
        <span className="font-mono text-xs font-bold text-emerald-600 dark:text-emerald-400">
          {r.confidencePct}% · {r.modelVersion}
        </span>
      ),
    },
    {
      key: "id",
      header: "Action",
      align: "right",
      render: (r) => (
        <button
          type="button"
          onClick={() => setInspecting(r)}
          className="rounded border border-[var(--surface-border)] px-2.5 py-1 text-xs font-semibold hover:border-terracotta"
        >
          Inspect Extraction
        </button>
      ),
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Document AI & OCR"
      navItems={navItems}
      topBarLeft={
        <h1 className="text-sm font-bold text-[var(--ink-primary)]">
          Vendor Bill Capture, Blur Check &amp; Urdu/English OCR Inbox
        </h1>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="7-Yr Fiscal S3" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="Pending Review Bills"
            value={String(bills.filter((b) => b.status === "extracted_needs_review").length)}
            deltaPercent={0}
            comparisonLabel="ready for 3-way match"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Avg OCR Confidence"
            value="97.0%"
            deltaPercent={1.4}
            comparisonLabel="v1.0-urdu-en"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Duplicates Blocked"
            value={String(bills.filter((b) => b.status === "duplicate_flagged").length)}
            deltaPercent={0}
            comparisonLabel="SHA-256 + normalized #"
            freshnessLabel="today"
          />
          <KpiMetricCard
            title="Offline Queued Captures"
            value={String(offlineQueued)}
            deltaPercent={0}
            comparisonLabel="auto-uploads on reconnect"
            freshnessLabel="browser queue"
          />
        </div>

        {/* Smart Upload & Camera Dropzone */}
        <section className="rounded-xl border border-dashed border-[var(--surface-border)] bg-[var(--surface-card)] p-5">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-terracotta/15 text-terracotta">
                <Camera className="h-5 w-5" />
              </div>
              <div>
                <p className="text-sm font-bold">Capture or Upload Vendor Bill (JPG, PNG, Multipage PDF)</p>
                <p className="text-xs text-[var(--ink-muted)]">
                  Runs Laplacian blur detection, SHA-256 duplicate fingerprinting, and Urdu/English OCR.
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setBlurWarning(true)}
                className="rounded-lg border border-[var(--surface-border)] px-3 py-2 text-xs font-semibold hover:border-amber-500"
              >
                Simulate Blurry Photo
              </button>
              <button
                type="button"
                onClick={() => setOfflineQueued((c) => c + 1)}
                className="inline-flex items-center gap-1.5 rounded-lg bg-terracotta px-3.5 py-2 text-xs font-semibold text-white hover:bg-terracotta-700"
              >
                <Upload className="h-3.5 w-3.5" />
                Queue Offline Capture
              </button>
            </div>
          </div>

          {blurWarning && (
            <div className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-xs">
              <div className="flex items-center gap-2 font-semibold text-amber-700 dark:text-amber-300">
                <AlertTriangle className="h-4 w-4" />
                Blurry photo detected — Sharpness variance 38/120. Retake recommended.
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setBlurWarning(false)}
                  className="rounded bg-amber-600 px-3 py-1.5 font-bold text-white"
                >
                  Retake Photo
                </button>
                <button
                  type="button"
                  onClick={() => setBlurWarning(false)}
                  className="rounded border border-amber-500/50 px-3 py-1.5 font-semibold"
                >
                  Upload Anyway (Faint Receipt)
                </button>
              </div>
            </div>
          )}
        </section>

        <DataTable
          columns={columns}
          data={bills}
          caption="Extracted vendor bills inbox"
        />
      </div>

      {inspecting && (
        <div
          className="fixed inset-0 z-50 flex justify-end bg-black/50 backdrop-blur-xs"
          onClick={() => setInspecting(null)}
        >
          <aside
            className="flex h-full w-full max-w-md flex-col border-l border-[var(--surface-border)] bg-[var(--surface-card)] p-6 shadow-elevated"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-[var(--surface-border)] pb-3">
              <h2 className="text-sm font-bold">
                OCR Extraction · {inspecting.invoiceNumber}
              </h2>
              <button type="button" onClick={() => setInspecting(null)}>
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="mt-4 space-y-3 text-xs">
              <p><strong>Supplier:</strong> {inspecting.supplier}</p>
              <p><strong>Total:</strong> <span className="font-mono">{inspecting.totalPkr}</span></p>
              <p><strong>Model:</strong> <span className="font-mono">{inspecting.modelVersion}</span> ({inspecting.confidencePct}%)</p>
              <div className="rounded-lg border border-[var(--surface-border)] bg-[var(--surface-subtle)] p-3 font-mono text-[11px]">
                {inspecting.sampleUrduLine}
              </div>
            </div>
          </aside>
        </div>
      )}
    </AppShell>
  );
}
