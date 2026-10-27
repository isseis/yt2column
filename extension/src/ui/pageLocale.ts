import type { Locale } from "../core/i18n/locale.ts";

/**
 * Sets the lang attribute of the document's root element to the locale. The
 * popup calls this before its first render, so the Japanese font and line
 * height rules apply from the first frame; child elements inherit them, so a
 * per-element font rule such as the request id's monospace still wins. Only a
 * Locale value reaches the attribute, never a browser-supplied string.
 */
export function applyPageLocale(doc: Document, locale: Locale): void {
  doc.documentElement.lang = locale;
}
