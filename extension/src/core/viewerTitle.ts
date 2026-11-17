// The viewer page's title (document.title), which the browser's history and
// tab strip show. It names the video the viewer is for, so the history tells
// one article from another, and ends with the extension's name, which marks a
// development build. The value comes from the same place as the viewer's
// heading: the job record, the start context, or, when neither is left, the
// title the viewer's URL carries. A link the viewer does not accept gets a
// fixed title for its state. The title is never empty.

import type { BuildKind } from "../buildKind.d.ts";
import { catalogs } from "./i18n/catalog.ts";
import type { Locale } from "./i18n/locale.ts";

/** Where a viewer's title comes from. */
export type TitleSource =
  /**
   * A job record: the video title the viewer heads the article with (the
   * module's neutralized one once done), and the heading collected with it.
   */
  | {
      readonly kind: "record";
      readonly videoTitle: string;
      readonly heading: string;
    }
  /** The start context, before a record exists: its video title and heading. */
  | {
      readonly kind: "start";
      readonly videoTitle: string;
      readonly heading: string;
    }
  /** Neither is left (the article was lost, or the context expired): the URL's title. */
  | { readonly kind: "url"; readonly urlTitle: string }
  /** The viewer's URL is not one it accepts. */
  | { readonly kind: "invalid-link" };

/**
 * The page title for source, in locale, ending with the name of buildKind:
 * "<part> - <name>". The part is the video title, then the heading when the
 * video title is empty, then a fixed text when both are; an invalid link's
 * part is the fixed text of that state.
 */
export function viewerTitle(
  source: TitleSource,
  buildKind: BuildKind,
  locale: Locale,
): string {
  const texts = catalogs[locale].viewerTitle;
  const firstNonEmpty = (...values: string[]): string =>
    values.find((value) => value !== "") ?? texts.untitled;
  let part: string;
  switch (source.kind) {
    case "record":
    case "start":
      part = firstNonEmpty(source.videoTitle, source.heading);
      break;
    case "url":
      part = firstNonEmpty(source.urlTitle);
      break;
    case "invalid-link":
      part = texts.invalidLink;
      break;
  }
  return `${part} - ${texts.name[buildKind]}`;
}
