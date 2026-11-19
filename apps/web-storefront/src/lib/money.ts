// Exact integer minor-unit (paisa / cents) currency math — zero floating-point drift.

export interface CartLinePricing {
  unitPriceMinor: number;
  modifiersMinor: number;
  quantity: number;
}

export interface CartTotalsMinor {
  subtotalMinor: number;
  taxMinor: number;
  deliveryFeeMinor: number;
  grandTotalMinor: number;
}

export function calcLineMinor(line: CartLinePricing): number {
  return (line.unitPriceMinor + line.modifiersMinor) * line.quantity;
}

export function calcCartTotalsMinor(
  lines: CartLinePricing[],
  taxRateBps: number, // e.g. 1600 = 16.00%
  deliveryFeeMinor: number
): CartTotalsMinor {
  const subtotalMinor = lines.reduce((sum, l) => sum + calcLineMinor(l), 0);
  const taxMinor = Math.round((subtotalMinor * taxRateBps) / 10000);
  const grandTotalMinor = subtotalMinor + taxMinor + deliveryFeeMinor;
  return {
    subtotalMinor,
    taxMinor,
    deliveryFeeMinor,
    grandTotalMinor,
  };
}

export function formatMinor(minor: number, currency = "PKR"): string {
  const negative = minor < 0;
  const abs = Math.abs(minor);
  const major = Math.floor(abs / 100);
  const frac = (abs % 100).toString().padStart(2, "0");
  return `${negative ? "-" : ""}${currency} ${major.toLocaleString()}.${frac}`;
}
