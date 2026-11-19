"use client";

import React, { useState } from "react";
import { CheckCircle2, FileCheck2, FileText, LayoutDashboard, Scale, ShieldCheck } from "lucide-react";
import {
  AppShell,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
} from "@hospitality/ui";

interface MatchLine {
  id: string;
  item: string;
  poQty: number;
  grnQty: number;
  billedQty: number;
  unit: string;
  unitPrice: number;
}

export default function ThreeWayMatchingPage() {
  const [lines] = useState<MatchLine[]>([
    {
      id: "m1",
      item: "Barista Oat Milk (ING-MILK-OAT)",
      poQty: 20,
      grnQty: 12,
      billedQty: 12,
      unit: "L",
      unitPrice: 850,
    },
    {
      id: "m2",
      item: "House Espresso Beans (ING-BEANS-ESP)",
      poQty: 10,
      grnQty: 8,
      billedQty: 10,
      unit: "kg",
      unitPrice: 4500,
    },
  ]);

  const [posted, setPosted] = useState(false);

  // Approved = min(grnQty, billedQty) * unitPrice; Held = max(0, billedQty - grnQty) * unitPrice
  const approvedGoods = lines.reduce(
    (sum, l) => sum + Math.min(l.grnQty, l.billedQty) * l.unitPrice,
    0
  );
  const heldDisputed = lines.reduce(
    (sum, l) => sum + Math.max(0, l.billedQty - l.grnQty) * l.unitPrice,
    0
  );
  const deliveryCharge = 300;
  const taxRounding = 0.4;
  const approvedPayable = approvedGoods + deliveryCharge + taxRounding;

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
      id: "matching",
      label: "Procurement · 3-Way Matching",
      icon: <Scale className="h-4 w-4" />,
      active: true,
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="3-Way Match & AP Posting"
      navItems={navItems}
      topBarLeft={
        <h1 className="text-sm font-bold text-[var(--ink-primary)]">
          Line-by-Line 3-Way Matching (PO + GRN + Vendor Invoice)
        </h1>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="Balanced Double-Entry" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="Approved Ready to Post"
            value={`PKR ${approvedPayable.toLocaleString(undefined, { minimumFractionDigits: 2 })}`}
            deltaPercent={0}
            comparisonLabel="physically confirmed GRN"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Held in Dispute / Review"
            value={`PKR ${heldDisputed.toLocaleString(undefined, { minimumFractionDigits: 2 })}`}
            deltaPercent={0}
            comparisonLabel="over-billed vs GRN"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Partial Shipments Open"
            value="1"
            deltaPercent={0}
            comparisonLabel="8 L remaining on PO-091"
            freshnessLabel="today"
          />
          <KpiMetricCard
            title="Duplicate Payables Blocked"
            value="0 extra"
            deltaPercent={0}
            comparisonLabel="idempotent AP ledger"
            freshnessLabel="enforced"
          />
        </div>

        {/* 3-Way Match Matrix */}
        <section className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--surface-border)] pb-3">
            <div>
              <h2 className="text-sm font-bold">3-Way Reconciliation Matrix · PO-2026-091 / GRN-402 / INV-0042</h2>
              <p className="text-xs text-[var(--ink-muted)]">
                Only physically confirmed GRN quantities proceed to Accounts Payable; over-billed quantities stay held in review.
              </p>
            </div>
            <button
              type="button"
              disabled={posted}
              onClick={() => setPosted(true)}
              className="inline-flex items-center gap-1.5 rounded-lg bg-terracotta px-4 py-2 text-xs font-bold text-white hover:bg-terracotta-700 disabled:opacity-50"
            >
              <CheckCircle2 className="h-4 w-4" />
              {posted ? "Payable Posted (Idempotent) ✓" : "Approve & Post Balanced Payable"}
            </button>
          </div>

          <div className="mt-4 overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-[var(--surface-border)] text-[11px] uppercase text-[var(--ink-muted)]">
                  <th className="py-2">Ingredient</th>
                  <th className="py-2 text-right">PO Ordered</th>
                  <th className="py-2 text-right">GRN Received</th>
                  <th className="py-2 text-right">Invoice Billed</th>
                  <th className="py-2 text-right">Approved (PKR)</th>
                  <th className="py-2 text-right">Held in Review</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--surface-border)] font-mono">
                {lines.map((l) => {
                  const over = Math.max(0, l.billedQty - l.grnQty);
                  const appAmt = Math.min(l.grnQty, l.billedQty) * l.unitPrice;
                  const heldAmt = over * l.unitPrice;
                  return (
                    <tr key={l.id}>
                      <td className="py-3 font-sans font-semibold">{l.item}</td>
                      <td className="py-3 text-right">{l.poQty} {l.unit}</td>
                      <td className="py-3 text-right text-emerald-600 dark:text-emerald-400">{l.grnQty} {l.unit}</td>
                      <td className="py-3 text-right">{l.billedQty} {l.unit}</td>
                      <td className="py-3 text-right font-bold">{appAmt.toLocaleString()}</td>
                      <td className="py-3 text-right">
                        {heldAmt > 0 ? (
                          <span className="rounded bg-rose-500/15 px-2 py-0.5 font-bold text-rose-600 dark:text-rose-400">
                            PKR {heldAmt.toLocaleString()} (Over-billed)
                          </span>
                        ) : (
                          <span className="text-emerald-600 dark:text-emerald-400">Matched ✓</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </section>

        {/* Balanced Double-Entry Journal Preview */}
        <section className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5">
          <div className="flex items-center gap-2 text-xs font-bold">
            <ShieldCheck className="h-4 w-4 text-emerald-500" />
            Balanced Double-Entry Journal Preview (SUM Debit == SUM Credit)
          </div>
          <div className="mt-3 space-y-1.5 font-mono text-xs">
            <div className="flex justify-between">
              <span>DR 1200 · Inventory / GRNI Clearing</span>
              <span>PKR {approvedGoods.toLocaleString(undefined, { minimumFractionDigits: 2 })}</span>
            </div>
            <div className="flex justify-between">
              <span>DR 5150 · Freight &amp; Delivery Inward</span>
              <span>PKR {deliveryCharge.toFixed(2)}</span>
            </div>
            <div className="flex justify-between">
              <span>DR 5990 · Tax Rounding Variance</span>
              <span>PKR {taxRounding.toFixed(2)}</span>
            </div>
            <div className="flex justify-between border-t border-[var(--surface-border)] pt-1.5 font-bold text-terracotta">
              <span>CR 2100 · Accounts Payable — Supplier</span>
              <span>PKR {approvedPayable.toLocaleString(undefined, { minimumFractionDigits: 2 })}</span>
            </div>
          </div>
          <div className="mt-3 flex items-center gap-1.5 text-[11px] text-[var(--ink-muted)]">
            <FileCheck2 className="h-3.5 w-3.5" />
            Direct Purchase (No-PO) requires separate Manager Authorization + Physical GRN Confirmation before posting.
          </div>
        </section>
      </div>
    </AppShell>
  );
}
