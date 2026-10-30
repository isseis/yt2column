import type { Locale } from "../../src/core/i18n/locale.ts";

/**
 * The two versions of the README: README.md, the Japanese primary version, and
 * README.en.md, its English version. A version's locale is also the message
 * catalog whose popup text that version quotes.
 */
export const readmeVersions: readonly { path: string; locale: Locale }[] = [
  { path: "README.md", locale: "ja" },
  { path: "README.en.md", locale: "en" },
];

/**
 * The heading of the README's browser extension section in each version. Each
 * spelling must equal that version's heading line.
 */
export const browserExtensionHeading: Record<Locale, string> = {
  ja: "## ブラウザ拡張",
  en: "## Browser extension",
};
