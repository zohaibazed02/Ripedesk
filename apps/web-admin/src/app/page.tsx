"use client";

import React, { useEffect, useState } from "react";
import {
  Activity,
  BookOpen,
  Boxes,
  CheckCircle2,
  Cpu,
  Database,
  HardDrive,
  LayoutDashboard,
  Moon,
  Radio,
  Receipt,
  Server,
  ShieldCheck,
  Sun,
  Users,
} from "lucide-react";
import {
  AppShell,
  BranchSelector,
  DataTable,
  KpiMetricCard,
  OrderModeBadge,
  SyncStatusBadge,
  type AppShellNavItem,
  type BranchOption,
  type CommandPaletteAction,
  type DataTableColumn,
  type OrderMode,
  type SyncStatus,
} from "@hospitality/ui";

interface LedgerEntry {
  id: string;
  orderNumber: string;
  branchCode: string;
  mode: OrderMode;
  itemsCount: number;
  grossAmount: string;
  syncState: SyncStatus;
  timestamp: string;
}

const BRANCHES: BranchOption[] = [
  {
    id: "br-downtown-01",
    organizationName: "Roast & Ember Hospitality Group",
    branchName: "Downtown Flagship Roastery",
    code: "RE-NYC-01",
    city: "New York, NY",
    syncStatus: "online",
    pendingSyncCount: 0,
    lastHeartbeat: "2s ago",
  },
  {
    id: "br-soho-02",
    organizationName: "Roast & Ember Hospitality Group",
    branchName: "SoHo Express Coffee Bar",
    code: "RE-NYC-02",
    city: "New York, NY",
    syncStatus: "lan",
    pendingSyncCount: 14,
    lastHeartbeat: "LAN active · 4s ago",
  },
  {
    id: "br-airport-03",
    organizationName: "Roast & Ember Hospitality Group",
    branchName: "JFK Terminal 4 Kiosk",
    code: "RE-JFK-03",
    city: "Queens, NY",
    syncStatus: "isolated",
    pendingSyncCount: 37,
    lastHeartbeat: "Offline 18m",
  },
];

const ALL_ORDER_MODES: OrderMode[] = [
  "QUICK_SALE",
  "TAKEAWAY",
  "DINE_IN",
  "DELIVERY",
  "QR_ORDER",
  "DRIVE_THRU",
];

const LEDGER_DATA: LedgerEntry[] = [
  {
    id: "ord-9041",
    orderNumber: "#ORD-9041",
    branchCode: "RE-NYC-01",
    mode: "QUICK_SALE",
    itemsCount: 2,
    grossAmount: "$14.50",
    syncState: "online",
    timestamp: "21:52:14",
  },
  {
    id: "ord-9042",
    orderNumber: "#ORD-9042",
    branchCode: "RE-NYC-01",
    mode: "DINE_IN",
    itemsCount: 6,
    grossAmount: "$84.20",
    syncState: "online",
    timestamp: "21:50:48",
  },
  {
    id: "ord-9043",
    orderNumber: "#ORD-9043",
    branchCode: "RE-NYC-02",
    mode: "TAKEAWAY",
    itemsCount: 3,
    grossAmount: "$22.75",
    syncState: "lan",
    timestamp: "21:49:11",
  },
  {
    id: "ord-9044",
    orderNumber: "#ORD-9044",
    branchCode: "RE-NYC-01",
    mode: "DELIVERY",
    itemsCount: 5,
    grossAmount: "$61.40",
    syncState: "online",
    timestamp: "21:47:30",
  },
  {
    id: "ord-9045",
    orderNumber: "#ORD-9045",
    branchCode: "RE-NYC-02",
    mode: "QR_ORDER",
    itemsCount: 4,
    grossAmount: "$38.90",
    syncState: "lan",
    timestamp: "21:45:03",
  },
  {
    id: "ord-9046",
    orderNumber: "#ORD-9046",
    branchCode: "RE-JFK-03",
    mode: "DRIVE_THRU",
    itemsCount: 2,
    grossAmount: "$18.60",
    syncState: "isolated",
    timestamp: "21:41:22",
  },
];

