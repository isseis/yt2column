import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import type { Message } from "../src/core/i18n/catalog.ts";
import { formatDuration, locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import type { JobDisplay, ViewerState } from "../src/core/jobView.ts";
import { startFailureMessage } from "../src/core/sendMessages.ts";
import {
  renderActionNotice,
  renderViewer,
  renderViewerHeader,
} from "../src/ui/renderViewer.ts";
import type {
  ActionNotice,
  ViewerControls,
  ViewerElements,
  ViewerView,
} from "../src/ui/renderViewer.ts";
import {
  expectLocaleMessage,
  expectMessage,
  query,
  viewerElements,
} from "./helpers/dom.ts";
import {
  article,
  doneJob,
  failedSteps,
  runningSteps,
} from "./helpers/record.ts";

const localeList = Object.keys(locales) as Locale[];
const malicious = "<img src=x onerror=alert(1)>";
const watchUrl = "https://www.youtube.com/watch?v=abc";
const running: JobDisplay = { kind: "running", steps: runningSteps };
const loading: ViewerState = { kind: "loading" };
const noControls: ViewerControls = { openOptions: () => {} };

/** A view of state in a release build, with the elapsed time and the existing-article mark. */
function view(state: ViewerState, elapsedMs = 0, existing = false): ViewerView {
  return { state, elapsedMs, existing, buildKind: "release" };
}

/** A job state. */
function jobState(job: JobDisplay): ViewerState {
  return { kind: "job", job };
}

/** Fresh skeleton elements with the view drawn into them. */
function drawn(
  viewerView: ViewerView,
  locale: Locale,
  controls: ViewerControls = noControls,
): ViewerElements {
  const elements = viewerElements();
  renderViewer(elements, viewerView, locale, controls);
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
    it(`shows the header's heading and channel name as text in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: malicious, channelName: "A channel", videoUrl: watchUrl },
        locale,
        undefined,
      );
      assert.equal(elements.heading.hidden, false);
      assert.equal(elements.heading.textContent, malicious);
      assert.equal(elements.heading.querySelector("img"), null);
      assert.equal(elements.channel.hidden, false);
      assert.equal(elements.channel.textContent, "A channel");
    });

    it(`hides the channel name the header does not hold in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: "A video", videoUrl: watchUrl },
        locale,
        undefined,
      );
      assert.equal(elements.channel.hidden, true);
    });

    it(`links an accepted watch URL in a new tab without the opener or the referrer in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: "A video", videoUrl: watchUrl },
        locale,
        undefined,
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
        renderViewerHeader(
          elements,
          { heading: "A video", videoUrl: url },
          locale,
          undefined,
        );
        assert.equal(elements.videoLink.hidden, true);
        assert.equal(elements.videoLink.hasAttribute("href"), false);
      });
    }

    it(`hides the whole header without its values in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: "A video", channelName: "A channel", videoUrl: watchUrl },
        locale,
        undefined,
      );
      renderViewerHeader(elements, undefined, locale, undefined);
      assert.equal(elements.heading.hidden, true);
      assert.equal(elements.channel.hidden, true);
      assert.equal(elements.videoLink.hidden, true);
      assert.equal(elements.videoLink.hasAttribute("href"), false);
    });
    it(`shows the done article's neutralized heading and channel name in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: "Live @here", channelName: "@handle", videoUrl: watchUrl },
        locale,
        {
          ...article,
          videoTitle: "Live @\u200bhere",
          channelName: "@\u200bhandle",
        },
      );
      assert.equal(elements.heading.textContent, "Live @\u200bhere");
      assert.equal(elements.channel.hidden, false);
      assert.equal(elements.channel.textContent, "@\u200bhandle");
    });

    it(`keeps the header's heading for an empty video title and hides an empty channel name in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        {
          heading: "A tab title",
          channelName: "A channel",
          videoUrl: watchUrl,
        },
        locale,
        { ...article, videoTitle: "", channelName: "" },
      );
      assert.equal(elements.heading.hidden, false);
      assert.equal(elements.heading.textContent, "A tab title");
      assert.equal(elements.channel.hidden, true);
      assert.equal(elements.channel.textContent, "");
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

    for (const [kind, state, messageFor] of [
      [
        "preparing",
        { kind: "preparing" },
        (l: Locale) => catalogs[l].viewer.preparing,
      ],
      ["loading", loading, (l: Locale) => catalogs[l].viewer.loading],
    ] as const) {
      it(`shows the loading mark with the ${kind} wait's sentence in ${locale}`, () => {
        const elements = drawn(view(state), locale);
        const indicator = query<HTMLElement>(elements.state, ".loading");
        assert.ok(indicator.querySelector(".loading-icon") !== null);
        expectLocaleMessage(indicator, locale, messageFor);
        assert.equal(elements.actions.hidden, true);
      });
    }

    it(`replaces the loading mark with the record's state in ${locale}`, () => {
      const elements = drawn(view(loading), locale);
      renderViewer(elements, view(jobState(running)), locale, noControls);
      assert.equal(elements.state.querySelector(".loading"), null);
      assert.ok(elements.state.querySelector(".step-list") !== null);
    });

    for (const failure of [
      { kind: "unknown" },
      { kind: "input-unavailable" },
      { kind: "rejected", reason: "invalid_body" },
    ] as const) {
      it(`shows the start failure ${JSON.stringify(failure)} without the loading mark or a setup operation in ${locale}`, () => {
        const elements = drawn(view({ kind: "start-failed", failure }), locale);
        assert.equal(elements.state.querySelector(".loading"), null);
        expectLocaleMessage(elements.state, locale, (l) =>
          startFailureMessage(failure, l),
        );
        assert.equal(elements.state.querySelector("button"), null);
        assert.equal(elements.actions.hidden, true);
      });
    }

    it(`shows a running job's steps without an article or operation in ${locale}`, () => {
      const elements = drawn(view(jobState(running), 3_000), locale);
      assert.ok(elements.state.querySelector(".step-list") !== null);
      assert.ok(elements.state.querySelector(".job-elapsed") !== null);
      assert.equal(elements.article.hidden, true);
      assert.equal(elements.actions.hidden, true);
    });

    it(`shows a done job's article drawn from its Markdown and its operations in ${locale}`, () => {
      const elements = drawn(view(jobState(doneJob)), locale);
      assert.equal(elements.article.hidden, false);
      // The fixture's "# Title\n\nBody", without its heading mark.
      assert.equal(
        query(elements.article, ".article-body").textContent,
        "TitleBody",
      );
      assert.equal(elements.actions.hidden, false);
      assert.equal(elements.save.disabled, false);
      assert.equal(elements.copy.disabled, false);
    });

    it(`shows an article its record could not hold, with saving and copying and the warning, in ${locale}`, () => {
      const elements = drawn(
        view(jobState({ kind: "unstored", steps: doneJob.steps, article })),
        locale,
      );
      expectMessage(elements.state, viewer.unstored);
      assert.equal(
        query(elements.article, ".article-body").textContent,
        "TitleBody",
      );
      assert.equal(elements.save.disabled, false);
      assert.equal(elements.copy.disabled, false);
    });

    it(`shows a lost job without an article or operation in ${locale}`, () => {
      const elements = drawn(view(jobState(doneJob)), locale);
      renderViewer(
        elements,
        view(jobState({ kind: "lost" })),
        locale,
        noControls,
      );
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
            steps: failedSteps,
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
      assert.equal(elements.state.querySelector("button"), null);
      assert.equal(elements.actions.hidden, true);
    });

    // The states that need a setting offer the operation that opens the
    // options page, inside the state section, after the message.
    const setupStates: readonly [
      string,
      ViewerState,
      (l: Locale) => Message,
    ][] = [
      [
        "a start refused for a missing API key",
        {
          kind: "start-failed",
          failure: { kind: "rejected", reason: "api_key_missing" },
        },
        (l) => catalogs[l].setup.api_key_missing,
      ],
      [
        "a run without an API key",
        jobState({ kind: "setup", failure: "api_key_missing" }),
        (l) => catalogs[l].setup.api_key_missing,
      ],
      [
        "a run with unusable settings",
        jobState({ kind: "setup", failure: "settings_invalid" }),
        (l) => catalogs[l].setup.settings_invalid,
      ],
      [
        "a run whose key was rejected",
        jobState({ kind: "key-rejected", steps: failedSteps }),
        (l) => catalogs[l].keyRejected,
      ],
    ];
    for (const [name, state, message] of setupStates) {
      it(`offers the setup operation after the message of ${name} in ${locale}`, () => {
        let opened = 0;
        const elements = drawn(view(state), locale, {
          openOptions: () => {
            opened += 1;
          },
        });
        expectLocaleMessage(elements.state, locale, message);
        const button = query<HTMLButtonElement>(elements.state, "button");
        assert.equal(button.type, "button");
        assert.equal(button.textContent, catalogs[locale].openOptions.setup);
        assert.equal(elements.state.lastElementChild, button);
        button.click();
        assert.equal(opened, 1);
      });
    }

    it(`shows the development build's mark first in the state section, as text, in ${locale}`, () => {
      const elements = drawn(
        { ...view(jobState(running)), buildKind: "dev" },
        locale,
      );
      const mark = elements.state.firstElementChild;
      assert.ok(mark !== null && mark.classList.contains("build-mark"));
      assert.equal(mark.textContent, catalogs[locale].buildMark);
      const release = drawn(view(jobState(running)), locale);
      assert.equal(release.state.querySelector(".build-mark"), null);
    });
  }
});

