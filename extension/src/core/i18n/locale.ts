/** The language the extension displays in. */
export type Locale = "en" | "ja";

/**
 * Runtime iteration order of the locales, and the keys resolveLocale compares
 * with. Typed as Record<Locale, true> so it stays in step with the union: a
 * locale added to one must be added to the other, or the type check fails.
 */
export const locales: Record<Locale, true> = {
  en: true,
  ja: true,
};

/** True when value is one of the locales' keys; an own property, not an inherited one. */
function isLocale(value: string): value is Locale {
  return Object.prototype.hasOwnProperty.call(locales, value);
}

/**
 * Maps the browser's UI language to a Locale. This is the only place that
 * chooses a behavior from the content of the UI-language string: the part
 * before the first "-", lowercased, must equal a locale's key exactly, and
 * every other value -- undefined included -- resolves to English. Only "-"
 * separates the subtag, so "ja_JP" is not Japanese.
 */
export function resolveLocale(uiLanguage: string | undefined): Locale {
  if (uiLanguage === undefined) {
    return "en";
  }
  const [primary = ""] = uiLanguage.split("-", 1);
  const candidate = primary.toLowerCase();
  return isLocale(candidate) ? candidate : "en";
}

/** Formats a count with the locale's number format. */
export function formatCount(value: number, locale: Locale): string {
  return new Intl.NumberFormat(locale).format(value);
}
