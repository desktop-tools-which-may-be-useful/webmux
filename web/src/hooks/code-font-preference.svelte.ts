import { store, stored } from "../api";
import { CodeFontSizeSchema } from "../schemas";

// Screens differ in size, so the code font is a per-browser preference and
// never syncs through server settings. Every code reader and the settings page
// share this module state; an unset or invalid value falls back to 13px.
export const minCodeFontSize = 10;
export const maxCodeFontSize = 24;
export const defaultCodeFontSize = 13;
const key = "codeFontSize";

const clampCodeFontSize = (value: number) =>
  Math.max(minCodeFontSize, Math.min(maxCodeFontSize, Math.round(value)));

const codeLineHeightFor = (fontSize: number) =>
  Math.round((clampCodeFontSize(fontSize) * 22) / defaultCodeFontSize);

function load(): number | null {
  const parsed = CodeFontSizeSchema.safeParse(stored(key, "x"));
  return parsed.success ? parsed.data : null;
}

let size = $state<number | null>(load());

export const codeFont = {
  get size() {
    return size ?? defaultCodeFontSize;
  },
  get lineHeight() {
    return codeLineHeightFor(this.size);
  },
  get itemMetrics() {
    return { lineHeight: this.lineHeight };
  },
  set(value: number) {
    size = clampCodeFontSize(value);
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
