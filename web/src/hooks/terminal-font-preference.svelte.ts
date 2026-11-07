import { store, stored } from "../api";
import { FontSizeSchema } from "../schemas";

// Screens differ in size, so the font is a per-browser preference and never
// syncs through server settings. Every pane and the settings page share this
// module state; an unset or invalid value falls back to the device default.
export const minFontSize = 8;
export const maxFontSize = 20;
const key = "fontSize";

const defaultFontSize = () =>
  matchMedia("(pointer: coarse)").matches ? 15 : 14;
const clampFontSize = (value: number) =>
  Math.max(minFontSize, Math.min(maxFontSize, Math.round(value)));

function load(): number | null {
  const parsed = FontSizeSchema.safeParse(stored(key, "x"));
  return parsed.success ? parsed.data : null;
}

let size = $state<number | null>(load());

export const terminalFont = {
  // Effective size: the saved value or the pointer-based default.
  get size() {
    return size ?? defaultFontSize();
  },
  set(value: number) {
    size = clampFontSize(value);
    store(key, String(size));
  },
  reset() {
    size = null;
    try {
      localStorage.removeItem("webmux." + key);
    } catch {
      /* private-mode storage is optional */
    }
  },
};
