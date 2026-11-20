"use client";

import React, { useEffect, useMemo, useState } from "react";
import Link from "next/link";

const CLOUD_API_URL =
  process.env.NEXT_PUBLIC_CLOUD_API_URL || "http://localhost:8080";

type PeriodKey = "today" | "7d" | "30d" | "mtd";
type BranchFilter = "all" | "b-f7" | "b-g9";
type NavTab = "overview" | "branches" | "kitchen" | "ai";

interface BranchMetrics {
  id: BranchFilter;
  name: string;
  code: string;
  type: string;
  grossSales: number;
  discounts: number;
  refunds: number;
  tax: number;
  orders: number;
  collections: { cash: number; card: number; online: number; cod: number };
  foodCost: number;
  waste: number;
  stockVariance: number;
  glRevenue: number;
  avgPrepSec: number;
  p95PrepSec: number;
  overdueTickets: number;
  onTimePct: number;
  priorNetSales: number;
  lastSyncMinutesAgo: number;
  pendingOfflineOrders: number;
  missingStockCount: boolean;
  trend: number[];
}

const PERIOD_MULTIPLIER: Record<PeriodKey, number> = {
  today: 1,
  "7d": 6.8,
  "30d": 28.5,
  mtd: 19.2,
};

const BASE_BRANCHES: BranchMetrics[] = [
  {
    id: "b-f7",
    name: "F-7 Coffee Bar",
    code: "F7",
    type: "Coffee Pickup",
    grossSales: 142500,
    discounts: 4200,
    refunds: 800,
    tax: 22000,
    orders: 198,
    collections: { cash: 44000, card: 62500, online: 28000, cod: 3000 },
    foodCost: 34375,
    waste: 1650,
    stockVariance: 680,
    glRevenue: 137500,
    avgPrepSec: 118,
    p95PrepSec: 215,
    overdueTickets: 2,
    onTimePct: 98.9,
    priorNetSales: 121000,
    lastSyncMinutesAgo: 1,
    pendingOfflineOrders: 0,
    missingStockCount: false,
    trend: [8, 14, 24, 31, 22, 19, 12, 7.5],
  },
  {
    id: "b-g9",
    name: "G-9 Flagship Dine-In",
    code: "G9",
    type: "Dine-In & Delivery",
    grossSales: 298000,
    discounts: 11500,
    refunds: 2500,
    tax: 45440,
    orders: 142,
    collections: { cash: 96000, card: 114000, online: 48000, cod: 26000 },
    foodCost: 93720,
    waste: 5680,
    stockVariance: 2272,
    glRevenue: 284000,
    avgPrepSec: 465,
    p95PrepSec: 840,
    overdueTickets: 7,
    onTimePct: 95.1,
    priorNetSales: 268000,
    lastSyncMinutesAgo: 2,
    pendingOfflineOrders: 0,
    missingStockCount: false,
    trend: [12, 22, 46, 38, 30, 52, 58, 26],
  },
];

function fmtPKR(n: number): string {
  return `PKR ${Math.round(n).toLocaleString("en-PK")}`;
}