describe("existing-article notice", () => {
  for (const locale of localeList) {
    it(`shows the notice and its steps, and announces its sentence first, in ${locale}`, () => {
      const elements = drawn(view(jobState(running), 0, true), locale);
      assert.equal(elements.existing.hidden, false);
      expectLocaleMessage(
        elements.existing,
        locale,
        (l) => catalogs[l].viewer.existing,
      );
      assert.equal(
        elements.stateStatus.firstElementChild?.textContent,
        catalogs[locale].viewer.existing.text,
      );
    });

    it(`hides the notice and keeps it out of the summary without the mark in ${locale}`, () => {
      const elements = drawn(view(jobState(running)), locale);
      assert.equal(elements.existing.hidden, true);
      assert.equal(elements.existing.textContent, "");
      assert.ok(
        !(elements.stateStatus.textContent ?? "").includes(
          catalogs[locale].viewer.existing.text,
        ),
      );
    });

    it(`draws a running job without its elapsed time once the mark is added in ${locale}`, () => {
      const elements = drawn(view(jobState(running), 65_000), locale);
      assert.ok(elements.state.querySelector(".job-elapsed") !== null);
      renderViewer(
        elements,
        view(jobState(running), 66_000, true),
        locale,
        noControls,
      );
      assert.equal(elements.state.querySelector(".job-elapsed"), null);
      assert.equal(elements.state.querySelectorAll(".step-row").length, 3);
    });
  }

  it("keeps the notice's nodes when it is drawn again unchanged", () => {
    const elements = drawn(view(jobState(running), 0, true), "en");
    const text = elements.existing.firstChild;
    renderViewer(elements, view(loading, 0, true), "en", noControls);
    assert.equal(elements.existing.firstChild, text);
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
      renderViewer(
        elements,
        view(jobState(running), 2_000),
        locale,
        noControls,
      );
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
        noControls,
      );
      assert.notEqual(elements.stateStatus.firstChild, written);
    });

    it(`keeps the state's nodes and writes only the elapsed time when nothing else changes in ${locale}`, () => {
      const elements = drawn(view(jobState(running), 1_000), locale);
      const list = query(elements.state, ".step-list");
      renderViewer(
        elements,
        view(jobState(running), 2_000),
        locale,
        noControls,
      );
      assert.equal(query(elements.state, ".step-list"), list);
      assert.equal(
        query(elements.state, ".job-elapsed dd").textContent,
        formatDuration(2_000, locale),
      );

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
        noControls,
      );
      assert.notEqual(query(elements.state, ".step-list"), list);
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
        "preparing",
        { kind: "preparing" },
        (l: Locale) => catalogs[l].viewer.preparing.text,
      ],
      ["loading", loading, (l: Locale) => catalogs[l].viewer.loading.text],
      [
        "done",
        jobState(doneJob),
        (l: Locale) => catalogs[l].popup.articleReady.text,
      ],
      [
        "failed",
        jobState({ kind: "failed", steps: failedSteps, failure: "llm_empty" }),
        (l: Locale) => catalogs[l].failure.llm_empty.text,
      ],
      [
        "an unknown cost",
        jobState({ kind: "stopped", steps: failedSteps }),
        (l: Locale) =>
          `${catalogs[l].viewer.cost} ${catalogs[l].viewer.costUnknown}`,
      ],
      [
        "invalid link",
        { kind: "invalid-link" },
        (l: Locale) => catalogs[l].viewer.invalidLink.text,
      ],
      [
        "an unknown start",
        { kind: "start-failed", failure: { kind: "unknown" } },
        (l: Locale) => catalogs[l].startUnknown.text,
      ],
      [
        "lost",
        jobState({ kind: "lost" }),
        (l: Locale) => catalogs[l].viewer.lost.text,
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
      const elements = drawn(view(jobState(doneJob)), locale);
      const summary = elements.stateStatus.firstChild;
      renderActionNotice(
        elements,
        { kind: "copied", mentionsNeutralized: false },
        locale,
      );
      assert.equal(
        elements.actionStatus.textContent,
        catalogs[locale].viewer.copied,
      );
      assert.equal(elements.stateStatus.firstChild, summary);
    });

    it(`keeps the focus where it is when the state changes in ${locale}`, () => {
      const elements = drawn(view(jobState(doneJob)), locale);
      elements.save.focus();
      renderViewer(
        elements,
        view(jobState({ kind: "unstored", steps: doneJob.steps, article })),
        locale,
        noControls,
      );
      renderActionNotice(elements, { kind: "saved" }, locale);
      assert.equal(elements.save.ownerDocument.activeElement, elements.save);
    });
  }
});

