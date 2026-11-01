import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import type { Message } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import type {
  JobDisplay,
  PollNotice,
  ViewerState,
} from "../src/core/jobView.ts";
import {
  publishResultMessage,
  startResultMessage,
} from "../src/core/sendMessages.ts";
import {
  renderActionNotice,
  renderViewer,
  renderViewerHeader,
} from "../src/ui/renderViewer.ts";
import type {
  ActionNotice,
  ViewerElements,
  ViewerView,
} from "../src/ui/renderViewer.ts";
import {
  expectLocaleMessage,
  expectMessage,
  query,
  viewerElements,
} from "./helpers/dom.ts";
import { article, doneJob, requestId, runningSteps } from "./helpers/job.ts";

const localeList = Object.keys(locales) as Locale[];
const malicious = "<img src=x onerror=alert(1)>";
const watchUrl = "https://www.youtube.com/watch?v=abc";
const none: PollNotice = { kind: "none" };
const running: JobDisplay = { kind: "running", steps: runningSteps };

/** A view of state, with the elapsed time and the publishing flag. */
function view(
  state: ViewerState,
  elapsedMs = 0,
  publishing = false,
): ViewerView {
  return { state, elapsedMs, publishing };
}

/** A job state with no notice. */
function jobState(job: JobDisplay, notice: PollNotice = none): ViewerState {
  return { kind: "job", job, notice };
}

/** Fresh skeleton elements with the view drawn into them. */
function drawn(viewerView: ViewerView, locale: Locale): ViewerElements {
  const elements = viewerElements();
  renderViewer(elements, viewerView, locale);
  return elements;
}

/** Every element a user can press or focus that is shown, in document order. */
function shownControls(elements: ViewerElements): Element[] {
  const doc = elements.state.ownerDocument;
  return Array.from(
    doc.querySelectorAll(
      "a[href], button, input, select, textarea, summary, [tabindex]",
    ),
  ).filter((element) => element.closest("[hidden]") === null);
}

describe("renderViewerHeader", () => {
  for (const locale of localeList) {
    it(`shows the URL's heading and channel name as text in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: malicious, channelName: "A channel", url: watchUrl },
        locale,
      );
      assert.equal(elements.heading.hidden, false);
      assert.equal(elements.heading.textContent, malicious);
      assert.equal(elements.heading.querySelector("img"), null);
      assert.equal(elements.channel.hidden, false);
      assert.equal(elements.channel.textContent, "A channel");
    });

    it(`hides the channel name the URL does not hold in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: "A video", url: watchUrl },
        locale,
      );
      assert.equal(elements.channel.hidden, true);
    });

    it(`links an accepted watch URL in a new tab without the opener or the referrer in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: "A video", url: watchUrl },
        locale,
      );
      const link = elements.videoLink;
      assert.equal(link.hidden, false);
      assert.equal(link.getAttribute("href"), watchUrl);
      assert.equal(link.getAttribute("target"), "_blank");
      assert.equal(link.getAttribute("rel"), "noopener noreferrer");
      assert.equal(link.textContent, catalogs[locale].viewer.videoLink);
    });

    for (const url of [
      "javascript:alert(1)",
      "https://www.example.com/watch?v=abc",
    ]) {
      it(`shows no link for ${url} in ${locale}`, () => {
        const elements = viewerElements();
        renderViewerHeader(elements, { heading: "A video", url }, locale);
        assert.equal(elements.videoLink.hidden, true);
        assert.equal(elements.videoLink.hasAttribute("href"), false);
      });
    }

    it(`hides the whole header without the URL's values in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: "A video", channelName: "A channel", url: watchUrl },
        locale,
      );
      renderViewerHeader(elements, undefined, locale);
      assert.equal(elements.heading.hidden, true);
      assert.equal(elements.channel.hidden, true);
      assert.equal(elements.videoLink.hidden, true);
      assert.equal(elements.videoLink.hasAttribute("href"), false);
    });
  }
});