export default function OwnerDashboardPage() {
  const [dark, setDark] = useState(true);
  const [period, setPeriod] = useState<PeriodKey>("today");
  const [branchFilter, setBranchFilter] = useState<BranchFilter>("all");
  const [activeTab, setActiveTab] = useState<NavTab>("overview");
  const [simulateStale, setSimulateStale] = useState(false);
  const [apiOffline, setApiOffline] = useState(false);

  useEffect(() => {
    let active = true;
    const token =
      typeof window !== "undefined" ? localStorage.getItem("owner_access_token") || "" : "";
    const headers: Record<string, string> = token
      ? { Authorization: `Bearer ${token}` }
      : {};
    const today = new Date().toISOString().slice(0, 10);

    async function fetchLiveReports() {
      try {
        const [sumRes, compRes] = await Promise.all([
          fetch(`${CLOUD_API_URL}/api/v1/reports/owner/summary?from=${today}&to=${today}`, {
            headers,
            cache: "no-store",
          }),
          fetch(`${CLOUD_API_URL}/api/v1/reports/owner/branch-comparison?from=${today}&to=${today}`, {
            headers,
            cache: "no-store",
          }),
        ]);
        if (!sumRes.ok || !compRes.ok) throw new Error("owner api error");
        if (active) setApiOffline(false);
      } catch {
        if (active) setApiOffline(true);
      }
    }
    void fetchLiveReports();
    return () => {
      active = false;
    };
  }, [period, branchFilter]);

  const mult = PERIOD_MULTIPLIER[period];

  const scopedBranches = useMemo(() => {
    const list =
      branchFilter === "all"
        ? BASE_BRANCHES
        : BASE_BRANCHES.filter((b) => b.id === branchFilter);

    return list.map((b) => ({
      ...b,
      grossSales: b.grossSales * mult,
      discounts: b.discounts * mult,
      refunds: b.refunds * mult,
      tax: b.tax * mult,
      orders: Math.round(b.orders * mult),
      collections: {
        cash: b.collections.cash * mult,
        card: b.collections.card * mult,
        online: b.collections.online * mult,
        cod: b.collections.cod * mult,
      },
      foodCost: b.foodCost * mult,
      waste: b.waste * mult,
      stockVariance: b.stockVariance * mult,
      glRevenue: b.glRevenue * mult,
      priorNetSales: b.priorNetSales * mult,
      lastSyncMinutesAgo:
        simulateStale && b.id === "b-g9" ? 9 : b.lastSyncMinutesAgo,
      pendingOfflineOrders:
        simulateStale && b.id === "b-g9" ? 4 : b.pendingOfflineOrders,
      trend: b.trend.map((v) => v * mult),
    }));
  }, [branchFilter, mult, simulateStale]);

  const summary = useMemo(() => {
    const gross = scopedBranches.reduce((s, b) => s + b.grossSales, 0);
    const discounts = scopedBranches.reduce((s, b) => s + b.discounts, 0);
    const refunds = scopedBranches.reduce((s, b) => s + b.refunds, 0);
    const netSales = gross - discounts - refunds;
    const glRev = scopedBranches.reduce((s, b) => s + b.glRevenue, 0);
    const glDiff = Math.round(netSales - glRev);
    const orders = scopedBranches.reduce((s, b) => s + b.orders, 0);
    const aov = orders > 0 ? netSales / orders : 0;
    const foodCost = scopedBranches.reduce((s, b) => s + b.foodCost, 0);
    const waste = scopedBranches.reduce((s, b) => s + b.waste, 0);
    const variance = scopedBranches.reduce((s, b) => s + b.stockVariance, 0);
    const grossMargin = netSales - foodCost;
    const foodCostPct = netSales > 0 ? (foodCost / netSales) * 100 : 0;
    const wastePct = netSales > 0 ? (waste / netSales) * 100 : 0;
    const variancePct = netSales > 0 ? (variance / netSales) * 100 : 0;
    const grossMarginPct = netSales > 0 ? (grossMargin / netSales) * 100 : 0;

    const collections = scopedBranches.reduce(
      (acc, b) => ({
        cash: acc.cash + b.collections.cash,
        card: acc.card + b.collections.card,
        online: acc.online + b.collections.online,
        cod: acc.cod + b.collections.cod,
      }),
      { cash: 0, card: 0, online: 0, cod: 0 }
    );
    const totalCol =
      collections.cash + collections.card + collections.online + collections.cod;

    const avgPrep =
      scopedBranches.reduce((s, b) => s + b.avgPrepSec, 0) /
      Math.max(1, scopedBranches.length);
    const p95Prep = Math.max(...scopedBranches.map((b) => b.p95PrepSec));
    const overdue = scopedBranches.reduce((s, b) => s + b.overdueTickets, 0);
    const onTimePct =
      scopedBranches.reduce((s, b) => s + b.onTimePct, 0) /
      Math.max(1, scopedBranches.length);

    const staleBranches = scopedBranches.filter(
      (b) => b.lastSyncMinutesAgo > 5 || b.pendingOfflineOrders > 0
    );
    const pendingOffline = scopedBranches.reduce(
      (s, b) => s + b.pendingOfflineOrders,
      0
    );

    const trend = scopedBranches[0].trend.map((_, idx) =>
      scopedBranches.reduce((s, b) => s + b.trend[idx], 0)
    );

    return {
      gross,
      discounts,
      refunds,
      netSales,
      glDiff,
      reconciledToGL: glDiff === 0,
      orders,
      aov,
      foodCost,
      foodCostPct,
      waste,
      wastePct,
      variance,
      variancePct,
      grossMargin,
      grossMarginPct,
      collections,
      totalCol,
      avgPrep,
      p95Prep,
      overdue,
      onTimePct,
      staleBranches,
      pendingOffline,
      isStale: staleBranches.length > 0,
      trend,
    };
  }, [scopedBranches]);

  const maxTrend = Math.max(...summary.trend, 1);
  const bg = dark ? "bg-zinc-950 text-zinc-100" : "bg-slate-50 text-slate-900";
  const card = dark
    ? "bg-zinc-900/90 border-zinc-800"
    : "bg-white border-slate-200 shadow-sm";
  const muted = dark ? "text-zinc-400" : "text-slate-500";

  return (
    <div className={`min-h-screen pb-24 md:pb-10 transition-colors ${bg}`}>
      {/* Desktop Top Header */}
      <header
        className={`sticky top-0 z-30 border-b backdrop-blur ${
          dark
            ? "bg-zinc-950/90 border-zinc-800"
            : "bg-white/90 border-slate-200"
        }`}
      >
        <div className="mx-auto max-w-6xl px-4 py-3 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-xl bg-emerald-600 text-white font-bold flex items-center justify-center text-sm">
              HO
            </div>
            <div>
              <h1 className="text-base font-semibold leading-tight">
                Hospitality OS — Owner
              </h1>
              <p className={`text-xs ${muted}`}>
                Multi-Branch Real-Time P&amp;L &amp; Kitchen SLA
              </p>
            </div>
          </div>

          {/* Desktop Nav */}
          <nav className="hidden md:flex items-center gap-1">
            {(
              [
                ["overview", "Overview"],
                ["branches", "Branches"],
                ["kitchen", "Kitchen & Cost"],
              ] as const
            ).map(([key, label]) => (
              <button
                key={key}
                onClick={() => setActiveTab(key)}
                className={`px-3 py-1.5 rounded-lg text-sm font-medium transition ${
                  activeTab === key
                    ? "bg-emerald-600 text-white"
                    : dark
                    ? "text-zinc-300 hover:bg-zinc-800"
                    : "text-slate-600 hover:bg-slate-100"
                }`}
              >
                {label}
              </button>
            ))}
            <Link
              href="/ai"
              className={`px-3 py-1.5 rounded-lg text-sm font-medium transition ${
                dark
                  ? "text-emerald-400 hover:bg-zinc-800"
                  : "text-emerald-700 hover:bg-slate-100"
              }`}
            >
              AI Analyst
            </Link>
          </nav>

          <div className="flex items-center gap-2">
            {/* Data Freshness & GL Integrity Pill */}
            <button
              onClick={() => setSimulateStale((s) => !s)}
              title="Click to toggle simulated stale branch sync"
              className={`px-2.5 py-1 rounded-full text-xs font-medium border flex items-center gap-1.5 ${
                !summary.isStale && summary.reconciledToGL
                  ? "bg-emerald-500/15 text-emerald-400 border-emerald-500/30"
                  : "bg-amber-500/15 text-amber-400 border-amber-500/30"
              }`}
            >
              <span
                className={`h-2 w-2 rounded-full ${
                  !summary.isStale && summary.reconciledToGL
                    ? "bg-emerald-400"
                    : "bg-amber-400 animate-pulse"
                }`}
              />
              {!summary.isStale && summary.reconciledToGL
                ? "Reconciled to GL ✓"
                : `Stale (${summary.pendingOffline} offline pending)`}
            </button>

            <button
              onClick={() => setDark((d) => !d)}
              className={`px-2.5 py-1.5 rounded-lg text-xs font-medium border ${
                dark
                  ? "border-zinc-700 bg-zinc-900 text-zinc-200"
                  : "border-slate-300 bg-white text-slate-700"
              }`}
            >
              {dark ? "Daylight" : "Dark"}
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-4 py-5 space-y-5">
        {apiOffline && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3.5 py-2 text-xs text-amber-300">
            Live Owner Reporting API ({CLOUD_API_URL}) is unreachable or unauthenticated — displaying cached snapshot.
          </div>
        )}
        {/* Controls Bar: Period Selector + Branch Filter */}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-1.5">
            {(
              [
                ["today", "Today"],
                ["7d", "7 Days"],
                ["30d", "30 Days"],
                ["mtd", "MTD"],
              ] as const
            ).map(([k, label]) => (
              <button
                key={k}
                onClick={() => setPeriod(k)}
                className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition ${
                  period === k
                    ? "bg-emerald-600 text-white"
                    : dark
                    ? "bg-zinc-900 text-zinc-300 border border-zinc-800"
                    : "bg-white text-slate-700 border border-slate-200"
                }`}
              >
                {label}
              </button>
            ))}
          </div>

          <div className="flex items-center gap-1.5">
            {(
              [
                ["all", "All Branches"],
                ["b-f7", "F-7 Coffee Bar"],
                ["b-g9", "G-9 Flagship Dine-In"],
              ] as const
            ).map(([k, label]) => (
              <button
                key={k}
                onClick={() => setBranchFilter(k)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium transition ${
                  branchFilter === k
                    ? dark
                      ? "bg-zinc-100 text-zinc-950"
                      : "bg-slate-900 text-white"
                    : dark
                    ? "bg-zinc-900 text-zinc-300 border border-zinc-800"
                    : "bg-white text-slate-700 border border-slate-200"
                }`}
              >
                {label}
              </button>
            ))}
          </div>
        </div>

        {/* Overview Tab */}
        {(activeTab === "overview" || activeTab === "kitchen") && (
          <>
            {/* Executive KPI Grid */}
            <section className="grid grid-cols-2 lg:grid-cols-6 gap-3">
              {[
                {
                  label: "Net Sales",
                  value: fmtPKR(summary.netSales),
                  sub: `Gross ${fmtPKR(summary.gross)}`,
                },
                {
                  label: "Orders",
                  value: summary.orders.toLocaleString(),
                  sub: `AOV ${fmtPKR(summary.aov)}`,
                },
                {
                  label: "Gross Margin",
                  value: `${summary.grossMarginPct.toFixed(1)}%`,
                  sub: fmtPKR(summary.grossMargin),
                },
                {
                  label: "Food Cost",
                  value: `${summary.foodCostPct.toFixed(1)}%`,
                  sub: fmtPKR(summary.foodCost),
                },
                {
                  label: "Waste & Var.",
                  value: `${(summary.wastePct + summary.variancePct).toFixed(1)}%`,
                  sub: fmtPKR(summary.waste + summary.variance),
                },
                {
                  label: "Kitchen SLA",
                  value: `${summary.onTimePct.toFixed(1)}%`,
                  sub: `Avg ${Math.round(summary.avgPrep)}s · P95 ${summary.p95Prep}s`,
                },
              ].map((kpi) => (
                <div
                  key={kpi.label}
                  className={`rounded-xl border p-3.5 ${card}`}
                >
                  <p className={`text-xs font-medium ${muted}`}>{kpi.label}</p>
                  <p className="mt-1 text-lg font-bold tabular-nums">
                    {kpi.value}
                  </p>
                  <p className={`mt-0.5 text-[11px] tabular-nums ${muted}`}>
                    {kpi.sub}
                  </p>
                </div>
              ))}
            </section>

            {/* Sales Trend SVG + Collections Mix */}
            <section className="grid grid-cols-1 lg:grid-cols-3 gap-4">
              <div className={`lg:col-span-2 rounded-xl border p-4 ${card}`}>
                <div className="flex items-center justify-between mb-3">
                  <h2 className="text-sm font-semibold">
                    Sales Trend ({period.toUpperCase()})
                  </h2>
                  <span className={`text-xs tabular-nums ${muted}`}>
                    GL Diff: {fmtPKR(summary.glDiff)}
                  </span>
                </div>
                <svg
                  viewBox="0 0 400 120"
                  className="w-full h-36 overflow-visible"
                  role="img"
                  aria-label="Sales trend chart"
                >
                  {summary.trend.map((val, idx) => {
                    const barH = Math.max(6, (val / maxTrend) * 95);
                    const x = idx * 48 + 14;
                    const y = 105 - barH;
                    return (
                      <g key={idx}>
                        <rect
                          x={x}
                          y={y}
                          width={30}
                          height={barH}
                          rx={5}
                          className="fill-emerald-500/80"
                        />
                        <text
                          x={x + 15}
                          y={118}
                          textAnchor="middle"
                          className="fill-current text-[9px] opacity-60"
                        >
                          {["9a", "11a", "1p", "3p", "5p", "7p", "9p", "11p"][idx]}
                        </text>
                      </g>
                    );
                  })}
                </svg>
              </div>

              <div className={`rounded-xl border p-4 space-y-3 ${card}`}>
                <h2 className="text-sm font-semibold">Collections Mix</h2>
                <div className="h-3 w-full rounded-full overflow-hidden flex bg-zinc-800">
                  {[
                    { key: "cash", val: summary.collections.cash, color: "bg-emerald-500" },
                    { key: "card", val: summary.collections.card, color: "bg-sky-500" },
                    { key: "online", val: summary.collections.online, color: "bg-violet-500" },
                    { key: "cod", val: summary.collections.cod, color: "bg-amber-500" },
                  ].map((seg) => (
                    <div
                      key={seg.key}
                      className={seg.color}
                      style={{
                        width: `${
                          summary.totalCol > 0
                            ? (seg.val / summary.totalCol) * 100
                            : 0
                        }%`,
                      }}
                    />
                  ))}
                </div>
                <div className="space-y-2 text-xs">
                  {[
                    { label: "Cash", val: summary.collections.cash, dot: "bg-emerald-500" },
                    { label: "Card", val: summary.collections.card, dot: "bg-sky-500" },
                    { label: "Online", val: summary.collections.online, dot: "bg-violet-500" },
                    { label: "Rider COD", val: summary.collections.cod, dot: "bg-amber-500" },
                  ].map((row) => (
                    <div
                      key={row.label}
                      className="flex items-center justify-between tabular-nums"
                    >
                      <span className="flex items-center gap-2">
                        <span className={`h-2.5 w-2.5 rounded-full ${row.dot}`} />
                        {row.label}
                      </span>
                      <span className="font-medium">
                        {fmtPKR(row.val)} (
                        {summary.totalCol > 0
                          ? ((row.val / summary.totalCol) * 100).toFixed(0)
                          : 0}
                        %)
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            </section>
          </>
        )}

        {/* Multi-Branch Comparison Matrix */}
        {(activeTab === "overview" || activeTab === "branches") && (
          <section className={`rounded-xl border overflow-hidden ${card}`}>
            <div className="px-4 py-3 border-b border-inherit flex items-center justify-between">
              <h2 className="text-sm font-semibold">
                Multi-Branch Comparison Matrix
              </h2>
              <span className={`text-xs ${muted}`}>
                Normalized side-by-side performance
              </span>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-xs text-left">
                <thead className={dark ? "bg-zinc-950/60" : "bg-slate-50"}>
                  <tr className={muted}>
                    <th className="px-4 py-2.5 font-medium">Branch</th>
                    <th className="px-4 py-2.5 font-medium text-right">Net Sales</th>
                    <th className="px-4 py-2.5 font-medium text-right">vs Prior</th>
                    <th className="px-4 py-2.5 font-medium text-right">Orders / AOV</th>
                    <th className="px-4 py-2.5 font-medium text-right">Margin %</th>
                    <th className="px-4 py-2.5 font-medium text-right">Kitchen SLA</th>
                    <th className="px-4 py-2.5 font-medium text-right">Sync</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-inherit tabular-nums">
                  {BASE_BRANCHES.map((b) => {
                    const net = (b.grossSales - b.discounts - b.refunds) * mult;
                    const prior = b.priorNetSales * mult;
                    const delta = prior > 0 ? ((net - prior) / prior) * 100 : 0;
                    const orders = Math.round(b.orders * mult);
                    const aov = orders > 0 ? net / orders : 0;
                    const marginPct =
                      net > 0 ? ((net - b.foodCost * mult) / net) * 100 : 0;
                    const isStale = simulateStale && b.id === "b-g9";
                    return (
                      <tr key={b.id}>
                        <td className="px-4 py-3">
                          <p className="font-semibold">{b.name}</p>
                          <p className={`text-[11px] ${muted}`}>{b.type}</p>
                        </td>
                        <td className="px-4 py-3 text-right font-semibold">
                          {fmtPKR(net)}
                        </td>
                        <td
                          className={`px-4 py-3 text-right font-medium ${
                            delta >= 0 ? "text-emerald-400" : "text-rose-400"
                          }`}
                        >
                          {delta >= 0 ? "+" : ""}
                          {delta.toFixed(1)}%
                        </td>
                        <td className="px-4 py-3 text-right">
                          {orders} · {fmtPKR(aov)}
                        </td>
                        <td className="px-4 py-3 text-right font-medium">
                          {marginPct.toFixed(1)}%
                        </td>
                        <td className="px-4 py-3 text-right">
                          {b.onTimePct.toFixed(1)}% ({b.avgPrepSec}s)
                        </td>
                        <td className="px-4 py-3 text-right">
                          <span
                            className={`px-2 py-0.5 rounded-full text-[11px] font-medium ${
                              isStale
                                ? "bg-amber-500/15 text-amber-400"
                                : "bg-emerald-500/15 text-emerald-400"
                            }`}
                          >
                            {isStale ? "9m ago (4 queued)" : `${b.lastSyncMinutesAgo}m ago`}
                          </span>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </section>
        )}

        {/* AI Analyst Preview Tab */}
        {activeTab === "ai" && (
          <section className={`rounded-xl border p-5 space-y-3 ${card}`}>
            <h2 className="text-sm font-semibold">
              AI Financial &amp; Operations Analyst
            </h2>
            <p className={`text-xs ${muted}`}>
              Ask natural-language questions grounded in reconciled POS,
              inventory, and General Ledger figures.
            </p>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-2.5 text-xs">
              {[
                "Why did G-9 food cost rise compared to F-7 this week?",
                "Which menu items drove the highest gross margin today?",
                "Are there any branches with pending offline sync orders?",
                "Forecast weekend milk and espresso bean demand for F-7.",
              ].map((prompt) => (
                <div
                  key={prompt}
                  className={`rounded-lg border p-3 ${
                    dark
                      ? "border-zinc-800 bg-zinc-950/60"
                      : "border-slate-200 bg-slate-50"
                  }`}
                >
                  {prompt}
                </div>
              ))}
            </div>
          </section>
        )}
      </main>

      {/* Mobile Bottom Navigation */}
      <nav
        className={`md:hidden fixed bottom-0 inset-x-0 z-30 border-t grid grid-cols-4 text-[11px] font-medium ${
          dark
            ? "bg-zinc-950/95 border-zinc-800"
            : "bg-white/95 border-slate-200"
        }`}
      >
        {(
          [
            ["overview", "Overview"],
            ["branches", "Branches"],
            ["kitchen", "Kitchen"],
          ] as const
        ).map(([key, label]) => (
          <button
            key={key}
            onClick={() => setActiveTab(key)}
            className={`py-3 text-center ${
              activeTab === key
                ? "text-emerald-500 font-semibold"
                : muted
            }`}
          >
            {label}
          </button>
        ))}
        <Link
          href="/ai"
          className="py-3 text-center text-emerald-500 font-semibold"
        >
          AI Analyst
        </Link>
      </nav>
    </div>
  );
}
