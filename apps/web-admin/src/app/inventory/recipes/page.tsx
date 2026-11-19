"use client";

import React, { useMemo, useState } from "react";
import Link from "next/link";
import { Boxes, ChefHat, GitBranch, LayoutDashboard, Sparkles } from "lucide-react";
import {
  AppShell,
  SyncStatusBadge,
  type AppShellNavItem,
} from "@hospitality/ui";

interface RecipeLineDraft {
  id: string;
  ingredientName: string;
  baseUnit: string;
  qty: number;
  wastePct: number;
  unitCostPkr: number;
}

interface RecipeTarget {
  id: string;
  targetType: "menu_item" | "modifier" | "prep_batch";
  name: string;
  version: number;
  menuPricePkr: number;
  lines: RecipeLineDraft[];
}

const PILOT_RECIPES: RecipeTarget[] = [
  {
    id: "rec-oat-latte",
    targetType: "menu_item",
    name: "Large Oat Latte",
    version: 2,
    menuPricePkr: 950,
    lines: [
      { id: "l1", ingredientName: "House Espresso Beans", baseUnit: "g", qty: 18, wastePct: 0, unitCostPkr: 4.5 },
      { id: "l2", ingredientName: "Barista Oat Milk", baseUnit: "ml", qty: 250, wastePct: 2, unitCostPkr: 0.85 },
      { id: "l3", ingredientName: "16oz Double-Wall Paper Cup", baseUnit: "pcs", qty: 1, wastePct: 0, unitCostPkr: 22.0 },
      { id: "l4", ingredientName: "16oz Sip-Through Lid", baseUnit: "pcs", qty: 1, wastePct: 0, unitCostPkr: 8.5 },
    ],
  },
  {
    id: "rec-extra-shot",
    targetType: "modifier",
    name: "Extra Shot (+18g Espresso)",
    version: 1,
    menuPricePkr: 220,
    lines: [
      { id: "l5", ingredientName: "House Espresso Beans", baseUnit: "g", qty: 18, wastePct: 0, unitCostPkr: 4.5 },
    ],
  },
  {
    id: "rec-tiramisu-tray",
    targetType: "prep_batch",
    name: "House Tiramisu Tray (Yield: 12 pcs)",
    version: 1,
    menuPricePkr: 10200,
    lines: [
      { id: "l6", ingredientName: "House Espresso Beans", baseUnit: "g", qty: 180, wastePct: 3, unitCostPkr: 4.5 },
      { id: "l7", ingredientName: "Mascarpone Cream", baseUnit: "g", qty: 1500, wastePct: 2, unitCostPkr: 1.6 },
    ],
  },
];

function computeRecipeCogs(lines: RecipeLineDraft[]): number {
  return lines.reduce(
    (sum, l) => sum + l.qty * (1 + l.wastePct / 100) * l.unitCostPkr,
    0
  );
}

