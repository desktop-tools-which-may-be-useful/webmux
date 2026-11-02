import type { ThemesType } from "@pierre/diffs";

export const codeThemes: ThemesType = {
  light: "pierre-light",
  dark: "pierre-dark",
};

export const codeLineHeight = 22;
export const codeItemMetrics = { lineHeight: codeLineHeight };
export const codeTokenizeLimit = 4096;

/** Only fixed application CSS enters the library's shadow roots. */
export const codeUnsafeCSS = `
  [data-line], [data-column-number] { font-variant-ligatures: none; }
`;

/** The full-file reader kept CodeMirror's four-column tabs; Diff did not. */
export const fileUnsafeCSS = `${codeUnsafeCSS}
  [data-line] { tab-size: 4; }
`;

export function pierreTheme(dark: boolean) {
  return {
    theme: codeThemes,
    themeType: dark ? ("dark" as const) : ("light" as const),
  };
}

/** Calls `update` now and whenever the document root gains or loses `.dark`. */
export function observeDarkTheme(update: (dark: boolean) => void) {
  const read = () =>
    update(document.documentElement.classList.contains("dark"));
  read();
  const observer = new MutationObserver(read);
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class"],
  });
  return () => observer.disconnect();
}
