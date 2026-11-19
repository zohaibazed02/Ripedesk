"use client";

import React, { useState } from "react";
import Link from "next/link";

const STAGES = [
  "Order Received",
  "Branch Accepted",
  "Preparing in Kitchen",
  "Out for Delivery",
  "Completed",
];

export default function OrderTrackingPage() {
  const [stageIdx, setStageIdx] = useState(3);
  const [progressPct, setProgressPct] = useState(55);
  const [expired, setExpired] = useState(false);

  const riderX = 40 + (progressPct / 100) * 320;
  const riderY = 120 - Math.sin((progressPct / 100) * Math.PI) * 55;
  const etaMins = Math.max(2, Math.round(((100 - progressPct) / 100) * 14));

  if (expired) {
    return (
      <main className="mx-auto flex min-h-screen max-w-xl flex-col items-center justify-center gap-4 px-5 text-center">
        <div className="rounded-2xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-8 shadow-card">
          <p className="font-mono text-xs font-bold uppercase text-rose-500">
            HTTP 410 · Privacy Link Expired
          </p>
          <h1 className="mt-2 text-xl font-extrabold">
            This tracking link has expired for privacy
          </h1>
          <p className="mt-2 text-xs text-[var(--ink-muted)]">
            Live rider coordinates are automatically purged and tracking tokens expire after delivery completion.
          </p>
          <button
            type="button"
            onClick={() => setExpired(false)}
            className="mt-5 rounded-lg bg-terracotta px-4 py-2 text-xs font-bold text-white"
          >
            Return to Demo Live View
          </button>
        </div>
      </main>
    );
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-2xl flex-col gap-5 px-5 py-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="font-mono text-xs uppercase tracking-wider text-terracotta">
            Live Order &amp; Rider Telemetry
          </p>
          <h1 className="mt-1 text-2xl font-extrabold tracking-tight">
            Order #ORD-4082 · ETA ~{etaMins} mins
          </h1>
          <p className="text-xs text-[var(--ink-muted)]">
            G-9 Flagship Dine-In → F-10 Markaz, Islamabad
          </p>
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => setProgressPct((p) => (p >= 95 ? 15 : p + 20))}
            className="rounded-lg border border-[var(--surface-border)] px-3 py-1.5 text-xs font-semibold hover:border-terracotta"
          >
            Move Rider →
          </button>
          <button
            type="button"
            onClick={() => setExpired(true)}
            className="rounded-lg border border-rose-500/40 px-3 py-1.5 text-xs font-semibold text-rose-600 dark:text-rose-400"
          >
            Test 410 Expired
          </button>
        </div>
      </div>

      {/* Interactive Vector Route & Rider Map */}
      <section className="overflow-hidden rounded-2xl border border-[var(--surface-border)] bg-[#0F172A] p-4 text-white shadow-card">
        <div className="mb-3 flex items-center justify-between text-xs">
          <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-500/20 px-2.5 py-0.5 font-mono text-[11px] font-bold text-emerald-300">
            <span className="h-2 w-2 rounded-full bg-emerald-400 animate-pulse" />
            Last updated 8s ago • Live GPS
          </span>
          <span className="font-mono text-[11px] text-slate-300">
            Active Trip Privacy Enforced
          </span>
        </div>

        <svg viewBox="0 0 400 160" className="h-44 w-full rounded-xl bg-slate-900/80">
          {/* Route Curve */}
          <path
            d="M 40 120 Q 200 10 360 120"
            fill="none"
            stroke="#EA580C"
            strokeWidth="3"
            strokeDasharray="6 6"
          />
          {/* Branch Pin */}
          <circle cx="40" cy="120" r="7" fill="#10B981" />
          <text x="20" y="145" fill="#94A3B8" fontSize="10" fontFamily="monospace">
            G-9 Branch
          </text>
          {/* Customer Drop-Off Pin */}
          <circle cx="360" cy="120" r="7" fill="#38BDF8" />
          <text x="315" y="145" fill="#94A3B8" fontSize="10" fontFamily="monospace">
            F-10 Drop-Off
          </text>
          {/* Animated Rider Scooter Pin */}
          <circle cx={riderX} cy={riderY} r="11" fill="#EA580C" opacity="0.3" />
          <circle cx={riderX} cy={riderY} r="6" fill="#EA580C" stroke="#FFFFFF" strokeWidth="2" />
        </svg>
      </section>

      {/* 5-Step Progress Tracker */}
      <section className="rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5 shadow-card">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-xs font-bold uppercase tracking-wider text-[var(--ink-muted)]">
            Order Milestones
          </h2>
          <button
            type="button"
            onClick={() => setStageIdx((i) => (i + 1) % STAGES.length)}
            className="text-xs font-semibold text-terracotta hover:underline"
          >
            Advance Stage
          </button>
        </div>
        <div className="space-y-3">
          {STAGES.map((label, idx) => {
            const done = idx <= stageIdx;
            const current = idx === stageIdx;
            return (
              <div key={label} className="flex items-center gap-3">
                <span
                  className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full font-mono text-[11px] font-bold ${
                    done
                      ? "bg-emerald-500 text-white"
                      : "border border-[var(--surface-border)] text-[var(--ink-muted)]"
                  }`}
                >
                  {idx + 1}
                </span>
                <span className={`text-xs font-bold ${current ? "text-terracotta" : ""}`}>
                  {label}
                </span>
              </div>
            );
          })}
        </div>
      </section>

      <Link href="/" className="text-center text-xs font-semibold text-terracotta hover:underline">
        ← Back to Storefront
      </Link>
    </main>
  );
}
