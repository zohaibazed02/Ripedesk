"use client";

import React from "react";
import {
  Zap,
  ShoppingBag,
  UtensilsCrossed,
  Bike,
  QrCode,
  Car,
} from "lucide-react";
import { cn } from "../lib/cn";

export type OrderMode =
  | "QUICK_SALE"
  | "TAKEAWAY"
  | "DINE_IN"
  | "DELIVERY"
  | "QR_ORDER"
  | "DRIVE_THRU";

export interface OrderModeBadgeProps {
  mode: OrderMode;
  size?: "sm" | "md";
  className?: string;
}

const ORDER_MODE_META: Record<
  OrderMode,
  {
    label: string;
    badgeClass: string;
    icon: React.ComponentType<{ className?: string }>;
  }
> = {
  QUICK_SALE: {
    label: "Quick Sale",
    badgeClass:
      "border-orange-500/25 bg-orange-500/10 text-orange-700 dark:text-orange-300",
    icon: Zap,
  },
  TAKEAWAY: {
    label: "Takeaway",
    badgeClass:
      "border-amber-500/25 bg-amber-500/10 text-amber-800 dark:text-amber-300",
    icon: ShoppingBag,
  },
  DINE_IN: {
    label: "Dine-In",
    badgeClass:
      "border-emerald-500/25 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
    icon: UtensilsCrossed,
  },
  DELIVERY: {
    label: "Delivery",
    badgeClass:
      "border-sky-500/25 bg-sky-500/10 text-sky-700 dark:text-sky-300",
    icon: Bike,
  },
  QR_ORDER: {
    label: "QR Order",
    badgeClass:
      "border-violet-500/25 bg-violet-500/10 text-violet-700 dark:text-violet-300",
    icon: QrCode,
  },
  DRIVE_THRU: {
    label: "Drive-Thru",
    badgeClass:
      "border-rose-500/25 bg-rose-500/10 text-rose-700 dark:text-rose-300",
    icon: Car,
  },
};

export function OrderModeBadge({
  mode,
  size = "md",
  className,
}: OrderModeBadgeProps) {
  const meta = ORDER_MODE_META[mode];
  const Icon = meta.icon;

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-md border font-medium whitespace-nowrap",
        size === "sm" ? "px-2 py-0.5 text-[11px]" : "px-2.5 py-1 text-xs",
        meta.badgeClass,
        className
      )}
    >
      <Icon className={size === "sm" ? "h-3 w-3" : "h-3.5 w-3.5"} />
      <span>{meta.label}</span>
    </span>
  );
}