const SYSTEM_NODES = [
  {
    name: "Cloud API (Go Modular Monolith)",
    endpoint: "services/cloud-api · :8080",
    status: "Operational",
    latency: "14ms",
    icon: Server,
  },
  {
    name: "Branch LAN Server (Go + SQLite)",
    endpoint: "services/branch-service · :8090",
    status: "LAN Active",
    latency: "2ms",
    icon: Radio,
  },
  {
    name: "PostgreSQL 16 (Multi-Tenant Ledger)",
    endpoint: "infra/docker-compose · :5432",
    status: "Healthy",
    latency: "4ms",
    icon: Database,
  },
  {
    name: "Redis 7 & MinIO S3 Storage",
    endpoint: "Queue + Vendor Bills · :6379 / :9000",
    status: "Healthy",
    latency: "1ms",
    icon: HardDrive,
  },
];

export default function AdminShowcasePage() {
  const [selectedBranchId, setSelectedBranchId] = useState<string>(
    BRANCHES[0].id
  );
  const [activeNav, setActiveNav] = useState<string>("overview");
  const [isDark, setIsDark] = useState<boolean>(false);
  const [selectedOrder, setSelectedOrder] = useState<LedgerEntry | null>(null);

  const activeBranch =
    BRANCHES.find((b) => b.id === selectedBranchId) ?? BRANCHES[0];

  useEffect(() => {
    const root = document.documentElement;
    if (isDark) {
      root.classList.add("dark");
    } else {
      root.classList.remove("dark");
    }
  }, [isDark]);

  const navItems: AppShellNavItem[] = [
    {
      id: "overview",
      label: "Design System & Health",
      icon: <LayoutDashboard className="h-4 w-4" />,
      active: activeNav === "overview",
      onClick: () => setActiveNav("overview"),
    },
    {
      id: "orders",
      label: "Omnichannel Orders",
      icon: <Receipt className="h-4 w-4" />,
      badge: "6 modes",
      active: activeNav === "orders",
      onClick: () => setActiveNav("orders"),
    },
    {
      id: "inventory",
      label: "Inventory & Recipes",
      icon: <Boxes className="h-4 w-4" />,
      active: activeNav === "inventory",
      onClick: () => {
        window.location.href = "/inventory/ingredients";
      },
    },
    {
      id: "accounting",
      label: "Accounting & ERP",
      icon: <BookOpen className="h-4 w-4" />,
      active: activeNav === "accounting",
      onClick: () => setActiveNav("accounting"),
    },
    {
      id: "hr",
      label: "Staff & Shifts",
      icon: <Users className="h-4 w-4" />,
      active: activeNav === "hr",
      onClick: () => setActiveNav("hr"),
    },
  ];

  const commandActions: CommandPaletteAction[] = [
    {
      id: "toggle-theme",
      label: isDark ? "Switch to Daylight Mode" : "Switch to High-Contrast Dark Mode",
      group: "Appearance",
      shortcut: "T",
      onSelect: () => setIsDark((prev) => !prev),
    },
    ...BRANCHES.map((branch) => ({
      id: `branch-${branch.id}`,
      label: `Switch Branch → ${branch.branchName} (${branch.code})`,
      group: "Multi-Tenant Branches",
      onSelect: () => setSelectedBranchId(branch.id),
    })),
  ];

  const columns: DataTableColumn<LedgerEntry>[] = [
    {
      key: "orderNumber",
      header: "Order #",
      sortable: true,
      render: (row) => (
        <span className="font-mono font-semibold text-[var(--ink-primary)]">
          {row.orderNumber}
        </span>
      ),
    },
    {
      key: "branchCode",
      header: "Branch",
      sortable: true,
      render: (row) => (
        <span className="font-mono text-[11px] text-[var(--ink-muted)]">
          {row.branchCode}
        </span>
      ),
    },
    {
      key: "mode",
      header: "Order Mode",
      sortable: true,
      render: (row) => <OrderModeBadge mode={row.mode} size="sm" />,
    },
    {
      key: "itemsCount",
      header: "Items",
      align: "right",
      sortable: true,
    },
    {
      key: "grossAmount",
      header: "Gross Total",
      align: "right",
      sortable: true,
      render: (row) => (
        <span className="font-mono font-semibold">{row.grossAmount}</span>
      ),
    },
    {
      key: "syncState",
      header: "Sync Status",
      render: (row) => (
        <SyncStatusBadge
          status={row.syncState}
          pendingSyncCount={row.syncState === "online" ? 0 : 1}
          compact
        />
      ),
    },
    {
      key: "timestamp",
      header: "Time",
      align: "right",
      sortable: true,
      render: (row) => (
        <span className="font-mono text-[11px] text-[var(--ink-muted)]">
          {row.timestamp}
        </span>
      ),
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Admin & ERP Control Plane"
      navItems={navItems}
      commandActions={commandActions}
      topBarLeft={
        <BranchSelector
          branches={BRANCHES}
          selectedBranchId={selectedBranchId}
          onSelectBranch={setSelectedBranchId}
        />
      }
      topBarRight={
        <div className="flex items-center gap-2.5">
          <SyncStatusBadge
            status={activeBranch.syncStatus}
            pendingSyncCount={activeBranch.pendingSyncCount}
            lastSyncedLabel={activeBranch.lastHeartbeat}
          />
          <button
            type="button"
            aria-label="Toggle light and dark theme"
            onClick={() => setIsDark((prev) => !prev)}
            className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[var(--surface-border)] bg-[var(--surface-card)] text-[var(--ink-secondary)] shadow-card transition-colors hover:border-terracotta/50 hover:text-[var(--ink-primary)]"
          >
            {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </button>
        </div>
      }
    >
      <div className="mx-auto max-w-7xl space-y-6">
        {/* Hero Header */}
        <section className="flex flex-col justify-between gap-4 rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5 shadow-card sm:flex-row sm:items-center">
          <div>
            <div className="inline-flex items-center gap-1.5 rounded-full bg-terracotta/10 px-2.5 py-0.5 text-[11px] font-semibold text-terracotta">
              <ShieldCheck className="h-3.5 w-3.5" />
              Step 1 · Monorepo Foundation & Design System
            </div>
            <h1 className="mt-2 text-xl font-bold tracking-tight text-[var(--ink-primary)]">
              UI/UX Design System & Architecture Showcase
            </h1>
            <p className="mt-1 text-xs text-[var(--ink-muted)]">
              Offline-first, multi-tenant Restaurant & Coffee Shop POS, ERP, and
              Delivery Ecosystem powered by Next.js 15, Go, Flutter, and OpenAPI
              3.1.
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <button
              type="button"
              onClick={() => setIsDark((prev) => !prev)}
              className="rounded-lg border border-[var(--surface-border)] bg-[var(--surface-subtle)] px-3 py-2 text-xs font-medium text-[var(--ink-primary)] transition-colors hover:border-terracotta/50"
            >
              {isDark ? "Daylight Mode" : "High-Contrast Dark"}
            </button>
            <button
              type="button"
              onClick={() =>
                setSelectedBranchId(
                  BRANCHES[
                    (BRANCHES.findIndex((b) => b.id === selectedBranchId) + 1) %
                      BRANCHES.length
                  ].id
                )
              }
              className="rounded-lg bg-terracotta px-3.5 py-2 text-xs font-semibold text-white shadow-card transition-colors hover:bg-terracotta-700"
            >
              Cycle Branch Sync State
            </button>
          </div>
        </section>

        {/* KPI Metric Cards */}
        <section aria-label="Key Performance Indicators">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <KpiMetricCard
              title="Net Sales (Today)"
              value="$14,820.40"
              deltaPercent={12.4}
              comparisonLabel="vs. last Thursday"
              freshnessLabel="2s ago"
              sparkline={[35, 42, 40, 51, 60, 58, 72, 84]}
            />
            <KpiMetricCard
              title="Active Orders (6 Modes)"
              value="342"
              deltaPercent={8.1}
              comparisonLabel="avg ticket $43.33"
              freshnessLabel="5s ago"
              sparkline={[20, 24, 28, 25, 34, 39, 44, 48]}
            />
            <KpiMetricCard
              title="Avg KDS Prep Time"
              value="4m 18s"
              deltaPercent={-6.2}
              comparisonLabel="faster than 4m 35s SLA"
              freshnessLabel="live LAN"
              sparkline={[62, 58, 55, 51, 49, 46, 44, 41]}
            />
            <KpiMetricCard
              title="COGS / Food Cost"
              value="26.8%"
              deltaPercent={-1.4}
              comparisonLabel="target <= 28.0%"
              freshnessLabel="1m ago"
              sparkline={[30, 29, 29, 28, 28, 27, 27, 26.8]}
            />
          </div>
        </section>

        {/* Sync States & 6 Order Modes Showcase */}
        <section className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <div className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5 shadow-card">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-semibold text-[var(--ink-primary)]">
                Offline-First Sync Indicators
              </h2>
              <span className="font-mono text-[11px] text-[var(--ink-muted)]">
                SyncStatusBadge
              </span>
            </div>
            <p className="mt-1 text-xs text-[var(--ink-muted)]">
              Real-time connectivity & local SQLite delta queue visibility across
              Cloud, Branch LAN, and Isolated terminals.
            </p>
            <div className="mt-4 flex flex-wrap gap-2.5">
              <SyncStatusBadge status="online" lastSyncedLabel="2s ago" />
              <SyncStatusBadge
                status="lan"
                pendingSyncCount={14}
                lastSyncedLabel="LAN 2ms"
              />
              <SyncStatusBadge
                status="isolated"
                pendingSyncCount={37}
                lastSyncedLabel="Offline 18m"
              />
            </div>
          </div>

          <div className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5 shadow-card">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-semibold text-[var(--ink-primary)]">
                Omnichannel Order Modes
              </h2>
              <span className="font-mono text-[11px] text-[var(--ink-muted)]">
                OrderModeBadge
              </span>
            </div>
            <p className="mt-1 text-xs text-[var(--ink-muted)]">
              Color-coded operational badges shared across POS, KDS, Storefront,
              and Accounting ledgers.
            </p>
            <div className="mt-4 flex flex-wrap gap-2">
              {ALL_ORDER_MODES.map((mode) => (
                <OrderModeBadge key={mode} mode={mode} />
              ))}
            </div>
          </div>
        </section>

        {/* High-Density Financial & Order Ledger Table */}
        <section className="space-y-2.5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <h2 className="text-sm font-semibold text-[var(--ink-primary)]">
                Live Omnichannel Order Ledger
              </h2>
              <p className="text-xs text-[var(--ink-muted)]">
                High-density keyboard-navigable DataTable (click or press ↑/↓ +
                Enter).
              </p>
            </div>
            {selectedOrder && (
              <div className="inline-flex items-center gap-2 rounded-lg border border-terracotta/30 bg-terracotta/10 px-3 py-1 text-xs text-[var(--ink-primary)]">
                <Activity className="h-3.5 w-3.5 text-terracotta" />
                <span>
                  Inspecting{" "}
                  <strong className="font-mono">
                    {selectedOrder.orderNumber}
                  </strong>{" "}
                  ({selectedOrder.grossAmount})
                </span>
              </div>
            )}
          </div>
          <DataTable
            columns={columns}
            data={LEDGER_DATA}
            caption="Recent omnichannel orders across tenant branches"
            onRowActivate={(row) => setSelectedOrder(row)}
          />
        </section>

        {/* System Health Status Cards */}
        <section className="space-y-3">
          <div className="flex items-center gap-2">
            <Cpu className="h-4 w-4 text-terracotta" />
            <h2 className="text-sm font-semibold text-[var(--ink-primary)]">
              Platform Infrastructure & Service Health
            </h2>
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
            {SYSTEM_NODES.map((node) => {
              const Icon = node.icon;
              return (
                <div
                  key={node.name}
                  className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-4 shadow-card"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--surface-subtle)] text-[var(--ink-primary)]">
                      <Icon className="h-4 w-4" />
                    </div>
                    <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-300">
                      <CheckCircle2 className="h-3 w-3" />
                      {node.status}
                    </span>
                  </div>
                  <p className="mt-3 text-xs font-semibold text-[var(--ink-primary)]">
                    {node.name}
                  </p>
                  <p className="mt-0.5 font-mono text-[11px] text-[var(--ink-muted)]">
                    {node.endpoint}
                  </p>
                  <div className="mt-3 flex items-center justify-between border-t border-[var(--surface-border)] pt-2 text-[11px] text-[var(--ink-muted)]">
                    <span>Latency</span>
                    <span className="font-mono font-semibold text-[var(--ink-primary)]">
                      {node.latency}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      </div>
    </AppShell>
  );
}