describe("renderViewer states", () => {
  for (const locale of localeList) {
    const viewer = catalogs[locale].viewer;

    it(`shows that the link is not valid, and no article or operation, in ${locale}`, () => {
      const elements = drawn(view({ kind: "invalid-link" }), locale);
      expectLocaleMessage(
        elements.state,
        locale,
        (l) => catalogs[l].viewer.invalidLink,
      );
      assert.equal(elements.article.hidden, true);
      assert.equal(elements.actions.hidden, true);
    });

    for (const [kind, messageFor] of [
      ["sending", (l: Locale) => catalogs[l].viewer.sending],
      ["loading", (l: Locale) => catalogs[l].viewer.loading],
    ] as const) {
      it(`shows the loading mark with the ${kind} wait's sentence in ${locale}`, () => {
        const elements = drawn(view({ kind }), locale);
        const indicator = query<HTMLElement>(elements.state, ".loading");
        assert.ok(indicator.querySelector(".loading-icon") !== null);
        expectLocaleMessage(indicator, locale, messageFor);
        assert.equal(elements.actions.hidden, true);
      });
    }

    it(`replaces the loading mark with the first status in ${locale}`, () => {
      const elements = drawn(view({ kind: "loading" }), locale);
      renderViewer(elements, view(jobState(running)), locale);
      assert.equal(elements.state.querySelector(".loading"), null);
      assert.ok(elements.state.querySelector(".step-list") !== null);
    });

    it(`shows a start that gave no result without the loading mark in ${locale}`, () => {
      const failure = { kind: "no-result" } as const;
      const elements = drawn(view({ kind: "start-failed", failure }), locale);
      assert.equal(elements.state.querySelector(".loading"), null);
      expectLocaleMessage(elements.state, locale, (l) =>
        startResultMessage(failure, l),
      );
      assert.equal(elements.actions.hidden, true);
    });

    it(`shows a running job's steps without an article or operation in ${locale}`, () => {
      const elements = drawn(view(jobState(running), 3_000), locale);
      assert.ok(elements.state.querySelector(".step-list") !== null);
      assert.ok(elements.state.querySelector(".job-elapsed") !== null);
      assert.equal(elements.article.hidden, true);
      assert.equal(elements.actions.hidden, true);
    });

    it(`shows a done job's article as text and its operations in ${locale}`, () => {
      const elements = drawn(
        view(jobState(doneJob({ state: "not_posted" }))),
        locale,
      );
      assert.equal(elements.article.hidden, false);
      assert.equal(
        query(elements.article, ".article-body").textContent,
        article.markdown,
      );
      assert.equal(elements.actions.hidden, false);
      assert.equal(elements.save.disabled, false);
      assert.equal(elements.copy.disabled, false);
    });

    it(`keeps a lost job's article, saving and copying, but not publishing, in ${locale}`, () => {
      const elements = drawn(
        view(jobState({ kind: "lost", kept: article })),
        locale,
      );
      expectMessage(elements.state, viewer.lost);
      assert.equal(
        query(elements.article, ".article-body").textContent,
        article.markdown,
      );
      assert.equal(elements.save.disabled, false);
      assert.equal(elements.copy.disabled, false);
      assert.equal(elements.publish.disabled, true);
    });

    it(`shows a lost job without a kept article without an article or operation in ${locale}`, () => {
      const elements = drawn(
        view(jobState(doneJob({ state: "not_posted" }))),
        locale,
      );
      renderViewer(elements, view(jobState({ kind: "lost" })), locale);
      expectMessage(elements.state, viewer.lost);
      assert.equal(elements.article.hidden, true);
      assert.equal(elements.article.textContent, "");
      assert.equal(elements.actions.hidden, true);
      assert.equal(elements.save.disabled, true);
      assert.equal(elements.copy.disabled, true);
    });

    it(`shows a failed job's failure without an operation in ${locale}`, () => {
      const elements = drawn(
        view(
          jobState({
            kind: "failed",
            steps: runningSteps,
            failure: "internal",
          }),
        ),
        locale,
      );
      expectLocaleMessage(
        elements.state,
        locale,
        (l) => catalogs[l].failure.internal,
      );
      assert.equal(elements.actions.hidden, true);
    });
  }
});

