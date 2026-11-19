import type { Config } from "tailwindcss";
import hospitalityPreset from "../../packages/ui/tailwind.preset";

const config: Config = {
  presets: [hospitalityPreset],
  content: [
    "./src/**/*.{js,ts,jsx,tsx,mdx}",
    "../../packages/ui/src/**/*.{js,ts,jsx,tsx,mdx}",
  ],
};

export default config;
