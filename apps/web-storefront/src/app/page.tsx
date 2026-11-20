"use client";

import React, { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { OrderModeBadge, type OrderMode } from "@hospitality/ui";
import { calcCartTotalsMinor, formatMinor } from "../lib/money";

const CLOUD_API_URL =
  process.env.NEXT_PUBLIC_CLOUD_API_URL || "http://localhost:8080";
const TENANT_SLUG =
  process.env.NEXT_PUBLIC_DEFAULT_TENANT_SLUG || "artisan-roast-bistro";

type StorefrontTheme = "coffee_pickup" | "qsr_delivery" | "dine_in_qr";

interface StoreItem {
  id: string;
  name: string;
  desc: string;
  priceMinor: number;
  theme: StorefrontTheme;
  comboUpgradeMinor?: number;
  tag?: string;
}

interface CartItem {
  id: string;
  name: string;
  unitPriceMinor: number;
  modifiersMinor: number;
  modifierLabel?: string;
  quantity: number;
}

const CATALOG: StoreItem[] = [
  {
    id: "cof-1",
    name: "Large Oat Latte",
    desc: "Double ristretto with silky steamed barista oat milk (16oz)",
    priceMinor: 95000,
    theme: "coffee_pickup",
    tag: "Signature",
  },
  {
    id: "cof-2",
    name: "18h Cold Brew Tonic",
    desc: "Single-origin Ethiopian cold brew over artisanal tonic",
    priceMinor: 78000,
    theme: "coffee_pickup",
    tag: "Iced",
  },
  {
    id: "qsr-1",
    name: "Artisan Smash Burger",
    desc: "Double dry-aged beef patty, caramelized onions, brioche bun",
    priceMinor: 115000,
    theme: "qsr_delivery",
    comboUpgradeMinor: 35000,
    tag: "Combo Ready",
  },
  {
    id: "qsr-2",
    name: "Crispy Hot Honey Chicken",
    desc: "Buttermilk fried chicken thigh, slaw, house pickles",
    priceMinor: 98000,
    theme: "qsr_delivery",
    comboUpgradeMinor: 35000,
    tag: "Popular",
  },
  {
    id: "qr-1",
    name: "Truffle Mushroom Pasta",
    desc: "Hand-rolled tagliatelle with wild porcini & black truffle cream",
    priceMinor: 185000,
    theme: "dine_in_qr",
    tag: "Hot Kitchen",
  },
  {
    id: "qr-2",
    name: "Pugliese Burrata Salad",
    desc: "Heirloom tomatoes, creamy burrata, aged balsamic glaze",
    priceMinor: 140000,
    theme: "dine_in_qr",
    tag: "Small Plate",
  },
];

export default function StorefrontPage() {
  const router = useRouter();
  const [theme, setTheme] = useState<StorefrontTheme>("coffee_pickup");
  const [branchOnline, setBranchOnline] = useState(true);
  const [items, setItems] = useState<StoreItem[]>(CATALOG);
  const [apiOffline, setApiOffline] = useState(false);
  const [checkoutError, setCheckoutError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [cart, setCart] = useState<CartItem[]>([]);
  const [checkoutOpen, setCheckoutOpen] = useState(false);
  const [paymentMethod, setPaymentMethod] = useState<"card" | "cod" | "counter">("card");

  useEffect(() => {
    let active = true;
    async function loadStorefront() {
      try {
        const [cfgRes, menuRes] = await Promise.all([
          fetch(`${CLOUD_API_URL}/api/v1/storefront/${TENANT_SLUG}/config`, { cache: "no-store" }),
          fetch(`${CLOUD_API_URL}/api/v1/storefront/${TENANT_SLUG}/menu`, { cache: "no-store" }),
        ]);
        if (!cfgRes.ok || !menuRes.ok) throw new Error("storefront api error");
        const cfgJson = (await cfgRes.json()) as {
          data?: { branches?: Array<{ is_online?: boolean }> };
        };
        const menuJson = (await menuRes.json()) as {
          data?: {
            items?: Array<{
              id: string;
              name: string;
              description?: string;
              base_price?: string;
            }>;
          };
        };
        if (!active) return;
        const firstBranch = cfgJson.data?.branches?.[0];
        if (firstBranch && typeof firstBranch.is_online === "boolean") {
          setBranchOnline(firstBranch.is_online);
        }
        const liveItems = menuJson.data?.items;
        if (Array.isArray(liveItems) && liveItems.length > 0) {
          setItems(
            liveItems.map((it, idx) => ({
              id: it.id,
              name: it.name,
              desc: it.description || "Freshly prepared to order",
              priceMinor: Math.round(Number(it.base_price || 0) * 100),
              theme: (["coffee_pickup", "qsr_delivery", "dine_in_qr"] as const)[idx % 3],
            }))
          );
        }
        setApiOffline(false);
      } catch {
        if (active) setApiOffline(true);
      }
    }
    void loadStorefront();
    return () => {
      active = false;
    };
  }, []);

  const themeOrderMode: OrderMode =
    theme === "coffee_pickup"
      ? "TAKEAWAY"
      : theme === "qsr_delivery"
        ? "DELIVERY"
        : "QR_ORDER";

  const deliveryFeeMinor = theme === "qsr_delivery" && cart.length > 0 ? 15000 : 0;
  const totals = useMemo(
    () => calcCartTotalsMinor(cart, 1600, deliveryFeeMinor),
    [cart, deliveryFeeMinor]
  );

  function addToCart(item: StoreItem, withAddon = false) {
    const modMinor = withAddon
      ? item.comboUpgradeMinor ?? 22000
      : 0;
    const modLabel = withAddon
      ? item.comboUpgradeMinor
        ? "Combo (+Fries & Drink)"
        : "Extra Shot (+PKR 220)"
      : undefined;

    setCart((prev) => [
      ...prev,
      {
        id: `${item.id}-${Date.now()}`,
        name: item.name,
        unitPriceMinor: item.priceMinor,
        modifiersMinor: modMinor,
        modifierLabel: modLabel,
        quantity: 1,
      },
    ]);
  }

  async function placeOrder() {
    if (!branchOnline || cart.length === 0 || submitting) return;
    setSubmitting(true);
    setCheckoutError(null);
    try {
      const res = await fetch(`${CLOUD_API_URL}/api/v1/storefront/${TENANT_SLUG}/checkout`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          idempotency_key: `web-${Date.now()}`,
          order_mode: themeOrderMode.toLowerCase(),
          payment_method: paymentMethod,
          items: cart.map((c) => ({
            menu_item_id: c.id,
            name: c.name,
            quantity: c.quantity,
            unit_price: ((c.unitPriceMinor + c.modifiersMinor) / 100).toFixed(2),
          })),
        }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = (await res.json()) as { data?: { order_id?: string; order_number?: string } };
      const trackingId = data.data?.order_id || data.data?.order_number || "ORD-4082";
      router.push(`/track/${trackingId}`);
    } catch {
      setApiOffline(true);
      setCheckoutError("Cloud API unreachable — checkout requires live branch heartbeat.");
    } finally {
      setSubmitting(false);
    }
  }

  const visibleItems = items.filter((i) => i.theme === theme);

  return (
    <div className="min-h-screen bg-[var(--surface-canvas)] text-[var(--ink-primary)] pb-24">
      {apiOffline && (
        <div className="bg-amber-500/15 border-b border-amber-500/30 px-4 py-2 text-center text-xs font-medium text-amber-300">
          Offline Fallback Mode: Cloud Storefront API ({CLOUD_API_URL}) is unreachable. Live checkout is paused until reconnected.
        </div>
      )}
      {checkoutError && (
        <div className="bg-rose-500/15 border-b border-rose-500/30 px-4 py-2 text-center text-xs font-medium text-rose-300">
          {checkoutError}
        </div>
      )}
      {/* Top Storefront Header & Theme Switcher */}
      <header className="sticky top-0 z-30 border-b border-[var(--surface-border)] bg-[var(--surface-card)]/95 px-4 py-3 backdrop-blur-md">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-base font-extrabold tracking-tight">
                Artisan Roast &amp; Bistro
              </span>
              <OrderModeBadge mode={themeOrderMode} size="sm" />
            </div>
            <p className="text-xs text-[var(--ink-muted)]">
              {theme === "coffee_pickup"
                ? "F-7 Coffee Bar · Pickup Ready in 8–12 mins"
                : theme === "qsr_delivery"
                  ? "Artisan Quick Kitchen · Delivery in 25–35 mins"
                  : "G-9 Flagship Dine-In · Table T2 • Main Dining Hall"}
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {(
              [
                ["coffee_pickup", "Coffee Pickup"],
                ["qsr_delivery", "QSR Delivery"],
                ["dine_in_qr", "Dine-In QR"],
              ] as const
            ).map(([t, label]) => (
              <button
                key={t}
                type="button"
                onClick={() => setTheme(t)}
                className={`rounded-lg border px-3 py-1.5 text-xs font-semibold transition-colors ${
                  theme === t
                    ? "border-terracotta bg-terracotta text-white"
                    : "border-[var(--surface-border)] bg-[var(--surface-card)]"
                }`}
              >
                {label}
              </button>
            ))}

            <button
              type="button"
              onClick={() => setBranchOnline((v) => !v)}
              className="rounded-lg border border-[var(--surface-border)] px-2.5 py-1.5 font-mono text-[11px]"
            >
              {branchOnline ? "Branch: Online" : "Branch: Offline"}
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl space-y-6 px-4 py-6">
        {/* Offline Heartbeat Guard Banner */}
        {!branchOnline && (
          <div className="rounded-xl border border-rose-500/40 bg-rose-500/10 p-4 text-xs font-semibold text-rose-700 dark:text-rose-300">
            This branch is temporarily offline (heartbeat expired). Immediate online checkout is paused until the branch reconnects.
          </div>
        )}

        {/* Dine-In QR Table Banner */}
        {theme === "dine_in_qr" && (
          <div className="rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-4 text-xs font-semibold text-emerald-800 dark:text-emerald-300">
            Table T2 • Main Dining Hall • Round Ordering Active — Items go straight to Hot Kitchen &amp; Barista KDS.
          </div>
        )}

        {/* Menu Grid */}
        <section className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {visibleItems.map((item) => (
            <article
              key={item.id}
              className="flex flex-col justify-between rounded-xl border border-[var(--surface-border)] bg-[var(--surface-card)] p-5 shadow-card"
            >
              <div>
                <div className="flex items-center justify-between">
                  <h2 className="text-base font-bold">{item.name}</h2>
                  {item.tag && (
                    <span className="rounded-full bg-terracotta/10 px-2.5 py-0.5 text-[11px] font-bold text-terracotta">
                      {item.tag}
                    </span>
                  )}
                </div>
                <p className="mt-1 text-xs text-[var(--ink-muted)]">{item.desc}</p>
              </div>
              <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-[var(--surface-border)] pt-3">
                <span className="font-mono text-sm font-extrabold text-terracotta">
                  {formatMinor(item.priceMinor)}
                </span>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={() => addToCart(item, false)}
                    className="rounded-lg border border-[var(--surface-border)] px-3 py-1.5 text-xs font-semibold hover:border-terracotta"
                  >
                    + Add
                  </button>
                  <button
                    type="button"
                    onClick={() => addToCart(item, true)}
                    className="rounded-lg bg-terracotta px-3 py-1.5 text-xs font-bold text-white hover:bg-terracotta-700"
                  >
                    {item.comboUpgradeMinor ? "+ Make Combo" : "+ Extra Shot"}
                  </button>
                </div>
              </div>
            </article>
          ))}
        </section>
      </main>

      {/* Sticky Bottom Cart Bar */}
      {cart.length > 0 && (
        <div className="fixed inset-x-0 bottom-0 z-40 border-t border-[var(--surface-border)] bg-[var(--surface-card)] px-4 py-3 shadow-elevated">
          <div className="mx-auto flex max-w-5xl items-center justify-between">
            <div>
              <p className="text-xs font-bold">{cart.length} item(s) in cart</p>
              <p className="font-mono text-sm font-extrabold text-terracotta">
                {formatMinor(totals.grandTotalMinor)}
              </p>
            </div>
            <button
              type="button"
              onClick={() => setCheckoutOpen(true)}
              className="rounded-xl bg-terracotta px-5 py-2.5 text-xs font-bold text-white hover:bg-terracotta-700"
            >
              Review &amp; Checkout →
            </button>
          </div>
        </div>
      )}

      {/* Unified Checkout Drawer */}
      {checkoutOpen && (
        <div
          className="fixed inset-0 z-50 flex justify-end bg-black/50 backdrop-blur-xs"
          onClick={() => setCheckoutOpen(false)}
        >
          <aside
            className="flex h-full w-full max-w-md flex-col bg-[var(--surface-card)] p-6 shadow-elevated"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="border-b border-[var(--surface-border)] pb-3 text-base font-bold">
              Checkout · {theme.replace("_", " ").toUpperCase()}
            </h2>
            <div className="mt-4 flex-1 space-y-2.5 overflow-y-auto text-xs">
              {cart.map((c) => (
                <div key={c.id} className="flex justify-between border-b border-[var(--surface-border)] pb-2">
                  <div>
                    <p className="font-semibold">{c.name}</p>
                    {c.modifierLabel && (
                      <p className="text-[11px] text-[var(--ink-muted)]">+ {c.modifierLabel}</p>
                    )}
                  </div>
                  <span className="font-mono font-bold">
                    {formatMinor(c.unitPriceMinor + c.modifiersMinor)}
                  </span>
                </div>
              ))}
            </div>

            <div className="space-y-3 border-t border-[var(--surface-border)] pt-4 text-xs">
              <div className="flex gap-2">
                {(
                  [
                    ["card", "Card / Online"],
                    ["cod", "Cash on Delivery"],
                    ["counter", "Pay at Table/Counter"],
                  ] as const
                ).map(([m, label]) => (
                  <button
                    key={m}
                    type="button"
                    onClick={() => setPaymentMethod(m)}
                    className={`flex-1 rounded-lg border py-2 text-[11px] font-semibold ${
                      paymentMethod === m
                        ? "border-terracotta bg-terracotta/10 text-terracotta"
                        : "border-[var(--surface-border)]"
                    }`}
                  >
                    {label}
                  </button>
                ))}
              </div>
              <div className="space-y-1 font-mono">
                <div className="flex justify-between">
                  <span>Subtotal</span>
                  <span>{formatMinor(totals.subtotalMinor)}</span>
                </div>
                <div className="flex justify-between">
                  <span>Tax (16%)</span>
                  <span>{formatMinor(totals.taxMinor)}</span>
                </div>
                {totals.deliveryFeeMinor > 0 && (
                  <div className="flex justify-between">
                    <span>Delivery Fee</span>
                    <span>{formatMinor(totals.deliveryFeeMinor)}</span>
                  </div>
                )}
                <div className="flex justify-between pt-1 text-sm font-extrabold text-terracotta">
                  <span>Grand Total</span>
                  <span>{formatMinor(totals.grandTotalMinor)}</span>
                </div>
              </div>
              <button
                type="button"
                disabled={!branchOnline}
                onClick={placeOrder}
                className="w-full rounded-xl bg-terracotta py-3 text-xs font-extrabold text-white hover:bg-terracotta-700 disabled:opacity-40"
              >
                {branchOnline ? "Place Order" : "Branch Offline — Checkout Paused"}
              </button>
            </div>
          </aside>
        </div>
      )}
    </div>
  );
}
