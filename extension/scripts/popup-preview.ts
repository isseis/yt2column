// Renders the popup's and the viewer's states for the manual checks that jsdom
// cannot make: the line clamping, the previews' direction, the colours, the
// focus indicator and the motion. Run from the repository root, with the
// directory to write into:
//
//   node extension/scripts/popup-preview.ts <output-directory>
//
// Each collected input goes through the production functions -- collect,
// acceptMetadata (readFailureResult for a failed read), prepareSend,
// buildCollectedView and renderPopup -- so the preview cannot show a state the
// popup cannot build. A rejected case builds the RejectedView the launch path
// returns for a collection it refuses. A viewer page draws a ViewerView with
// renderViewerHeader and renderViewer, the functions the viewer uses. The
// skeletons are the shipped popup.html and viewer.html, the stylesheets are
// copied next to the pages, and the written pages load no external resource
// and run no script. This file never reaches dist/: the build
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
import type { JobDisplay } from "../src/core/jobView.ts";
import { classifyStatus } from "../src/core/response.ts";
import type { Article, PublishStatus, Steps } from "../src/core/response.ts";
import { findPopupElements } from "../src/launch.ts";
import { applyPageLocale } from "../src/ui/pageLocale.ts";
import { renderPopup } from "../src/ui/render.ts";
import { renderViewer, renderViewerHeader } from "../src/ui/renderViewer.ts";
import type { ViewerHeader, ViewerView } from "../src/ui/renderViewer.ts";
import { findViewerElements } from "../src/viewerApp.ts";

/** The directory holding static/, src/ and scripts/. */
const extensionDir = path.dirname(path.dirname(fileURLToPath(import.meta.url)));

/** The watch URL every collected preview is built from. */
const watchUrl = "https://www.youtube.com/watch?v=preview";

/** A selection whose request body is over the 4 MiB limit. */
const oversizedSelection = "a".repeat(4_200_000);

/** A caption line put before each tail, so the head preview differs from the tail preview. */
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