describe("article redraw", () => {
  /** A done job's view whose article has markdown. */
  const withMarkdown = (markdown: string): ViewerView =>
    view(
      jobState({
        ...doneJob,
        article: { ...article, markdown },
      }),
    );

  it("keeps the drawn article for the same Markdown and draws it again when the Markdown changes", () => {
    const elements = drawn(withMarkdown("# Title\n\nBody"), "en");
    const body = query(elements.article, ".article-body");
    renderViewer(elements, withMarkdown("# Title\n\nBody"), "en", noControls);
    assert.equal(query(elements.article, ".article-body"), body);
    renderViewer(elements, withMarkdown("# Another"), "en", noControls);
    const replaced = query(elements.article, ".article-body");
    assert.notEqual(replaced, body);
    assert.equal(replaced.textContent, "Another");
  });

  it("draws the same article again after it was hidden", () => {
    const elements = drawn(withMarkdown("# Title"), "en");
    const body = query(elements.article, ".article-body");
    renderViewer(elements, view(loading), "en", noControls);
    assert.equal(elements.article.childElementCount, 0);
    renderViewer(elements, withMarkdown("# Title"), "en", noControls);
    const again = query(elements.article, ".article-body");
    assert.notEqual(again, body);
    assert.equal(again.textContent, "Title");
  });

  it("keeps the focus on a link and a selection in the article when the same Markdown comes again", () => {
    const markdown = "Read <https://a.example/x> and more";
    const elements = drawn(withMarkdown(markdown), "en");
    const doc = elements.article.ownerDocument;
    const link = query<HTMLElement>(elements.article, "a");
    link.focus();
    assert.equal(doc.activeElement, link);
    const selection = doc.getSelection();
    assert.ok(selection !== null);
    const textNode = query(elements.article, "p").firstChild;
    assert.ok(textNode !== null);
    selection.setBaseAndExtent(textNode, 0, textNode, 4);
    renderViewer(elements, withMarkdown(markdown), "en", noControls);
    assert.equal(doc.activeElement, link);
    assert.equal(selection.anchorNode, textNode);
    assert.equal(selection.toString(), "Read");
  });

  it("shows save and copy, pressable, before it draws the article", () => {
    const elements = viewerElements();
    const replace = elements.article.replaceChildren.bind(elements.article);
    let drawnWithArticle = false;
    // Only this element's method is replaced: it checks the buttons at the
    // moment the article's children are replaced with the drawn article.
    elements.article.replaceChildren = (...nodes: (Node | string)[]) => {
      if (nodes.length > 0) {
        assert.equal(elements.actions.hidden, false);
        assert.equal(elements.save.disabled, false);
        assert.equal(elements.copy.disabled, false);
        drawnWithArticle = true;
      }
      replace(...nodes);
    };
    renderViewer(elements, withMarkdown("# Title"), "en", noControls);
    assert.ok(drawnWithArticle);
  });
});

