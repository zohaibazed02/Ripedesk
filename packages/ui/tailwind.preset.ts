import type { Config } from "tailwindcss";

const hospitalityPreset: Partial<Config> = {
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        espresso: {
          DEFAULT: "#0F172A",
          950: "#09090B",
          900: "#0F172A",
          800: "#1E293B",
          700: "#334155",
        },
        terracotta: {
          DEFAULT: "#EA580C",
          50: "#FFF7ED",
          100: "#FFEDD5",
          500: "#F97316",
          600: "#EA580C",
          700: "#C2410C",
        },
        surface: {
          canvas: "var(--surface-canvas)",
          card: "var(--surface-card)",
          subtle: "var(--surface-subtle)",
          border: "var(--surface-border)",
        },
        ink: {
          primary: "var(--ink-primary)",
          secondary: "var(--ink-secondary)",
          muted: "var(--ink-muted)",
        },
        status: {
          online: "#10B981",
          warning: "#F59E0B",
          danger: "#E11D48",
        },
      },
      fontFamily: {
        sans: ["var(--font-inter)", "Inter", "system-ui", "-apple-system", "sans-serif"],
        mono: ["var(--font-jetbrains-mono)", "JetBrains Mono", "ui-monospace", "monospace"],
      },
      boxShadow: {
        card: "0 1px 2px 0 rgba(15, 23, 42, 0.04)",
        elevated: "0 8px 24px -6px rgba(15, 23, 42, 0.12)",
      },
      keyframes: {
        "status-pulse": {
          "0%, 100%": { opacity: "1", transform: "scale(1)" },
          "50%": { opacity: "0.45", transform: "scale(1.18)" },
        },
      },
      animation: {
        "status-pulse": "status-pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite",
      },
    },
  },
};

export default hospitalityPreset;