/** Left-to-right, right-to-left, mixed and marked tails, each at least 100 code points. */
const tailCases: readonly { readonly name: string; readonly tail: string }[] = [
  { name: "tail-ltr-en", tail: `${"left to right ".repeat(10)}END` },
  { name: "tail-rtl-en", tail: hebrewWord.repeat(30) },
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

const runningSteps: Steps = {
  draft: "done",
  shorten: "running",
  review: "pending",
};
const doneSteps: Steps = { draft: "done", shorten: "skipped", review: "done" };
const failedSteps: Steps = {
  draft: "done",
  shorten: "failed",
  review: "pending",
};

/**
 * The article of a done status response, checked by classifyStatus as the
 * viewer checks every status, so its branded cost is built by its parser.
 */
function previewArticleFrom(): Article {
  const body = {
    status: "ok",
    requestId: "0123456789abcdef",
    webhookConfigured: true,
    job: {
      state: "done",
      steps: doneSteps,
      article: {
        markdown:
          "# A video title\n\nA channel\n\nThe article's first paragraph, long enough to wrap on a narrow page. ".repeat(
            3,
          ),
        file: "---\ntitle: A video title\n---\n\nThe article\n",
        cost: { state: "known", usd: "0.1234" },
        durationMs: 81_234,
      },
      publish: { state: "not_posted" },
    },
  };
  const result = classifyStatus({
    kind: "response",
    status: 200,
    bytes: new TextEncoder().encode(JSON.stringify(body)),
  });
  if (result.kind !== "ok" || result.job.state !== "done") {
    throw new Error(`the preview article was refused: ${result.kind}`);
  }
  return result.job.article;
}

const previewArticle = previewArticleFrom();

function doneJob(publish: PublishStatus, webhookConfigured = true): JobDisplay {
  return {
    kind: "done",
    steps: doneSteps,
    article: previewArticle,
    publish,
    webhookConfigured,
  };
}

/** A viewer state drawn on the shipped viewer.html, one file each. */
export interface ViewerPreviewCase {
  readonly name: string;
  readonly locale: Locale;
  /** The URL's values; absent for a link that failed the URL check. */
  readonly header?: ViewerHeader;
  readonly view: ViewerView;
}

const viewerHeader: ViewerHeader = {
  heading: "A video title",
  channelName: "A channel",
  url: watchUrl,
};

/** A job view with no notice. */
function jobView(job: JobDisplay, elapsedMs = 0): ViewerView {
  return {
    state: { kind: "job", job, notice: { kind: "none" } },
    elapsedMs,
    publishing: false,
  };
}

/**
 * Every viewer state kind, every job kind and every row of the publish
 * button's table, one page each; the running job also in Japanese.
 */
export const viewerPreviewCases: readonly ViewerPreviewCase[] = [
  {
    name: "viewer-invalid-link-en",
    locale: "en",
    view: { state: { kind: "invalid-link" }, elapsedMs: 0, publishing: false },
  },
  {
    name: "viewer-sending-en",
    locale: "en",
    header: viewerHeader,
    view: { state: { kind: "sending" }, elapsedMs: 0, publishing: false },
  },
  {
    name: "viewer-start-failed-en",
    locale: "en",
    header: viewerHeader,
    view: {
      state: { kind: "start-failed", failure: { kind: "no-result" } },
      elapsedMs: 0,
      publishing: false,
    },
  },
  {
    name: "viewer-loading-en",
    locale: "en",
    header: viewerHeader,
    view: {
      state: { kind: "loading", notice: { kind: "none" } },
      elapsedMs: 0,
      publishing: false,
    },
  },
  {
    name: "viewer-running-en",
    locale: "en",
    header: viewerHeader,
    view: jobView({ kind: "running", steps: runningSteps }, 65_000),
  },
  {
    name: "viewer-running-ja",
    locale: "ja",
    header: viewerHeader,
    view: jobView({ kind: "running", steps: runningSteps }, 65_000),
  },
  {
    name: "viewer-failed-en",
    locale: "en",
    header: viewerHeader,
    view: jobView({
      kind: "failed",
      steps: failedSteps,
      failure: "llm_failed",
    }),
  },
  {
    name: "viewer-lost-en",
    locale: "en",
    header: viewerHeader,
    view: jobView({ kind: "lost", kept: previewArticle }),
  },
  {
    name: "viewer-posted-en",
    locale: "en",
    header: viewerHeader,
    view: jobView(doneJob({ state: "posted" })),
  },
  {
    name: "viewer-posting-en",
    locale: "en",
    header: viewerHeader,
    view: jobView(doneJob({ state: "posting" })),
  },
  {
    name: "viewer-unpostable-en",
    locale: "en",
    header: viewerHeader,
    view: jobView(doneJob({ state: "unpostable", reason: "mention" })),
  },
  {
    name: "viewer-webhook-not-configured-en",
    locale: "en",
    header: viewerHeader,
    view: jobView(doneJob({ state: "not_posted" }, false)),
  },
  {
    name: "viewer-publish-failed-en",
    locale: "en",
    header: viewerHeader,
    view: jobView(doneJob({ state: "failed", reason: "webhook" })),
  },
  {
    name: "viewer-not-posted-en",
    locale: "en",
    header: viewerHeader,
    view: jobView(doneJob({ state: "not_posted" })),
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
function renderPage(skeleton: string, view: PopupView, locale: Locale): string {
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

/** The viewer page for one case: the shipped skeleton, the locale, and the render. */
function renderViewerPage(
  skeleton: string,
  previewCase: ViewerPreviewCase,
): string {
  const dom = new JSDOM(skeleton);
  const doc = dom.window.document;
  applyPageLocale(doc, previewCase.locale);
  const elements = findViewerElements(doc);
  if (elements === undefined) {
    throw new Error("viewer.html does not have the skeleton's elements");
  }
  renderViewerHeader(elements, previewCase.header, previewCase.locale);
  renderViewer(elements, previewCase.view, previewCase.locale);
  for (const script of doc.querySelectorAll("script")) {
    script.remove();
  }
  return dom.serialize();
}

/** The stylesheets popup.html and viewer.html load, copied next to the pages. */
const stylesheets = ["theme.css", "style.css", "viewer.css"] as const;

/** Writes one page per input set plus the stylesheets into outputDir. */
async function writePreviews(outputDir: string): Promise<void> {
  mkdirSync(outputDir, { recursive: true });
  for (const stylesheet of stylesheets) {
    copyFileSync(
      path.join(extensionDir, "static", stylesheet),
      path.join(outputDir, stylesheet),
    );
  }
  const skeleton = readFileSync(
    path.join(extensionDir, "static", "popup.html"),
    "utf8",
  );
  for (const previewCase of previewCases) {
    const view = await buildView(previewCase);
    writeFileSync(
      path.join(outputDir, `${previewCase.name}.html`),
      renderPage(skeleton, view, previewCase.locale),
    );
  }
  const viewerSkeleton = readFileSync(
    path.join(extensionDir, "static", "viewer.html"),
    "utf8",
  );
  for (const previewCase of viewerPreviewCases) {
    writeFileSync(
      path.join(outputDir, `${previewCase.name}.html`),
      renderViewerPage(viewerSkeleton, previewCase),
    );
  }
}

if (import.meta.main) {
  const outputDir = process.argv[2];
  if (outputDir === undefined) {
    process.stderr.write(
      "usage: node extension/scripts/popup-preview.ts <output-directory>\n",
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
