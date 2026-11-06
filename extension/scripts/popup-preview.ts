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
import type {
  JobPopupView,
  PopupJobState,
  PopupView,
} from "../src/core/popupView.ts";
import type { RejectionReason } from "../src/core/types.ts";
import type { JobDisplay, PollNotice } from "../src/core/jobView.ts";
import { classifyStatus, parseRequestId } from "../src/core/response.ts";
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
 * A request id made up for the preview pages, in the server's form (16
 * lowercase hexadecimal digits). The server draws its ids at random from every
 * value of that form, so no value is reserved; this one is a placeholder only
 * because the preview never talks to a server. It is the id of every status
 * response the preview builds, and a rejected notice shows it after its
 * message.
 */
const previewRequestId = "0123456789abcdef";

/** Japanese "the article's title". */
const japaneseArticleTitle = "記事のタイトル";

/** Japanese "a section heading". */
const japaneseSectionHeading = "節の見出し";

/**
 * Japanese "the first paragraph; a sentence long enough to wrap on a narrow
 * page," repeated to wrap.
 */
const japaneseLongLine =
  "最初の段落です。狭いページでは折り返す長さの文で、".repeat(3);

/** Japanese "a line break inside a line becomes a break inside the paragraph." */
const japaneseBreakLine = "行の途中の改行は、段落の中の改行になります。";

/** Japanese "speaker", followed by a full-width colon. */
const japaneseSpeaker = "話者";
const fullWidthColon = "：";

/** Japanese "source: ", the start of the source line. */
const japaneseSource = "出典: ";

/**
 * The display Markdown of the done article, with every block and inline
 * piece the viewer draws: the title and a section heading, the channel name
 * line, a paragraph with a line break, strong and emphasised text, a code
 * span, a bullet list, an ordered list from a nine-digit start number, a code
 * block with a long line, lines that start with a right-to-left word and with
 * U+202E (RIGHT-TO-LEFT OVERRIDE) above a line with a link, and the source
 * line. It holds no CR: the test reads the written page back as HTML, which
 * turns a CR into a LF.
 */
const previewMarkdown = [
  `# ${japaneseArticleTitle}`,
  "",
  "A channel",
  "",
  `## ${japaneseSectionHeading}`,
  "",
  japaneseLongLine,
  japaneseBreakLine,
  "",
  `**${japaneseSpeaker}**${fullWidthColon}A *stressed* word and a \`code span\`.`,
  "",
  "- A bullet item",
  "- Another bullet item",
  "",
  "999999998. The first item of a nine-digit start number",
  "999999998. The second item",
  "",
  "```text",
  `${"a long line of code ".repeat(12)}end`,
  "```",
  "",
  `${hebrewWord} starts this line.`,
  "‮This line starts with an override.",
  "A link after them: <https://example.com/after>",
  "",
  `${japaneseSource}<${watchUrl}>`,
].join("\n");

/**
 * The article of a done status response, checked by classifyStatus as the
 * viewer checks every status, so its branded cost is built by its parser.
 */