describe("publish rows", () => {
  /** The rows of the requirements' table, top first, and two that overlap. */
  const rows: readonly {
    readonly name: string;
    readonly job: Extract<JobDisplay, { kind: "done" | "lost" }>;
    readonly publishing?: boolean;
    readonly enabled: boolean;
    readonly messages: (locale: Locale) => Message[];
  }[] = [
    {
      name: "lost",
      job: { kind: "lost", kept: article },
      enabled: false,
      messages: (l) => [{ text: catalogs[l].viewer.lost.text, steps: [] }],
    },
    {
      name: "posted",
      job: doneJob({ state: "posted" }),
      enabled: false,
      messages: (l) => [catalogs[l].publishStates.posted],
    },
    {
      name: "posted without a Webhook",
      job: doneJob({ state: "posted" }, false),
      enabled: false,
      messages: (l) => [catalogs[l].publishStates.posted],
    },
    {
      name: "posting",
      job: doneJob({ state: "posting" }),
      enabled: false,
      messages: (l) => [catalogs[l].publishStates.posting],
    },
    {
      name: "unpostable without a Webhook",
      job: doneJob({ state: "unpostable", reason: "mention" }, false),
      enabled: false,
      messages: (l) => [
        catalogs[l].publishStates.unpostable,
        catalogs[l].publishReasons.mention,
      ],
    },
    {
      name: "unpostable for another reason",
      job: doneJob({ state: "unpostable", reason: "other" }),
      enabled: false,
      messages: (l) => [
        catalogs[l].publishStates.unpostable,
        catalogs[l].publishReasons.other,
      ],
    },
    {
      name: "webhook not configured",
      job: doneJob({ state: "failed", reason: "webhook" }, false),
      enabled: false,
      messages: (l) => [catalogs[l].viewer.webhookNotConfigured],
    },
    {
      name: "failed",
      job: doneJob({ state: "failed", reason: "internal" }),
      enabled: true,
      messages: (l) => [
        catalogs[l].publishStates.failed,
        catalogs[l].publishReasons.internal,
      ],
    },
    {
      name: "failed at Slack",
      job: doneJob({ state: "failed", reason: "webhook" }),
      enabled: true,
      messages: (l) => [
        catalogs[l].publishStates.failed,
        catalogs[l].publishReasons.webhook,
      ],
    },
    {
      name: "not posted",
      job: doneJob({ state: "not_posted" }),
      enabled: true,
      messages: (l) => [catalogs[l].publishStates.not_posted],
    },
    {
      name: "not posted while publishing",
      job: doneJob({ state: "not_posted" }),
      publishing: true,
      enabled: false,
      messages: (l) => [catalogs[l].publishStates.not_posted],
    },
  ];

  for (const locale of localeList) {
    for (const row of rows) {
      it(`draws the ${row.name} row ${row.enabled ? "pressable" : "disabled"} with its wording in ${locale}`, () => {
        const elements = drawn(
          view(jobState(row.job), 0, row.publishing ?? false),
          locale,
        );
        assert.equal(elements.publish.disabled, !row.enabled);
        assert.equal(
          elements.publish.textContent,
          catalogs[locale].viewer.publish,
        );
        // Exactly the applying row's wording: a lower row's never joins it.
        assert.equal(
          elements.publishRow.textContent,
          row
            .messages(locale)
            .map((message) => message.text + message.steps.join(""))
            .join(""),
        );
      });
    }
  }

  it("disables the button again when a pressable row becomes a disabled one", () => {
    const elements = drawn(
      view(jobState(doneJob({ state: "not_posted" }))),
      "en",
    );
    assert.equal(elements.publish.disabled, false);
    renderViewer(elements, view(jobState(doneJob({ state: "posting" }))), "en");
    assert.equal(elements.publish.disabled, true);
  });
});

