"use client";

import React, { useMemo, useState } from "react";
import Link from "next/link";
import { Boxes, ChefHat, LayoutDashboard, Plus, Search, X } from "lucide-react";
import {
  AppShell,
  DataTable,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
  type DataTableColumn,
} from "@hospitality/ui";

interface IngredientRow {
  id: string;
  sku: string;
  name: string;
  baseUnit: "g" | "ml" | "pcs";
  purchasingUnit: string;
  packSummary: string;
  valuationCostPkr: string;
  lowStockThreshold: string;
  trackLotExpiry: boolean;
  isLowStock: boolean;
}

const PILOT_INGREDIENTS: IngredientRow[] = [
  {
    id: "ing-beans",
    sku: "ING-BEANS-ESP",
    name: "House Espresso Beans",
    baseUnit: "g",
    purchasingUnit: "kg",
    packSummary: "1 Bag = 1 kg = 1,000 g",
    valuationCostPkr: "4.5000",
    lowStockThreshold: "2,000 g",
    trackLotExpiry: true,
    isLowStock: false,
  },
  {
    id: "ing-oat",
    sku: "ING-MILK-OAT",
    name: "Barista Oat Milk",
    baseUnit: "ml",
    purchasingUnit: "case",
    packSummary: "1 Case = 12 L = 12,000 ml",
    valuationCostPkr: "0.8500",
    lowStockThreshold: "5,000 ml",
    trackLotExpiry: true,
    isLowStock: true,
  },
  {
    id: "ing-cup",
    sku: "ING-CUP-16OZ",
    name: "16oz Double-Wall Paper Cup",
    baseUnit: "pcs",
    purchasingUnit: "box",
    packSummary: "1 Box = 10 Sleeves = 500 pcs",
    valuationCostPkr: "22.0000",
    lowStockThreshold: "100 pcs",
    trackLotExpiry: false,
    isLowStock: false,
  },
  {
    id: "ing-lid",
    sku: "ING-LID-16OZ",
    name: "16oz Sip-Through Lid",
    baseUnit: "pcs",
    purchasingUnit: "box",
    packSummary: "1 Box = 500 pcs",
    valuationCostPkr: "8.5000",
    lowStockThreshold: "100 pcs",
    trackLotExpiry: false,
    isLowStock: false,
  },
  {
    id: "ing-burrata",
    sku: "ING-CHE-BUR",
    name: "Pugliese Burrata Ball",
    baseUnit: "pcs",
    purchasingUnit: "pack",
    packSummary: "1 Pack = 6 pcs",
    valuationCostPkr: "390.0000",
    lowStockThreshold: "12 pcs",
    trackLotExpiry: true,
    isLowStock: true,
  },
];

