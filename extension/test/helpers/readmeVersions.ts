import type { Locale } from "../../src/core/i18n/locale.ts";
import { developmentDoc } from "./developmentDoc.ts";
import { installGuide } from "./installGuide.ts";

/**
 * The two versions of the README: README.md, the Japanese primary version, and
 * README.en.md, its English version. A version's locale selects the wording of
 * the minimum browser version it states.
 */
export const readmeVersions: readonly { path: string; locale: Locale }[] = [
  { path: "README.md", locale: "ja" },
  { path: "README.en.md", locale: "en" },
];

/**
 * Every document that states the minimum browser version, with the language
 * whose phrase it uses: the README versions, the install guide, and the
 * development guide.
 */
export const minimumChromeDocs: readonly { path: string; locale: Locale }[] = [
  ...readmeVersions,
  installGuide,
  { path: developmentDoc.path, locale: "ja" },
];

/**
 * How a document in each language states the minimum browser version, for
 * Chrome and for the Chromium that Brave is built on, with the version number
 * as the first group. The phrase must equal that language's wording.
 */
export const minimumChromePhrase: Record<Locale, RegExp> = {
  ja: /(?:Chrome|Chromium) (\d+) 以降/gu,
  en: /(?:Chrome|Chromium) (\d+) or later/gu,
};
