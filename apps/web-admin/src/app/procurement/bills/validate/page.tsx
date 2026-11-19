"use client";

import React, { useState } from "react";
import Link from "next/link";
import { AlertTriangle, CheckCircle2, FileText, History, LayoutDashboard, ZoomIn } from "lucide-react";
import {
  AppShell,
  SyncStatusBadge,
  type AppShellNavItem,
} from "@hospitality/ui";

interface EditableLine {
  lineNumber: number;
  rawDescription: string;
  mappedIngredient: string;
  unit: string;
  qty: number;
  unitPrice: number;
  lastPaid: number;
  saveAlias: boolean;
}

export default function VendorBillValidationWorkbenchPage() {
  const [lines, setLines] = useState<EditableLine[]>([
    {
      lineNumber: 1,
      rawDescription: "Barista Oat Milk 1L / اوٹ ملک (12 Pack)",
      mappedIngredient: "Barista Oat Milk (ING-MILK-OAT)",
      unit: "L",
      qty: 12,
      unitPrice: 1050,
      lastPaid: 850,
      saveAlias: true,
    },
    {
      lineNumber: 2,
      rawDescription: "Espresso Beans Roast / کافی بینز",
      mappedIngredient: "",
      unit: "kg",
      qty: 5,
      unitPrice: 4500,
      lastPaid: 4500,
      saveAlias: true,
    },
  ]);

  const [editHistory, setEditHistory] = useState<string[]>([
    "23:31 · OCR v1.0-urdu-en extracted 2 lines (97% confidence)",
  ]);
  const [approved, setApproved] = useState(false);

  const unmappedCount = lines.filter((l) => !l.mappedIngredient).length;
  const grandTotal = lines.reduce((sum, l) => sum + l.qty * l.unitPrice, 0);

  function mapLineIngredient(lineNumber: number, ing: string) {
    setLines((prev) =>
      prev.map((l) => (l.lineNumber === lineNumber ? { ...l, mappedIngredient: ing } : l))
    );
    setEditHistory((prev) => [
      `Just now · Reviewer mapped Line #${lineNumber} → ${ing} (alias saved)`,
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
      id: "bills",
      label: "Vendor Bills (OCR Inbox)",
      icon: <FileText className="h-4 w-4" />,
      onClick: () => {
        window.location.href = "/procurement/bills";
      },
    },
    {
      id: "validate",
      label: "Split-Screen Validation",
      icon: <CheckCircle2 className="h-4 w-4" />,
      active: true,
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Bill Validation Workbench"
      navItems={navItems}
      topBarLeft={
        <div className="flex items-center gap-3">
          <h1 className="text-sm font-bold text-[var(--ink-primary)]">
            Split-Screen Vendor Bill Validation · INV-2026-0042
          </h1>
          <Link
            href="/procurement/bills"
            className="rounded-md border border-[var(--surface-border)] px-2.5 py-1 text-xs font-medium text-[var(--ink-secondary)] hover:border-terracotta"
          >
            ← Back to OCR Inbox
          </Link>
        </div>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="Audit Log Active" />}
    >
      <div className="mx-auto grid max-w-7xl grid-cols-1 gap-5 lg:grid-cols-12">
        {/* Left Pane: Original Document Preview */}
        <section className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-4 lg:col-span-5">
          <div className="flex items-center justify-between border-b border-[var(--surface-border)] pb-3">
            <span className="text-xs font-bold">Original Bill Scan (SHA-256 Verified)</span>
            <span className="inline-flex items-center gap-1 font-mono text-[11px] text-[var(--ink-muted)]">
              <ZoomIn className="h-3.5 w-3.5" /> 100% · Sharpness 412
            </span>
          </div>
          <div className="mt-4 rounded-lg border border-dashed border-[var(--surface-border)] bg-[var(--surface-subtle)] p-6 font-mono text-xs space-y-3">
            <p className="text-center font-bold">ALPINE DAIRY &amp; OAT DISTRIBUTORS</p>
            <p className="text-center text-[11px] text-[var(--ink-muted)]">
              Invoice # INV-2026-0042 · Date: 2026-11-19 · PO: PO-2026-091
            </p>
            <hr className="border-[var(--surface-border)]" />
            <p>1. Barista Oat Milk 1L / اوٹ ملک — 12 L × 1,050 = 12,600</p>
            <p>2. Espresso Beans Roast / کافی بینز — 5 kg × 4,500 = 22,500</p>
            <hr className="border-[var(--surface-border)]" />
            <p className="text-right font-bold">GRAND TOTAL: PKR {grandTotal.toLocaleString()}</p>
          </div>
        </section>

        {/* Right Pane: Editable Lines, Anomaly Flags & Reviewer History */}
        <section className="space-y-4 lg:col-span-7">
          <div className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--surface-border)] pb-3">
              <div>
                <h2 className="text-sm font-bold">Extracted Lines &amp; Anomaly Flags</h2>
                <p className="text-xs text-[var(--ink-muted)]">
                  Resolve all red errors before approving for 3-way PO/GRN matching.
                </p>
              </div>
              <button
                type="button"
                disabled={unmappedCount > 0 || approved}
                onClick={() => setApproved(true)}
                className="rounded-lg bg-terracotta px-4 py-2 text-xs font-bold text-white hover:bg-terracotta-700 disabled:opacity-40"
              >
                {approved ? "Approved for 3-Way Match ✓" : "Approve for 3-Way Match"}
              </button>
            </div>

            <div className="mt-4 space-y-3">
              {lines.map((line) => {
                const priceVarPct = ((line.unitPrice - line.lastPaid) / line.lastPaid) * 100;
                const isPriceSpike = Math.abs(priceVarPct) > 15;
                return (
                  <div
                    key={line.lineNumber}
                    className="rounded-lg border border-[var(--surface-border)] p-3.5 text-xs space-y-2"
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-bold">
                        #{line.lineNumber} · {line.rawDescription}
                      </span>
                      <span className="font-mono font-bold">
                        PKR {(line.qty * line.unitPrice).toLocaleString()}
                      </span>
                    </div>

                    <div className="flex flex-wrap items-center gap-2">
                      {!line.mappedIngredient ? (
                        <button
                          type="button"
                          onClick={() =>
                            mapLineIngredient(line.lineNumber, "House Espresso Beans (ING-BEANS-ESP)")
                          }
                          className="rounded bg-rose-600 px-2.5 py-1 text-[11px] font-bold text-white"
                        >
                          ERROR: Unknown Item — Click to Map to Espresso Beans
                        </button>
                      ) : (
                        <span className="rounded bg-emerald-500/15 px-2 py-0.5 text-[11px] font-semibold text-emerald-600 dark:text-emerald-400">
                          Mapped: {line.mappedIngredient}
                        </span>
                      )}

                      {isPriceSpike && (
                        <span className="inline-flex items-center gap-1 rounded bg-amber-500/15 px-2 py-0.5 text-[11px] font-semibold text-amber-700 dark:text-amber-300">
                          <AlertTriangle className="h-3 w-3" />
                          +{priceVarPct.toFixed(1)}% vs last paid (PKR {line.lastPaid})
                        </span>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Immutable Reviewer Edit History */}
          <div className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-4">
            <div className="flex items-center gap-1.5 text-xs font-bold">
              <History className="h-3.5 w-3.5 text-terracotta" />
              Immutable Reviewer Edit History
            </div>
            <ul className="mt-2 space-y-1 font-mono text-[11px] text-[var(--ink-muted)]">
              {editHistory.map((h, i) => (
                <li key={i}>• {h}</li>
              ))}
            </ul>
          </div>
        </section>
      </div>
    </AppShell>
  );
}
