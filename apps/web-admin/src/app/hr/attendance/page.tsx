"use client";

import React, { useState } from "react";
import { AlertTriangle, CheckCircle2, Clock, Download, LayoutDashboard, Users } from "lucide-react";
import {
  AppShell,
  DataTable,
  KpiMetricCard,
  SyncStatusBadge,
  type AppShellNavItem,
  type DataTableColumn,
} from "@hospitality/ui";

interface AttendanceRow {
  id: string;
  empCode: string;
  staffName: string;
  role: string;
  branch: string;
  regularHrs: string;
  otHrs: string;
  exception: string;
  status: "pending_review" | "approved";
}

const PILOT_ATTENDANCE: AttendanceRow[] = [
  {
    id: "att-1",
    empCode: "EMP-01",
    staffName: "Ayesha Khan",
    role: "Cashier / Barista",
    branch: "F-7 Coffee Bar",
    regularHrs: "8.00",
    otHrs: "1.50",
    exception: "Late (+18m) · OT (+1.50h)",
    status: "approved",
  },
  {
    id: "att-2",
    empCode: "EMP-02",
    staffName: "Hamza Raza",
    role: "Floor Waiter",
    branch: "G-9 Flagship Dine-In",
    regularHrs: "0.00",
    otHrs: "0.00",
    exception: "Missing Clock-Out",
    status: "pending_review",
  },
];

export default function HrAttendancePage() {
  const [rows, setRows] = useState<AttendanceRow[]>(PILOT_ATTENDANCE);
  const [tab, setTab] = useState<"attendance" | "roster" | "export">("attendance");

  const openExceptions = rows.filter((r) => r.status === "pending_review").length;

  function resolveAndApprove(id: string) {
    setRows((prev) =>
      prev.map((r) =>
        r.id === id
          ? { ...r, regularHrs: "8.00", exception: "Manager Adjusted (Audited)", status: "approved" }
          : r
      )
    );
  }

  function downloadPayrollCsv() {
    const approved = rows.filter((r) => r.status === "approved");
    const lines = [
      "employee_code,staff_name,role,branch,regular_hours,overtime_hours",
      ...approved.map((r) => `${r.empCode},${r.staffName},${r.role},${r.branch},${r.regularHrs},${r.otHrs}`),
    ];
    const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "approved-payroll-hours.csv";
    a.click();
    URL.revokeObjectURL(url);
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
      id: "hr",
      label: "HR · Rosters & Attendance",
      icon: <Users className="h-4 w-4" />,
      active: true,
    },
  ];

  const columns: DataTableColumn<AttendanceRow>[] = [
    { key: "empCode", header: "Code", render: (r) => <span className="font-mono font-bold">{r.empCode}</span> },
    { key: "staffName", header: "Staff Member", sortable: true },
    { key: "role", header: "Role" },
    { key: "branch", header: "Branch" },
    { key: "regularHrs", header: "Reg Hrs", align: "right" },
    { key: "otHrs", header: "OT Hrs", align: "right" },
    {
      key: "exception",
      header: "Exceptions",
      render: (r) => (
        <span className="inline-flex items-center gap-1 rounded bg-amber-500/15 px-2 py-0.5 text-[11px] font-semibold text-amber-700 dark:text-amber-300">
          <Clock className="h-3 w-3" /> {r.exception}
        </span>
      ),
    },
    {
      key: "id",
      header: "Action",
      align: "right",
      render: (r) =>
        r.status === "pending_review" ? (
          <button
            type="button"
            onClick={() => resolveAndApprove(r.id)}
            className="rounded bg-terracotta px-2.5 py-1 text-xs font-semibold text-white hover:bg-terracotta-700"
          >
            Resolve &amp; Approve
          </button>
        ) : (
          <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="h-3.5 w-3.5" /> Approved
          </span>
        ),
    },
  ];

  return (
    <AppShell
      brandName="Hospitality OS"
      brandSubtitle="Workforce & Attendance"
      navItems={navItems}
      topBarLeft={
        <h1 className="text-sm font-bold text-[var(--ink-primary)]">
          Staff Rosters, PIN Attendance Exceptions &amp; Payroll Export
        </h1>
      }
      topBarRight={<SyncStatusBadge status="online" lastSyncedLabel="Offline PIN Sync" />}
    >
      <div className="mx-auto max-w-7xl space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiMetricCard title="Clocked-In Staff" value="8" deltaPercent={0} comparisonLabel="across 2 branches" freshnessLabel="live" />
          <KpiMetricCard title="Open Exceptions" value={String(openExceptions)} deltaPercent={0} comparisonLabel="must resolve for payroll" freshnessLabel="live" />
          <KpiMetricCard title="Approved Hours (Period)" value="17.50 h" deltaPercent={4.0} comparisonLabel="16.00 Reg + 1.50 OT" freshnessLabel="today" />
          <KpiMetricCard
            title="Payroll Export Status"
            value={openExceptions === 0 ? "Ready ✓" : "Pending"}
            deltaPercent={0}
            comparisonLabel={openExceptions === 0 ? "all logs approved" : `${openExceptions} unapproved`}
            freshnessLabel="live"
          />
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex gap-2">
            {(
              [
                ["attendance", "Attendance & Exceptions"],
                ["roster", "Weekly Staff Roster"],
                ["export", "Approved-Hours Export"],
              ] as const
            ).map(([id, label]) => (
              <button
                key={id}
                type="button"
                onClick={() => setTab(id)}
                className={`rounded-lg border px-3.5 py-2 text-xs font-semibold ${
                  tab === id
                    ? "border-terracotta bg-terracotta text-white"
                    : "border-[var(--surface-border)] bg-[var(--surface-card)]"
                }`}
              >
                {label}
              </button>
            ))}
          </div>

          <button
            type="button"
            onClick={downloadPayrollCsv}
            className="inline-flex items-center gap-1.5 rounded-lg bg-terracotta px-3.5 py-2 text-xs font-bold text-white hover:bg-terracotta-700"
          >
            <Download className="h-3.5 w-3.5" />
            Download Payroll CSV
          </button>
        </div>

        {openExceptions > 0 && (
          <div className="flex items-center gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-2.5 text-xs font-semibold text-amber-700 dark:text-amber-300">
            <AlertTriangle className="h-4 w-4" />
            {openExceptions} unapproved attendance exception(s) will be excluded from CSV payroll export until resolved.
          </div>
        )}

        <DataTable columns={columns} data={rows} caption="Staff attendance and approved hours" />
      </div>
    </AppShell>
  );
}
