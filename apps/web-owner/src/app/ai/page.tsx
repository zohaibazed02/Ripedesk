"use client";

import React, { useState } from "react";
import Link from "next/link";

const CLOUD_API_URL =
  process.env.NEXT_PUBLIC_CLOUD_API_URL || "http://localhost:8080";

interface ContributingFactor {
  item: string;
  branch: string;
  category: "ingredient_cost_spike" | "stock_variance" | "waste" | "discount_mix";
  impactPKR: number;
  impactPct: number;
}

interface HumanSuggestion {
  type: "reorder_quantity" | "waste_investigation" | "stock_count_audit";
  title: string;
  action: string;
  impactPKR: number;
  href: string;
}

interface GroundedAnswer {
  question: string;
  summary: string;
  dateRange: string;
  branches: string[];
  warnings: string[];
  factors: ContributingFactor[];
  suggestions: HumanSuggestion[];
  toolsInvoked: string[];
  modelVersion: string;
}

const PRESET_ANSWERS: Record<string, GroundedAnswer> = {
  margins: {
    question: "Why did margins drop this week?",
    summary:
      "Gross margin slipped 2.4% (PKR 34,050) across F-7 Coffee Bar and G-9 Flagship Dine-In, driven primarily by an Oat Milk supplier price spike (+21.4%), unexplained Espresso Bean stock variance (+10.0%), and Tiramisu prep waste at G-9.",
    dateRange: "2026-11-14 → 2026-11-20",
    branches: ["F-7 Coffee Bar", "G-9 Flagship Dine-In"],
    warnings: [
      "G-9 Flagship Dine-In has 3 pending offline orders (last sync 9m ago).",
      "G-9 Flagship Dine-In is missing its closing physical stock count for Nov 19.",
    ],
    factors: [
      {
        item: "Oat Milk (1L)",
        branch: "F-7 Coffee Bar",
        category: "ingredient_cost_spike",
        impactPKR: 18450,
        impactPct: 21.4,
      },
      {
        item: "Espresso Beans (1kg)",
        branch: "G-9 Flagship Dine-In",
        category: "stock_variance",
        impactPKR: 9200,
        impactPct: 10.0,
      },
      {
        item: "Tiramisu Prep",
        branch: "G-9 Flagship Dine-In",
        category: "waste",
        impactPKR: 6400,
        impactPct: 4.8,
      },
    ],
    suggestions: [
      {
        type: "reorder_quantity",
        title: "Review Oat Milk reorder (36 cartons)",
        action:
          "F-7 Coffee Bar is at 14 cartons vs 30-carton safety stock. Review alternative supplier quote before approving PO.",
        impactPKR: 25200,
        href: "/procurement/purchase-orders",
      },
      {
        type: "waste_investigation",
        title: "Audit Espresso Bean & Tiramisu variance at G-9",
        action:
          "Compare grinder calibration and evening prep logs against theoretical recipe usage.",
        impactPKR: 15600,
        href: "/inventory/recipes",
      },
    ],
    toolsInvoked: [
      "get_margin_drivers",
      "get_branch_freshness_and_gaps",
      "get_waste_and_variance_hotspots",
      "get_reorder_suggestions",
    ],
    modelVersion: "grounded-analyst-v1.0",
  },
  freshness: {
    question: "Which branches have stale data or missing stock counts?",
    summary:
      "F-7 Coffee Bar is fully synced (1m ago) with up-to-date stock counts. G-9 Flagship Dine-In is flagged stale (9m since last sync, 3 offline orders queued) and has an overdue closing stock count.",
    dateRange: "2026-11-20 (Live)",
    branches: ["F-7 Coffee Bar", "G-9 Flagship Dine-In"],
    warnings: [
      "G-9 Flagship Dine-In: 3 offline orders pending sync — P&L figures may shift once replayed.",
      "G-9 Flagship Dine-In: Missing physical stock count for Nov 19.",
    ],
    factors: [],
    suggestions: [
      {
        type: "stock_count_audit",
        title: "Complete closing stock count at G-9 Flagship Dine-In",
        action:
          "Notify G-9 branch manager to submit physical count before weekly close.",
        impactPKR: 0,
        href: "/inventory/ledger",
      },
    ],
    toolsInvoked: ["get_branch_freshness_and_gaps"],
    modelVersion: "grounded-analyst-v1.0",
  },
  reorders: {
    question: "Suggest reorder quantities for human review",
    summary:
      "2 low-stock ingredients require human-review PO approval based on 3-day lead time, safety stock, and zero open inbound POs.",
    dateRange: "2026-11-20",
    branches: ["F-7 Coffee Bar", "G-9 Flagship Dine-In"],
    warnings: [],
    factors: [],
    suggestions: [
      {
        type: "reorder_quantity",
        title: "Oat Milk (1L) — F-7 Coffee Bar (36 cartons)",
        action: "Current 14 cartons vs 30 safety stock. Advisory only — no PO auto-created.",
        impactPKR: 25200,
        href: "/procurement/purchase-orders",
      },
      {
        type: "reorder_quantity",
        title: "House Espresso Beans — G-9 Flagship (18 kg)",
        action: "Current 6 kg vs 15 kg safety stock. Advisory only — requires manager approval.",
        impactPKR: 43200,
        href: "/procurement/purchase-orders",
      },
    ],
    toolsInvoked: ["get_reorder_suggestions", "get_branch_freshness_and_gaps"],
    modelVersion: "grounded-analyst-v1.0",
  },
  waste: {
    question: "Where is our highest kitchen waste & variance?",
    summary:
      "G-9 Flagship Dine-In accounts for 78% of recorded waste & unexplained variance this week, led by Espresso Beans (PKR 9,200 variance) and Tiramisu prep waste (PKR 6,400).",
    dateRange: "2026-11-14 → 2026-11-20",
    branches: ["F-7 Coffee Bar", "G-9 Flagship Dine-In"],
    warnings: [
      "G-9 Flagship Dine-In is missing its closing stock count — variance may be understated.",
    ],
    factors: [
      {
        item: "Espresso Beans (1kg)",
        branch: "G-9 Flagship Dine-In",
        category: "stock_variance",
        impactPKR: 9200,
        impactPct: 10.0,
      },
      {
        item: "Tiramisu Prep",
        branch: "G-9 Flagship Dine-In",
        category: "waste",
        impactPKR: 6400,
        impactPct: 4.8,
      },
    ],
    suggestions: [
      {
        type: "waste_investigation",
        title: "Investigate G-9 Espresso & Tiramisu variance",
        action: "Review recipe yields and shift-level waste logs with kitchen lead.",
        impactPKR: 15600,
        href: "/inventory/ledger",
      },
    ],
    toolsInvoked: ["get_waste_and_variance_hotspots", "get_branch_freshness_and_gaps"],
    modelVersion: "grounded-analyst-v1.0",
  },
};