export default function RecipesPage() {
  const [recipes, setRecipes] = useState<RecipeTarget[]>(PILOT_RECIPES);
  const [tab, setTab] = useState<"menu_item" | "modifier" | "prep_batch">("menu_item");
  const [selectedId, setSelectedId] = useState<string>(PILOT_RECIPES[0].id);
  const [includeExtraShot, setIncludeExtraShot] = useState(true);

  const selected = useMemo(
    () => recipes.find((r) => r.id === selectedId) ?? recipes[0],
    [recipes, selectedId]
  );

  const cogs = computeRecipeCogs(selected.lines);
  const foodCostPct = selected.menuPricePkr > 0 ? (cogs / selected.menuPricePkr) * 100 : 0;
  const grossMarginPct = 100 - foodCostPct;

  const foodCostBadgeCls =
    foodCostPct < 28
      ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"
      : foodCostPct <= 35
        ? "bg-amber-500/15 text-amber-600 dark:text-amber-400"
        : "bg-rose-500/15 text-rose-600 dark:text-rose-400";

  function updateLineQty(lineId: string, qty: number) {
    setRecipes((prev) =>
      prev.map((r) =>
        r.id === selected.id
          ? {
              ...r,
              lines: r.lines.map((l) => (l.id === lineId ? { ...l, qty: Math.max(0, qty) } : l)),
            }
          : r
      )
    );
  }

  function publishNewVersion() {
    setRecipes((prev) =>
      prev.map((r) => (r.id === selected.id ? { ...r, version: r.version + 1 } : r))
    );
  }

  // Order Combo Simulator: Large Oat Latte + optional Extra Shot
  const comboBeans = 18 + (includeExtraShot ? 18 : 0);
  const comboPrice = 950 + (includeExtraShot ? 220 : 0);
  const comboCogs = comboBeans * 4.5 + 250 * 0.85 + 22 + 8.5;
  const comboMargin = ((comboPrice - comboCogs) / comboPrice) * 100;

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
      active: true,
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Versioned Recipes & COGS"
      navItems={navItems}
      topBarLeft={
        <div className="flex items-center gap-3">
          <h1 className="text-sm font-bold text-[var(--ink-primary)]">
            Versioned Recipe Builder &amp; Food-Cost Simulator
          </h1>
          <Link
            href="/inventory/ingredients"
            className="rounded-md border border-[var(--surface-border)] px-2.5 py-1 text-xs font-medium text-[var(--ink-secondary)] hover:border-terracotta"
          >
            ← Ingredient Master
          </Link>
        </div>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="Immutable Snapshots" />}
    >
      <div className="mx-auto grid max-w-7xl grid-cols-1 gap-5 lg:grid-cols-12">
        {/* Left Selector */}
        <section className="space-y-3 lg:col-span-4">
          <div className="flex gap-1.5 rounded-lg border border-[var(--surface-border)] bg-[var(--surface-card)] p-1">
            {(
              [
                ["menu_item", "Menu Items"],
                ["modifier", "Modifiers"],
                ["prep_batch", "Prep Batches"],
              ] as const
            ).map(([id, label]) => (
              <button
                key={id}
                type="button"
                onClick={() => setTab(id)}
                className={`flex-1 rounded-md py-1.5 text-xs font-semibold transition-colors ${
                  tab === id
                    ? "bg-terracotta text-white"
                    : "text-[var(--ink-muted)] hover:text-[var(--ink-primary)]"
                }`}
              >
                {label}
              </button>
            ))}
          </div>

          <div className="space-y-2">
            {recipes
              .filter((r) => r.targetType === tab)
              .map((r) => {
                const rCogs = computeRecipeCogs(r.lines);
                const rPct = r.menuPricePkr > 0 ? (rCogs / r.menuPricePkr) * 100 : 0;
                const active = r.id === selected.id;
                return (
                  <button
                    key={r.id}
                    type="button"
                    onClick={() => setSelectedId(r.id)}
                    className={`w-full rounded-xl border p-3.5 text-left transition-colors ${
                      active
                        ? "border-terracotta bg-terracotta/5"
                        : "border-[var(--surface-border)] bg-[var(--surface-card)] hover:border-terracotta/40"
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-bold">{r.name}</span>
                      <span className="rounded bg-[var(--surface-subtle)] px-1.5 py-0.5 font-mono text-[10px] font-bold">
                        v{r.version} Active
                      </span>
                    </div>
                    <div className="mt-2 flex items-center justify-between font-mono text-[11px] text-[var(--ink-muted)]">
                      <span>Price: PKR {r.menuPricePkr.toFixed(0)}</span>
                      <span>COGS: PKR {rCogs.toFixed(2)}</span>
                      <span className="font-bold text-[var(--ink-primary)]">
                        {rPct.toFixed(1)}%
                      </span>
                    </div>
                  </button>
                );
              })}
          </div>

          {/* Live Order Combo Simulator */}
          <div className="rounded-xl border border-terracotta/30 bg-[var(--surface-card)] p-4">
            <div className="flex items-center justify-between">
              <span className="inline-flex items-center gap-1.5 text-xs font-bold text-terracotta">
                <Sparkles className="h-3.5 w-3.5" />
                Order Combo Simulator
              </span>
              <label className="inline-flex items-center gap-1.5 text-[11px] font-semibold">
                <input
                  type="checkbox"
                  checked={includeExtraShot}
                  onChange={(e) => setIncludeExtraShot(e.target.checked)}
                />
                + Extra Shot
              </label>
            </div>
            <p className="mt-1 text-[11px] text-[var(--ink-muted)]">
              Large Oat Latte {includeExtraShot ? "+ Extra Shot" : "(Base)"} explosion:
            </p>
            <ul className="mt-2 space-y-1 font-mono text-[11px]">
              <li>• House Espresso Beans: {comboBeans} g</li>
              <li>• Barista Oat Milk: 250 ml</li>
              <li>• 16oz Cup + Sip Lid: 1 pcs each</li>
            </ul>
            <div className="mt-3 flex items-center justify-between border-t border-[var(--surface-border)] pt-2 font-mono text-xs font-bold">
              <span>COGS: PKR {comboCogs.toFixed(2)}</span>
              <span className="text-emerald-600 dark:text-emerald-400">
                Margin: {comboMargin.toFixed(1)}%
              </span>
            </div>
          </div>
        </section>

        {/* Right Interactive Recipe Editor */}
        <section className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5 lg:col-span-8">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--surface-border)] pb-4">
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-base font-bold">{selected.name}</h2>
                <span className="inline-flex items-center gap-1 rounded-full bg-[var(--surface-subtle)] px-2 py-0.5 font-mono text-[11px] font-bold">
                  <GitBranch className="h-3 w-3" />v{selected.version}
                </span>
              </div>
              <p className="mt-0.5 text-xs text-[var(--ink-muted)]">
                Immutable versioned snapshot · Editing publishes v{selected.version + 1} without altering historical COGS.
              </p>
            </div>
            <button
              type="button"
              onClick={publishNewVersion}
              className="rounded-lg bg-terracotta px-4 py-2 text-xs font-bold text-white hover:bg-terracotta-700"
            >
              Publish v{selected.version + 1}
            </button>
          </div>

          <div className="mt-4 grid grid-cols-3 gap-3">
            <div className="rounded-lg border border-[var(--surface-border)] p-3">
              <p className="text-[11px] text-[var(--ink-muted)]">Theoretical COGS</p>
              <p className="mt-1 font-mono text-lg font-bold">PKR {cogs.toFixed(2)}</p>
            </div>
            <div className="rounded-lg border border-[var(--surface-border)] p-3">
              <p className="text-[11px] text-[var(--ink-muted)]">Food Cost %</p>
              <span className={`mt-1 inline-block rounded px-2 py-0.5 font-mono text-base font-bold ${foodCostBadgeCls}`}>
                {foodCostPct.toFixed(1)}%
              </span>
            </div>
            <div className="rounded-lg border border-[var(--surface-border)] p-3">
              <p className="text-[11px] text-[var(--ink-muted)]">Gross Margin %</p>
              <p className="mt-1 font-mono text-lg font-bold text-emerald-600 dark:text-emerald-400">
                {grossMarginPct.toFixed(1)}%
              </p>
            </div>
          </div>

          <div className="mt-5 space-y-2">
            {selected.lines.map((line) => {
              const lineCost = line.qty * (1 + line.wastePct / 100) * line.unitCostPkr;
              return (
                <div
                  key={line.id}
                  className="grid grid-cols-12 items-center gap-2 rounded-lg border border-[var(--surface-border)] px-3 py-2 text-xs"
                >
                  <span className="col-span-5 font-semibold">{line.ingredientName}</span>
                  <div className="col-span-3 flex items-center gap-1.5">
                    <input
                      type="number"
                      min={0}
                      step="1"
                      value={line.qty}
                      onChange={(e) => updateLineQty(line.id, Number(e.target.value))}
                      className="w-20 rounded border border-[var(--surface-border)] bg-transparent px-2 py-1 font-mono text-xs"
                    />
                    <span className="font-mono text-[var(--ink-muted)]">{line.baseUnit}</span>
                  </div>
                  <span className="col-span-2 font-mono text-[11px] text-[var(--ink-muted)]">
                    +{line.wastePct}% waste
                  </span>
                  <span className="col-span-2 text-right font-mono font-bold">
                    PKR {lineCost.toFixed(2)}
                  </span>
                </div>
              );
            })}
          </div>
        </section>
      </div>
    </AppShell>
  );
}