describe("status regions", () => {
  for (const locale of localeList) {
    it(`announces every step's name with its state while the job runs in ${locale}`, () => {
      const catalog = catalogs[locale];
      const elements = drawn(view(jobState(running)), locale);
      assert.deepEqual(
        Array.from(elements.stateStatus.children, (p) => p.textContent),
        [
          catalog.viewer.running,
          new Intl.ListFormat(locale, { type: "conjunction" }).format(
            (["draft", "shorten", "review"] as const).map(
              (name) =>
                `${catalog.stepNames[name]} ${catalog.stepStates[runningSteps[name]]}`,
            ),
          ),
        ],
      );
    });

    it(`writes the state summary only when it changes, not when the elapsed time does, in ${locale}`, () => {
      const elements = drawn(view(jobState(running), 1_000), locale);
      const written = elements.stateStatus.firstChild;
      assert.ok(written !== null);
      assert.ok(
        (elements.stateStatus.textContent ?? "").includes(
          catalogs[locale].viewer.running,
        ),
      );
      renderViewer(elements, view(jobState(running), 2_000), locale);
      assert.equal(elements.stateStatus.firstChild, written);

      renderViewer(
        elements,
        view(
          jobState({
            kind: "running",
            steps: { ...runningSteps, shorten: "done" },
          }),
          3_000,
        ),
        locale,
      );
      assert.notEqual(elements.stateStatus.firstChild, written);
    });

    it(`keeps the elapsed time out of both status regions in ${locale}`, () => {
      const elements = drawn(view(jobState(running), 65_000), locale);
      const elapsed = query(elements.state, ".job-elapsed");
      assert.equal(elapsed.closest("[role=status], [aria-live]"), null);
      for (const region of [elements.stateStatus, elements.actionStatus]) {
        assert.equal(region.querySelector(".job-elapsed"), null);
        assert.equal(
          (region.textContent ?? "").includes(
            elapsed.querySelector("dd")?.textContent ?? "",
          ),
          false,
        );
      }
    });

    for (const [name, state, text] of [
      [
        "sending",
        { kind: "sending" },
        (l: Locale) => catalogs[l].viewer.sending.text,
      ],
      [
        "loading",
        { kind: "loading" },
        (l: Locale) => catalogs[l].viewer.loading.text,
      ],
      [
        "done",
        jobState(doneJob({ state: "not_posted" })),
        (l: Locale) => catalogs[l].popup.articleReady.text,
      ],
      [
        "failed",
        jobState({ kind: "failed", steps: runningSteps, failure: "llm_empty" }),
        (l: Locale) => catalogs[l].failure.llm_empty.text,
      ],
      [
        "invalid link",
        { kind: "invalid-link" },
        (l: Locale) => catalogs[l].viewer.invalidLink.text,
      ],
      [
        "start without a result",
        { kind: "start-failed", failure: { kind: "no-result" } },
        (l: Locale) => catalogs[l].startResult.noResult.text,
      ],
      [
        "lost",
        jobState({ kind: "lost" }),
        (l: Locale) => catalogs[l].viewer.lost.text,
      ],
      [
        "status checks stopped",
        jobState(running, {
          kind: "stopped",
          outcome: { kind: "unexpected", check: "syntax" },
        }),
        (l: Locale) => catalogs[l].statusExchange.unexpected.text,
      ],
      [
        "status not read",
        jobState(running, {
          kind: "retrying",
          outcome: { kind: "no-response" },
        }),
        (l: Locale) => catalogs[l].statusExchange.noResponse.text,
      ],
    ] as const satisfies readonly (readonly [
      string,
      ViewerState,
      (l: Locale) => string,
    ])[]) {
      it(`announces ${name} in the state summary in ${locale}`, () => {
        const elements = drawn(view(state), locale);
        assert.ok(
          (elements.stateStatus.textContent ?? "").includes(text(locale)),
        );
      });
    }

    it(`writes an operation's result beside the operations and leaves the state summary in ${locale}`, () => {
      const elements = drawn(
        view(jobState(doneJob({ state: "not_posted" }))),
        locale,
      );
      const summary = elements.stateStatus.firstChild;
      renderActionNotice(elements, { kind: "copied" }, locale);
      assert.equal(
        elements.actionStatus.textContent,
        catalogs[locale].viewer.copied,
      );
      assert.equal(elements.stateStatus.firstChild, summary);
    });

    it(`keeps the focus where it is when the state changes in ${locale}`, () => {
      const elements = drawn(
        view(jobState(doneJob({ state: "not_posted" }))),
        locale,
      );
      elements.save.focus();
      renderViewer(
        elements,
        view(jobState(doneJob({ state: "posting" }))),
        locale,
      );
      renderActionNotice(elements, { kind: "saved" }, locale);
      assert.equal(elements.save.ownerDocument.activeElement, elements.save);
    });
  }
});

describe("publish state announcements", () => {
  for (const locale of localeList) {
    it(`announces each publish state as the post starts, finishes or fails in ${locale}`, () => {
      const catalog = catalogs[locale];
      const elements = viewerElements();
      for (const [publish, sentences] of [
        [{ state: "not_posted" }, [catalog.publishStates.not_posted.text]],
        [{ state: "posting" }, [catalog.publishStates.posting.text]],
        [{ state: "posted" }, [catalog.publishStates.posted.text]],
        [
          { state: "failed", reason: "webhook" },
          [
            catalog.publishStates.failed.text,
            catalog.publishReasons.webhook.text,
          ],
        ],
      ] as const) {
        renderViewer(elements, view(jobState(doneJob(publish))), locale);
        assert.deepEqual(
          Array.from(elements.stateStatus.children, (p) => p.textContent),
          [catalog.popup.articleReady.text, ...sentences],
          publish.state,
        );
        // A poll that repeats the state, even while a publish is in flight,
        // writes nothing.
        const written = elements.stateStatus.firstChild;
        renderViewer(
          elements,
          view(jobState(doneJob(publish)), 0, true),
          locale,
        );
        assert.equal(elements.stateStatus.firstChild, written, publish.state);
      }
    });
  }
});