describe("article headings", () => {
  it("puts the article's title and section headings under the video's h1, in document order", () => {
    const elements = drawn(
      view(
        jobState({
          ...doneJob,
          article: {
            ...article,
            markdown: "# Title\n\nBody\n\n## One\n\nText\n\n## Two",
          },
        }),
      ),
      "en",
    );
    const doc = elements.article.ownerDocument;
    assert.deepEqual(
      Array.from(doc.querySelectorAll("h1, h2, h3, h4, h5, h6"), (heading) => [
        heading.tagName.toLowerCase(),
        heading.id === "heading" ? "#heading" : heading.textContent,
      ]),
      [
        ["h1", "#heading"],
        ["h2", "Title"],
        ["h3", "One"],
        ["h3", "Two"],
      ],
    );
  });
});

describe("renderActionNotice", () => {
  const notices: readonly {
    readonly notice: ActionNotice;
    readonly message: (locale: Locale) => Message;
  }[] = [
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
      it(`shows the ${notice.kind} result in ${locale}`, () => {
        const elements = viewerElements();
        renderActionNotice(elements, notice, locale);
        expectLocaleMessage(elements.actionStatus, locale, message);
      });
    }

    it(`clears the result for none in ${locale}`, () => {
      const elements = viewerElements();
      renderActionNotice(
        elements,
        { kind: "copied", mentionsNeutralized: false },
        locale,
      );
      renderActionNotice(elements, { kind: "none" }, locale);
      assert.equal(elements.actionStatus.textContent, "");
    });
  }
});

describe("order and controls", () => {
  for (const locale of localeList) {
    const viewer = catalogs[locale].viewer;

    it(`puts the link, then save and copy, in display order without tabindex in ${locale}`, () => {
      const elements = viewerElements();
      renderViewerHeader(
        elements,
        { heading: "A video", videoUrl: watchUrl },
        locale,
        undefined,
      );
      renderViewer(elements, view(jobState(doneJob)), locale, noControls);
      // Compared by id: deepEqual finds two of jsdom's elements equal
      // whichever elements they are.
      assert.deepEqual(
        shownControls(elements).map((control) => control.id),
        [elements.videoLink.id, elements.save.id, elements.copy.id],
      );
      assert.equal(
        elements.state.ownerDocument.querySelector("[tabindex]"),
        null,
      );
    });

    it(`names each native button by its visible label in ${locale}`, () => {
      const elements = drawn(view(jobState(doneJob)), locale);
      for (const [button, label] of [
        [elements.save, viewer.save],
        [elements.copy, viewer.copy],
      ] as const) {
        assert.equal(button.tagName.toLowerCase(), "button");
        assert.equal(button.type, "button");
        assert.equal(button.textContent, label);
        assert.equal(button.hasAttribute("aria-label"), false);
      }
    });
  }
});