export default function OwnerAIAnalystPage() {
  const [selectedKey, setSelectedKey] = useState<string>("margins");
  const [customQuery, setCustomQuery] = useState("");
  const [liveAnswer, setLiveAnswer] = useState<GroundedAnswer | null>(null);
  const [loading, setLoading] = useState(false);
  const [apiNotice, setApiNotice] = useState<string | null>(null);
  const answer = liveAnswer ?? PRESET_ANSWERS[selectedKey] ?? PRESET_ANSWERS.margins;

  async function askLiveAI(question: string, fallbackKey: string) {
    setSelectedKey(fallbackKey);
    setLoading(true);
    setApiNotice(null);
    const today = new Date().toISOString().slice(0, 10);
    const token =
      typeof window !== "undefined" ? localStorage.getItem("owner_access_token") || "" : "";
    try {
      const res = await fetch(`${CLOUD_API_URL}/api/v1/ai/ask`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ question, from: today, to: today }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const json = (await res.json()) as {
        data?: {
          question?: string;
          answer_summary?: string;
          date_range?: { from: string; to: string };
          branches_analyzed?: string[];
          data_quality_warnings?: string[];
          audit_metadata?: { model_version?: string; tools_invoked?: string[] };
        };
      };
      if (json.data?.answer_summary) {
        setLiveAnswer({
          ...PRESET_ANSWERS[fallbackKey],
          question: json.data.question || question,
          summary: json.data.answer_summary,
          dateRange: json.data.date_range
            ? `${json.data.date_range.from} → ${json.data.date_range.to}`
            : today,
          branches: json.data.branches_analyzed || [],
          warnings: json.data.data_quality_warnings || [],
          modelVersion: json.data.audit_metadata?.model_version || "grounded-analyst-v1.0",
          toolsInvoked: json.data.audit_metadata?.tools_invoked || [],
        });
      }
    } catch {
      setLiveAnswer(null);
      setApiNotice(`AI API (${CLOUD_API_URL}/api/v1/ai/ask) unreachable or unauthenticated — showing cached grounded example.`);
    } finally {
      setLoading(false);
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const q = customQuery.trim();
    if (!q) return;
    const lower = q.toLowerCase();
    const key =
      lower.includes("stale") || lower.includes("fresh") || lower.includes("count")
        ? "freshness"
        : lower.includes("reorder") || lower.includes("po")
        ? "reorders"
        : lower.includes("waste") || lower.includes("variance")
        ? "waste"
        : "margins";
    void askLiveAI(q, key);
    setCustomQuery("");
  };

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100 pb-24 md:pb-10">
      <header className="sticky top-0 z-30 border-b border-zinc-800 bg-zinc-950/90 backdrop-blur">
        <div className="mx-auto max-w-5xl px-4 py-3 flex items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <Link
              href="/"
              className="h-9 w-9 rounded-xl bg-emerald-600 text-white font-bold flex items-center justify-center text-sm"
            >
              HO
            </Link>
            <div>
              <h1 className="text-base font-semibold leading-tight">
                AI Reporting Analyst
              </h1>
              <p className="text-xs text-zinc-400">
                Grounded in Reconciled POS, Inventory &amp; GL Tools
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <span className="px-2.5 py-1 rounded-full text-xs font-medium bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
              Read-Only • Tenant-Scoped
            </span>
            <Link
              href="/"
              className="px-3 py-1.5 rounded-lg text-xs font-medium bg-zinc-900 border border-zinc-800 text-zinc-200 hover:bg-zinc-800"
            >
              ← Overview
            </Link>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-4 py-5 space-y-5">
        {/* Quick-Ask Chips + Input */}
        <section className="rounded-xl border border-zinc-800 bg-zinc-900/90 p-4 space-y-3">
          <form onSubmit={handleSubmit} className="flex gap-2">
            <input
              type="text"
              value={customQuery}
              onChange={(e) => setCustomQuery(e.target.value)}
              placeholder="Ask about margins, waste, stale branches, or reorder suggestions..."
              className="flex-1 rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-500 focus:outline-none focus:border-emerald-500"
            />
            <button
              type="submit"
              className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-sm font-semibold"
            >
              Ask
            </button>
          </form>

          <div className="flex flex-wrap gap-2">
            {(
              [
                ["margins", "Why did margins drop this week?"],
                ["freshness", "Which branches have stale data or missing stock counts?"],
                ["reorders", "Suggest reorder quantities for human review"],
                ["waste", "Where is our highest kitchen waste & variance?"],
              ] as const
            ).map(([key, label]) => (
                <button
                  key={key}
                  disabled={loading}
                  onClick={() => void askLiveAI(label, key)}
                  className={`px-3 py-1.5 rounded-full text-xs font-medium transition border ${
                    selectedKey === key
                      ? "bg-emerald-600 text-white border-emerald-500"
                      : "bg-zinc-950 text-zinc-300 border-zinc-800 hover:border-zinc-700"
                  }`}
                >
                  {label}
                </button>
            ))}
          </div>
        </section>

        {/* Grounded Answer Workspace */}
        {apiNotice && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3.5 py-2 text-xs text-amber-300">
            {apiNotice}
          </div>
        )}
        <section className="rounded-xl border border-zinc-800 bg-zinc-900/90 p-5 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-base font-semibold">{answer.question}</h2>
            <div className="flex flex-wrap items-center gap-1.5 text-[11px]">
              <span className="px-2 py-0.5 rounded-md bg-zinc-800 text-zinc-300">
                {answer.dateRange}
              </span>
              {answer.branches.map((b) => (
                <span
                  key={b}
                  className="px-2 py-0.5 rounded-md bg-zinc-800 text-zinc-300"
                >
                  {b}
                </span>
              ))}
            </div>
          </div>

          <p className="text-sm text-zinc-200 leading-relaxed">
            {answer.summary}
          </p>

          {/* Data Freshness & Gaps Callout */}
          {answer.warnings.length > 0 && (
            <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3.5 space-y-1">
              <p className="text-xs font-semibold text-amber-400">
                Data Freshness &amp; Completeness Warnings
              </p>
              <ul className="list-disc list-inside text-xs text-amber-200/90 space-y-0.5">
                {answer.warnings.map((w) => (
                  <li key={w}>{w}</li>
                ))}
              </ul>
            </div>
          )}

          {/* Contributing Factors Table */}
          {answer.factors.length > 0 && (
            <div className="overflow-x-auto rounded-lg border border-zinc-800">
              <table className="w-full text-xs text-left">
                <thead className="bg-zinc-950/70 text-zinc-400">
                  <tr>
                    <th className="px-3.5 py-2 font-medium">Item / Ingredient</th>
                    <th className="px-3.5 py-2 font-medium">Branch</th>
                    <th className="px-3.5 py-2 font-medium">Driver</th>
                    <th className="px-3.5 py-2 font-medium text-right">Impact (PKR)</th>
                    <th className="px-3.5 py-2 font-medium text-right">Impact (%)</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-800 tabular-nums">
                  {answer.factors.map((f) => (
                    <tr key={`${f.item}-${f.branch}`}>
                      <td className="px-3.5 py-2.5 font-medium">{f.item}</td>
                      <td className="px-3.5 py-2.5 text-zinc-300">{f.branch}</td>
                      <td className="px-3.5 py-2.5">
                        <span className="px-2 py-0.5 rounded bg-zinc-800 text-zinc-300">
                          {f.category.replace(/_/g, " ")}
                        </span>
                      </td>
                      <td className="px-3.5 py-2.5 text-right font-semibold text-rose-400">
                        PKR {f.impactPKR.toLocaleString("en-PK")}
                      </td>
                      <td className="px-3.5 py-2.5 text-right text-rose-400">
                        +{f.impactPct.toFixed(1)}%
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Human-Review Suggestions */}
          {answer.suggestions.length > 0 && (
            <div className="space-y-2.5">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-zinc-400">
                Suggestions for Human Review
              </h3>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {answer.suggestions.map((s) => (
                  <div
                    key={s.title}
                    className="rounded-lg border border-zinc-800 bg-zinc-950/70 p-3.5 space-y-2"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30">
                        Advisory — Human Review Required
                      </span>
                      {s.impactPKR > 0 && (
                        <span className="text-xs font-semibold tabular-nums text-zinc-300">
                          Est. PKR {s.impactPKR.toLocaleString("en-PK")}
                        </span>
                      )}
                    </div>
                    <p className="text-sm font-semibold">{s.title}</p>
                    <p className="text-xs text-zinc-400">{s.action}</p>
                    <a
                      href={s.href}
                      className="inline-block text-xs font-medium text-emerald-400 hover:underline"
                    >
                      Verify in Admin ERP ({s.href}) →
                    </a>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Underlying Report Links & Model Audit Footer */}
          <div className="pt-3 border-t border-zinc-800 flex flex-wrap items-center justify-between gap-3 text-[11px] text-zinc-400">
            <div className="flex flex-wrap items-center gap-2">
              {[
                ["P&L Statements", "/accounting/statements"],
                ["Inventory Ledger", "/inventory/ledger"],
                ["Purchase Orders", "/procurement/purchase-orders"],
                ["Recipes", "/inventory/recipes"],
              ].map(([label, href]) => (
                <a
                  key={href}
                  href={href}
                  className="px-2 py-0.5 rounded bg-zinc-800 text-zinc-300 hover:text-white"
                >
                  {label}
                </a>
              ))}
            </div>
            <p className="font-mono">
              Model: {answer.modelVersion} • Tools: {answer.toolsInvoked.join(", ")}
            </p>
          </div>
        </section>
      </main>
    </div>
  );
}