export default function IngredientsPage() {
  const [ingredients, setIngredients] = useState<IngredientRow[]>(PILOT_INGREDIENTS);
  const [query, setQuery] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [newSku, setNewSku] = useState("");
  const [newName, setNewName] = useState("");
  const [newBaseUnit, setNewBaseUnit] = useState<"g" | "ml" | "pcs">("g");
  const [newPack, setNewPack] = useState("1 Pack = 1,000 g");
  const [newCost, setNewCost] = useState("1.5000");

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return ingredients;
    return ingredients.filter(
      (i) => i.name.toLowerCase().includes(q) || i.sku.toLowerCase().includes(q)
    );
  }, [ingredients, query]);

  const lowStockCount = ingredients.filter((i) => i.isLowStock).length;
  const lotTrackedCount = ingredients.filter((i) => i.trackLotExpiry).length;

  function handleSaveIngredient(e: React.FormEvent) {
    e.preventDefault();
    if (!newSku.trim() || !newName.trim()) return;
    setIngredients((prev) => [
      {
        id: `ing-${Date.now()}`,
        sku: newSku.trim().toUpperCase(),
        name: newName.trim(),
        baseUnit: newBaseUnit,
        purchasingUnit: "pack",
        packSummary: newPack.trim(),
        valuationCostPkr: newCost.trim(),
        lowStockThreshold: `500 ${newBaseUnit}`,
        trackLotExpiry: true,
        isLowStock: false,
      },
      ...prev,
    ]);
    setNewSku("");
    setNewName("");
    setDrawerOpen(false);
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
      active: true,
    },
    {
      id: "recipes",
      label: "Inventory · Recipes & COGS",
      icon: <ChefHat className="h-4 w-4" />,
      onClick: () => {
        window.location.href = "/inventory/recipes";
      },
    },
  ];

  const columns: DataTableColumn<IngredientRow>[] = [
    {
      key: "sku",
      header: "SKU",
      sortable: true,
      render: (r) => <span className="font-mono text-xs font-semibold">{r.sku}</span>,
    },
    {
      key: "name",
      header: "Ingredient",
      sortable: true,
      render: (r) => (
        <div className="flex items-center gap-2">
          <span className="font-semibold">{r.name}</span>
          {r.isLowStock && (
            <span className="rounded bg-rose-500/15 px-1.5 py-0.5 text-[10px] font-bold text-rose-600 dark:text-rose-400">
              LOW STOCK
            </span>
          )}
        </div>
      ),
    },
    {
      key: "baseUnit",
      header: "Base Unit",
      render: (r) => (
        <span className="rounded bg-[var(--surface-subtle)] px-2 py-0.5 font-mono text-[11px] font-bold">
          {r.baseUnit}
        </span>
      ),
    },
    {
      key: "packSummary",
      header: "Pack Conversions",
      render: (r) => (
        <span className="rounded-md border border-[var(--surface-border)] bg-[var(--surface-subtle)] px-2 py-0.5 font-mono text-[11px]">
          {r.packSummary}
        </span>
      ),
    },
    {
      key: "valuationCostPkr",
      header: "Cost / Base Unit (PKR)",
      align: "right",
      sortable: true,
      render: (r) => (
        <span className="font-mono font-semibold">
          PKR {r.valuationCostPkr}/{r.baseUnit}
        </span>
      ),
    },
    {
      key: "trackLotExpiry",
      header: "Lot / Expiry",
      align: "center",
      render: (r) =>
        r.trackLotExpiry ? (
          <span className="text-xs font-semibold text-emerald-600 dark:text-emerald-400">
            Tracked
          </span>
        ) : (
          <span className="text-xs text-[var(--ink-muted)]">—</span>
        ),
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Inventory & ERP"
      navItems={navItems}
      topBarLeft={
        <div className="flex items-center gap-3">
          <h1 className="text-sm font-bold text-[var(--ink-primary)]">
            Ingredient Master &amp; Pack Conversions
          </h1>
          <Link
            href="/inventory/recipes"
            className="rounded-md border border-[var(--surface-border)] px-2.5 py-1 text-xs font-medium text-[var(--ink-secondary)] hover:border-terracotta"
          >
            Open Recipe Builder →
          </Link>
        </div>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="PostgreSQL RLS" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard
            title="Active Ingredients"
            value={String(ingredients.length)}
            deltaPercent={4.2}
            comparisonLabel="Artisan Roast & Bistro"
            freshnessLabel="just now"
          />
          <KpiMetricCard
            title="Low-Stock Alerts"
            value={String(lowStockCount)}
            deltaPercent={-12.0}
            comparisonLabel="below threshold"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Lot / Expiry Tracked"
            value={String(lotTrackedCount)}
            deltaPercent={0}
            comparisonLabel="FEFO enabled"
            freshnessLabel="live"
          />
          <KpiMetricCard
            title="Avg Valuation Freshness"
            value="14m"
            deltaPercent={2.1}
            comparisonLabel="weighted avg cost"
            freshnessLabel="synced"
          />
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="relative w-full max-w-sm">
            <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-[var(--ink-muted)]" />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search SKU or ingredient name..."
              className="w-full rounded-lg border border-[var(--surface-border)] bg-[var(--surface-card)] py-2 pl-9 pr-3 text-xs focus:border-terracotta focus:outline-none"
            />
          </div>
          <button
            type="button"
            onClick={() => setDrawerOpen(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-terracotta px-3.5 py-2 text-xs font-semibold text-white hover:bg-terracotta-700"
          >
            <Plus className="h-4 w-4" />
            Add Ingredient &amp; Pack Conversion
          </button>
        </div>

        <DataTable
          columns={columns}
          data={filtered}
          caption="Tenant ingredient master with base units and pack conversions"
        />
      </div>

      {drawerOpen && (
        <div
          className="fixed inset-0 z-50 flex justify-end bg-black/50 backdrop-blur-xs"
          onClick={() => setDrawerOpen(false)}
        >
          <aside
            className="flex h-full w-full max-w-md flex-col border-l border-[var(--surface-border)] bg-[var(--surface-card)] p-6 shadow-elevated"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-[var(--surface-border)] pb-3">
              <h2 className="text-sm font-bold">Add Ingredient &amp; Pack Conversion</h2>
              <button type="button" onClick={() => setDrawerOpen(false)}>
                <X className="h-4 w-4" />
              </button>
            </div>
            <form onSubmit={handleSaveIngredient} className="mt-4 space-y-4">
              <div>
                <label className="block text-xs font-semibold">SKU</label>
                <input
                  required
                  value={newSku}
                  onChange={(e) => setNewSku(e.target.value)}
                  placeholder="ING-SYRUP-VAN"
                  className="mt-1 w-full rounded-lg border border-[var(--surface-border)] bg-transparent px-3 py-2 font-mono text-xs"
                />
              </div>
              <div>
                <label className="block text-xs font-semibold">Ingredient Name</label>
                <input
                  required
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="Madagascar Vanilla Syrup"
                  className="mt-1 w-full rounded-lg border border-[var(--surface-border)] bg-transparent px-3 py-2 text-xs"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold">Base Unit</label>
                  <select
                    value={newBaseUnit}
                    onChange={(e) => setNewBaseUnit(e.target.value as "g" | "ml" | "pcs")}
                    className="mt-1 w-full rounded-lg border border-[var(--surface-border)] bg-[var(--surface-card)] px-3 py-2 text-xs"
                  >
                    <option value="g">g (grams)</option>
                    <option value="ml">ml (milliliters)</option>
                    <option value="pcs">pcs (pieces)</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold">Cost / Base Unit (PKR)</label>
                  <input
                    required
                    value={newCost}
                    onChange={(e) => setNewCost(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-[var(--surface-border)] bg-transparent px-3 py-2 font-mono text-xs"
                  />
                </div>
              </div>
              <div>
                <label className="block text-xs font-semibold">Pack Conversion Chain</label>
                <input
                  value={newPack}
                  onChange={(e) => setNewPack(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-[var(--surface-border)] bg-transparent px-3 py-2 font-mono text-xs"
                />
              </div>
              <button
                type="submit"
                className="w-full rounded-lg bg-terracotta py-2.5 text-xs font-bold text-white hover:bg-terracotta-700"
              >
                Save Ingredient
              </button>
            </form>
          </aside>
        </div>
      )}
    </AppShell>
  );
}
