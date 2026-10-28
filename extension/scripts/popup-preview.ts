// Renders the popup's states for the manual checks that jsdom cannot make:
// the line clamping, the previews' direction, the colours and the focus
// indicator. Run from the repository root, with the directory to write into:
//
//   node extension/scripts/popup-preview.ts <output-directory>
//
// Each collected input goes through the production functions -- collect,
// acceptMetadata (readFailureResult for a failed read), prepareSend,
// buildCollectedView and renderPopup -- so the preview cannot show a state the
// popup cannot build. A rejected case builds the RejectedView the launch path
// returns for a collection it refuses. The skeleton is the shipped popup.html,
// the stylesheet is copied next to the pages, and the written pages load no
// external resource and run no script. This file never reaches dist/: the build
// compiles src/ only (tsconfig.build.json) and check-dist reports any extra file.
//
// Text that must be Japanese, right-to-left, or a bidi control is written with
// \u escapes so the source stays ASCII; each such value says what it is.

import { copyFileSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { JSDOM } from "jsdom";

import { collect } from "../src/core/collect.ts";
import type { SelectionReader } from "../src/core/collect.ts";
import { acceptMetadata, readFailureResult } from "../src/core/metadata.ts";
import type { MetadataResult } from "../src/core/metadata.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { prepareSend } from "../src/core/prepare.ts";
import { buildCollectedView } from "../src/core/popupView.ts";
import type { PopupView } from "../src/core/popupView.ts";
import type { RejectionReason } from "../src/core/types.ts";
import { findPopupElements } from "../src/launch.ts";
import { applyPageLocale } from "../src/ui/pageLocale.ts";
import { renderPopup } from "../src/ui/render.ts";

/** The directory holding static/, src/ and scripts/. */
const extensionDir = path.dirname(path.dirname(fileURLToPath(import.meta.url)));

/** The watch URL every collected preview is built from. */
const watchUrl = "https://www.youtube.com/watch?v=preview";

/** A selection whose request body is over the 4 MiB limit. */
const oversizedSelection = "a".repeat(4_200_000);

/** A response line at the start of a selection whose tail is above 100 code points. */
const selectionPrefix = `00:00 ${"a".repeat(120)}\n`;

/** Japanese "a long video title", past two lines at 360 CSS pixels. */
const japaneseTitle =
  "\u9577\u3044\u52d5\u753b\u306e\u30bf\u30a4\u30c8\u30eb".repeat(6);

/** Japanese "a channel name", cut to the 200-code-point display limit. */
const japaneseChannelName = "\u30c1\u30e3\u30f3\u30cd\u30eb\u540d"
  .repeat(40)
  .slice(0, 200);

/** Japanese "a tab title". */
const japaneseTabTitle = "\u30bf\u30d6\u306e\u30bf\u30a4\u30c8\u30eb";

/** Hebrew "shalom", for a right-to-left tail. */
const hebrewWord = "\u05e9\u05dc\u05d5\u05dd";

/** Left-to-right, right-to-left, mixed and marked tails, each longer than 100 code points. */
const tailCases: readonly { readonly name: string; readonly tail: string }[] = [
  { name: "tail-ltr-en", tail: `${"left to right ".repeat(10)}END` },
  { name: "tail-rtl-en", tail: `${hebrewWord.repeat(30)}` },
  { name: "tail-mixed-en", tail: `${"a".repeat(50)}\u05e9`.repeat(2) },
  { name: "tail-mark-en", tail: `${"a".repeat(99)}\u200f` },
];

/** A view built from page values, or a collection the popup refuses to show. */
type PreviewCase =
  | {
      readonly kind: "collected";
      readonly name: string;
      readonly locale: Locale;
      readonly selection: string;
      readonly tabTitle: string;
      readonly metadata: MetadataResult;
    }
  | {
      readonly kind: "rejected";
      readonly name: string;
      readonly locale: Locale;
      readonly reason: RejectionReason;
    };

/** The input sets the manual checks use, one file each. */
export const previewCases: readonly PreviewCase[] = [
  {
    kind: "collected",
    name: "worst-case-en",
    locale: "en",
    selection: oversizedSelection,
    tabTitle: "A tab title",
    metadata: acceptMetadata({
      title:
        "A very long video title that needs more than two lines on the page".repeat(
          2,
        ),
      channelName: "A very long channel name ".repeat(10),
      description: "A description",
      descriptionCollapsed: true,
    }),
  },
  {
    kind: "collected",
    name: "worst-case-ja",
    locale: "ja",
    selection: oversizedSelection,
    tabTitle: japaneseTabTitle,
    metadata: acceptMetadata({
      title: japaneseTitle,
      channelName: japaneseChannelName,
      description: "\u8aac\u660e",
      descriptionCollapsed: true,
    }),
  },
  ...tailCases.map(({ name, tail }): PreviewCase => ({
    kind: "collected",
    name,
    locale: "en",
    selection: `${selectionPrefix}${tail}`,
    tabTitle: "A tab title",
    metadata: acceptMetadata({ descriptionCollapsed: false }),
  })),
  {
    kind: "collected",
    name: "rtl-all-en",
    locale: "en",
    selection: hebrewWord.repeat(30),
    tabTitle: "A tab title",
    metadata: acceptMetadata({ descriptionCollapsed: false }),
  },
  {
    kind: "collected",
    name: "bidi-control-en",
    locale: "en",
    selection: `\u202e${"a".repeat(200)}\u202e`,
    tabTitle: "\u202eA tab title",
    metadata: acceptMetadata({
      title: "\u202eA video title",
      channelName: "\u202eA channel name",
      description: "A description",
      descriptionCollapsed: false,
    }),
  },
  {
    kind: "collected",
    name: "read-failure-en",
    locale: "en",
    selection: `00:00 a caption\n00:05 another caption`,
    tabTitle: "A tab title",
    metadata: readFailureResult(),
  },
  {
    kind: "rejected",
    name: "rejected-en",
    locale: "en",
    reason: "not-watch-page",
  },
  {
    kind: "rejected",
    name: "rejected-ja",
    locale: "ja",
    reason: "empty-selection",
  },
];

/** A reader that returns text as the page's selection at the watch URL. */
function readerReturning(text: string): SelectionReader {
  return {
    read: async () => ({ text, documentUrl: watchUrl }),
  };
}

/** The view one input set shows, built by the production functions. */
async function buildView(previewCase: PreviewCase): Promise<PopupView> {
  if (previewCase.kind === "rejected") {
    return { kind: "rejected", reason: previewCase.reason };
  }
  const outcome = await collect(
    { tabId: 1, url: watchUrl, title: previewCase.tabTitle },
    readerReturning(previewCase.selection),
  );
  if (outcome.kind !== "collected") {
    throw new Error(
      `the input set ${previewCase.name} was rejected: ${outcome.reason}`,
    );
  }
  return buildCollectedView(
    outcome.input,
    previewCase.metadata,
    prepareSend(outcome.input, previewCase.metadata.values),
  );
}

/** The page for one view: the shipped skeleton, the locale, and the render. */
function renderPage(view: PopupView, locale: Locale): string {
  const skeleton = readFileSync(
    path.join(extensionDir, "static", "popup.html"),
    "utf8",
  );
  const dom = new JSDOM(skeleton);
  const doc = dom.window.document;
  applyPageLocale(doc, locale);
  const elements = findPopupElements(doc);
  if (elements === undefined) {
    throw new Error("popup.html does not have the skeleton's elements");
  }
  renderPopup(elements, view, locale);
  // The preview is a static page: no script runs next to the rendered state.
  for (const script of doc.querySelectorAll("script")) {
    script.remove();
  }
  return dom.serialize();
}

/** Writes one page per input set plus the stylesheet into outputDir. */
async function writePreviews(outputDir: string): Promise<void> {
  mkdirSync(outputDir, { recursive: true });
  copyFileSync(
    path.join(extensionDir, "static", "style.css"),
    path.join(outputDir, "style.css"),
  );
  for (const previewCase of previewCases) {
    const view = await buildView(previewCase);
    writeFileSync(
      path.join(outputDir, `${previewCase.name}.html`),
      renderPage(view, previewCase.locale),
    );
  }
}

if (import.meta.main) {
  const outputDir = process.argv[2];
  if (outputDir === undefined) {
    process.stderr.write(
      "usage: node scripts/popup-preview.ts <output-directory>\n",
    );
    process.exit(2);
  }
  try {
    await writePreviews(outputDir);
  } catch (error) {
    process.stderr.write(
      `popup-preview: ${error instanceof Error ? error.message : String(error)}\n`,
    );
    process.exit(1);
  }
}