function previewArticleFrom(): Article {
  const body = {
    status: "ok",
    requestId: previewRequestId,
    webhookConfigured: true,
    job: {
      state: "done",
      steps: doneSteps,
      article: {
        markdown: previewMarkdown,
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
    existing: false,
  };
}

/**
 * Every viewer state kind, every job kind and every row of the publish
 * button's table, one page each; the running job also in Japanese, a loading
 * state whose polling stopped, a publish in flight, and an existing article's
 * running and done jobs in both locales.
 */
export const viewerPreviewCases: readonly ViewerPreviewCase[] = [
  {
    name: "viewer-invalid-link-en",
    locale: "en",
    view: {
      state: { kind: "invalid-link" },
      elapsedMs: 0,
      publishing: false,
      existing: false,
    },
  },
  {
    name: "viewer-sending-en",
    locale: "en",
    header: viewerHeader,
    view: {
      state: { kind: "sending" },
      elapsedMs: 0,
      publishing: false,
      existing: false,
    },
  },
  {
    name: "viewer-start-failed-en",
    locale: "en",
    header: viewerHeader,
    view: {
      state: { kind: "start-failed", failure: { kind: "no-result" } },
      elapsedMs: 0,
      publishing: false,
      existing: false,
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
      existing: false,
    },
  },
  {
    name: "viewer-loading-stopped-en",
    locale: "en",
    header: viewerHeader,
    view: {
      state: {
        kind: "loading",
        notice: {
          kind: "stopped",
          outcome: { kind: "unexpected", check: "syntax" },
        },
      },
      elapsedMs: 0,
      publishing: false,
      existing: false,
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
    name: "viewer-publishing-en",
    locale: "en",
    header: viewerHeader,
    view: { ...jobView(doneJob({ state: "not_posted" })), publishing: true },
  },
  {
    name: "viewer-not-posted-en",
    locale: "en",
    header: viewerHeader,
    view: jobView(doneJob({ state: "not_posted" })),
  },
  // A viewer of an existing article of the same video (its URL holds e=1),
  // running and done, in both locales.
  ...(["en", "ja"] as const).flatMap((locale) => [
    {
      name: `viewer-existing-running-${locale}`,
      locale,
      header: viewerHeader,
      view: {
        ...jobView({ kind: "running", steps: runningSteps }, 65_000),
        existing: true,
      },
    },
    {
      name: `viewer-existing-done-${locale}`,
      locale,
      header: viewerHeader,
      view: { ...jobView(doneJob({ state: "not_posted" })), existing: true },
    },
  ]),
];

/** A popup job state drawn on the shipped popup.html, one file each. */
export interface PopupJobPreviewCase {
  readonly name: string;
  readonly locale: Locale;
  readonly state: PopupJobState;
  readonly elapsedMs: number;
  /**
   * The input set whose video section and details the page keeps; absent for
   * a short heading and no channel name. A worst case measures whether the
   * buttons fit without scrolling.
   */
  readonly base?: "worst-case-en" | "worst-case-ja";
}

const noNotice = { kind: "none" } as const;
const retryingNotice = {
  kind: "retrying",
  outcome: { kind: "not-sent", cause: "server-unavailable" },
} as const;
const stoppedNotice = {
  kind: "stopped",
  outcome: { kind: "unexpected", check: "syntax" },
} as const;

/**
 * The tallest notice the choice can show beside a running article: a
 * rejected status, whose message is followed by its request id.
 */
function rejectedNotice(): PollNotice {
  const requestId = parseRequestId(previewRequestId);
  if (requestId === undefined) {
    throw new Error("the preview request id was refused");
  }
  return {
    kind: "stopped",
    outcome: { kind: "rejected", reason: "internal_error", requestId },
  };
}

/**
 * The choice with each existing-article state and each poll notice, the
 * ended choice, and the worst case of the choice in both locales.
 */
const choicePreviewCases: readonly PopupJobPreviewCase[] = [
  {
    name: "popup-choice-loading-en",
    locale: "en",
    state: { kind: "choice", existing: "loading", notice: noNotice },
    elapsedMs: 0,
  },
  {
    name: "popup-choice-running-en",
    locale: "en",
    state: { kind: "choice", existing: "running", notice: noNotice },
    elapsedMs: 0,
  },
  {
    name: "popup-choice-done-en",
    locale: "en",
    state: { kind: "choice", existing: "done", notice: noNotice },
    elapsedMs: 0,
  },
  {
    name: "popup-choice-done-ja",
    locale: "ja",
    state: { kind: "choice", existing: "done", notice: noNotice },
    elapsedMs: 0,
  },
  {
    name: "popup-choice-retrying-en",
    locale: "en",
    state: { kind: "choice", existing: "running", notice: retryingNotice },
    elapsedMs: 0,
  },
  {
    name: "popup-choice-stopped-en",
    locale: "en",
    state: { kind: "choice", existing: "loading", notice: stoppedNotice },
    elapsedMs: 0,
  },
  {
    name: "popup-choice-worst-case-en",
    locale: "en",
    state: { kind: "choice", existing: "running", notice: rejectedNotice() },
    elapsedMs: 0,
    base: "worst-case-en",
  },
  {
    name: "popup-choice-worst-case-ja",
    locale: "ja",
    state: { kind: "choice", existing: "running", notice: rejectedNotice() },
    elapsedMs: 0,
    base: "worst-case-ja",
  },
  {
    name: "popup-choice-ended-en",
    locale: "en",
    state: { kind: "choice-ended" },
    elapsedMs: 0,
  },
  {
    name: "popup-choice-ended-ja",
    locale: "ja",
    state: { kind: "choice-ended" },
    elapsedMs: 0,
  },
];

/**
 * Every popup job state kind and every job kind, with the running state also
 * in Japanese, then the choice's pages.
 */
export const popupJobPreviewCases: readonly PopupJobPreviewCase[] = [
  {
    name: "popup-job-sending-en",
    locale: "en",
    state: { kind: "sending" },
    elapsedMs: 0,
  },
  {
    name: "popup-job-start-failed-en",
    locale: "en",
    state: { kind: "start-failed", failure: { kind: "no-result" } },
    elapsedMs: 0,
  },
  {
    name: "popup-job-loading-en",
    locale: "en",
    state: { kind: "loading", notice: noNotice },
    elapsedMs: 0,
  },
  {
    name: "popup-job-loading-stopped-en",
    locale: "en",
    state: {
      kind: "loading",
      notice: {
        kind: "stopped",
        outcome: { kind: "unexpected", check: "syntax" },
      },
    },
    elapsedMs: 0,
  },
  {
    name: "popup-job-running-en",
    locale: "en",
    state: {
      kind: "job",
      job: { kind: "running", steps: runningSteps },
      notice: noNotice,
    },
    elapsedMs: 65_000,
  },
  {
    name: "popup-job-running-ja",
    locale: "ja",
    state: {
      kind: "job",
      job: { kind: "running", steps: runningSteps },
      notice: noNotice,
    },
    elapsedMs: 65_000,
  },
  {
    name: "popup-job-done-en",
    locale: "en",
    state: {
      kind: "job",
      job: doneJob({ state: "not_posted" }),
      notice: noNotice,
    },
    elapsedMs: 81_000,
  },
  {
    name: "popup-job-failed-en",
    locale: "en",
    state: {
      kind: "job",
      job: { kind: "failed", steps: failedSteps, failure: "llm_failed" },
      notice: noNotice,
    },
    elapsedMs: 0,
  },
  {
    name: "popup-job-lost-en",
    locale: "en",
    state: {
      kind: "job",
      job: { kind: "lost", kept: previewArticle },
      notice: noNotice,
    },
    elapsedMs: 0,
  },
  ...choicePreviewCases,
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
function renderPage(
  skeleton: string,
  view: PopupView,
  locale: Locale,
  elapsedMs = 0,
): string {
  const dom = new JSDOM(skeleton);
  const doc = dom.window.document;
  applyPageLocale(doc, locale);
  const elements = findPopupElements(doc);
  if (elements === undefined) {
    throw new Error("popup.html does not have the skeleton's elements");
  }
  renderPopup(elements, view, locale, elapsedMs);
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

/** The video section and details every popup job preview shares, built by buildView. */
async function collectedPreviewBase() {
  const view = await buildView({
    kind: "collected",
    name: "popup-job-base",
    locale: "en",
    selection: "00:00 a caption",
    tabTitle: "A tab title",
    metadata: acceptMetadata({ descriptionCollapsed: false }),
  });
  if (view.kind !== "collected") {
    throw new Error("the popup job preview's collected base was rejected");
  }
  return view;
}

/** The collected view of the input set named name, built by buildView. */
async function namedCollectedView(name: string) {
  const previewCase = previewCases.find((candidate) => candidate.name === name);
  if (previewCase === undefined) {
    throw new Error(`no input set is named ${name}`);
  }
  const view = await buildView(previewCase);
  if (view.kind !== "collected") {
    throw new Error(`the input set ${name} was rejected`);
  }
  return view;
}

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
  const base = await collectedPreviewBase();
  for (const previewCase of popupJobPreviewCases) {
    const collected =
      previewCase.base === undefined
        ? base
        : await namedCollectedView(previewCase.base);
    const view: JobPopupView = {
      kind: "job",
      video: collected.video,
      details: collected.details,
      state: previewCase.state,
    };
    writeFileSync(
      path.join(outputDir, `${previewCase.name}.html`),
      renderPage(skeleton, view, previewCase.locale, previewCase.elapsedMs),
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