describe("article redraw", () => {
  it("keeps the article element when its text is unchanged and replaces it when it changes", () => {
    const elements = drawn(
      view(jobState(doneJob({ state: "not_posted" }))),
      "en",
    );
    const body = query(elements.article, ".article-body");
    renderViewer(elements, view(jobState(doneJob({ state: "posting" }))), "en");
    assert.equal(query(elements.article, ".article-body"), body);
    renderViewer(
      elements,
      view(
        jobState({
          ...doneJob({ state: "posting" }),
          article: { ...article, markdown: "# Another" },
        }),
      ),
      "en",
    );
    const replaced = query(elements.article, ".article-body");
    assert.notEqual(replaced, body);
    assert.equal(replaced.textContent, "# Another");
  });

  it("clears the publish row when the article is no longer shown", () => {
    const elements = drawn(view(jobState(doneJob({ state: "posted" }))), "en");
    renderViewer(elements, view({ kind: "loading" }), "en");
    assert.equal(elements.publishRow.textContent, "");
  });
});

describe("renderActionNotice", () => {
  const notices: readonly {
    readonly notice: ActionNotice;
    readonly message: (locale: Locale) => Message;
  }[] = [
    {
      notice: { kind: "rejected", reason: "article_not_ready", requestId },
      message: (l) =>
        publishResultMessage(
          { kind: "rejected", reason: "article_not_ready", requestId },
          l,
        ),
    },
    {
      notice: { kind: "not-sent", cause: "server-unavailable" },
      message: (l) =>
        publishResultMessage(
          { kind: "not-sent", cause: "server-unavailable" },
          l,
        ),
    },
    {
      notice: { kind: "unknown", outcome: { kind: "no-response" } },
      message: (l) => publishResultMessage({ kind: "no-response" }, l),
    },
    {
      notice: { kind: "unknown", outcome: { kind: "host-failed" } },
      message: (l) => publishResultMessage({ kind: "host-failed" }, l),
    },
    {
      notice: { kind: "saved" },
      message: (l) => ({ text: catalogs[l].viewer.saved, steps: [] }),
    },
    {
      notice: { kind: "copy-failed" },
      message: (l) => ({ text: catalogs[l].viewer.copyFailed, steps: [] }),
    },
  ];

  for (const locale of localeList) {
    for (const { notice, message } of notices) {
      it(`shows the ${notice.kind}${"outcome" in notice ? ` ${notice.outcome.kind}` : ""} result in ${locale}`, () => {
        const elements = viewerElements();
        renderActionNotice(elements, notice, locale);
        expectLocaleMessage(elements.actionStatus, locale, message);
        assert.equal(
          elements.actionStatus.querySelector(".send-request-id")?.textContent,
          notice.kind === "rejected" ? requestId : undefined,
        );
      });
    }

    it(`clears the result for none in ${locale}`, () => {
      const elements = viewerElements();
      renderActionNotice(elements, { kind: "copied" }, locale);
      renderActionNotice(elements, { kind: "none" }, locale);
      assert.equal(elements.actionStatus.textContent, "");
    });
  }
});

describe("order and controls", () => {
  for (const locale of localeList) {
    const viewer = catalogs[locale].viewer;

    it(`puts the link, then publish, save and copy, in display order without tabindex in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: "A video", url: watchUrl },
        locale,
      );
      renderViewer(
        elements,
        view(jobState(doneJob({ state: "not_posted" }))),
        locale,
      );
      assert.deepEqual(shownControls(elements), [
        elements.videoLink,
        elements.publish,
        elements.save,
        elements.copy,
      ]);
      assert.equal(
        elements.state.ownerDocument.querySelector("[tabindex]"),
        null,
      );
    });

    it(`names each native button by its visible label in ${locale}`, () => {
      const elements = drawn(
        view(jobState(doneJob({ state: "posted" }))),
        locale,
      );
      for (const [button, label] of [
        [elements.publish, viewer.publish],
        [elements.save, viewer.save],
        [elements.copy, viewer.copy],
      ] as const) {
        assert.equal(button.tagName.toLowerCase(), "button");
        assert.equal(button.type, "button");
        assert.equal(button.textContent, label);
        assert.equal(button.hasAttribute("aria-label"), false);
      }
      // A button that cannot be pressed says so to assistive technology.
      assert.equal(elements.publish.hasAttribute("disabled"), true);
    });
  }
});
