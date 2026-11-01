import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { formatDuration, formatUsd, locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import type {
  JobDisplay,
  PollNotice,
  StartFailure,
} from "../src/core/jobView.ts";
import { stepStates } from "../src/core/response.ts";
import type { RequestId, StepState, Steps } from "../src/core/response.ts";
import {
  startResultMessage,
  statusNoticeMessage,
} from "../src/core/sendMessages.ts";
import {
  loadingIndicator,
  renderJobState,
  startFailure,
} from "../src/ui/renderJob.ts";
import { container, expectLocaleMessage, query } from "./helpers/dom.ts";
import { article, doneJob, requestId, runningSteps } from "./helpers/job.ts";

const localeList = Object.keys(locales) as Locale[];
const stateList = Object.keys(stepStates) as StepState[];
const malicious = "<img src=x onerror=alert(1)>";
const none: PollNotice = { kind: "none" };

/** Every step in the same state. */
function allSteps(state: StepState): Steps {
  return { draft: state, shorten: state, review: state };
}

/** The symbol's shapes with their attributes, so two symbols compare by drawing. */
function drawing(icon: Element): string {
  return Array.from(icon.children, (shape) => {
    const attributes = Array.from(
      shape.attributes,
      (attribute) => `${attribute.name}=${attribute.value}`,
    ).sort();
    return `${shape.tagName.toLowerCase()}(${attributes.join(",")})`;
  }).join(" ");
}

/** The step rows drawn into root. */
function stepRows(root: HTMLElement): HTMLLIElement[] {
  return Array.from(root.querySelectorAll<HTMLLIElement>("ol.step-list > li"));
}

/** The text of every element that announces its changes. */
function liveRegions(root: HTMLElement): Element[] {
  return Array.from(
    root.querySelectorAll("[aria-live], [role=status], [role=alert]"),
  );
}

describe("step checklist", () => {
  for (const locale of localeList) {
    it(`gives every step state its own symbol shape and its own word in ${locale}`, () => {
      const drawings = new Set<string>();
      const words = new Set<string>();
      for (const state of stateList) {
        const root = container();
        renderJobState(
          root,
          { kind: "failed", steps: allSteps(state), failure: "internal" },
          none,
          0,
          locale,
        );
        const [row] = stepRows(root);
        assert.ok(row !== undefined);
        const icon = query<SVGSVGElement>(row, "svg");
        assert.ok(icon.classList.contains(`step-icon-${state}`), state);
        drawings.add(drawing(icon));
        words.add(query(row, ".step-state").textContent ?? "");
      }
      assert.equal(drawings.size, stateList.length);
      assert.equal(words.size, stateList.length);
    });

    it(`lists the steps in the server's order with each step's own state in ${locale}`, () => {
      const catalog = catalogs[locale];
      const root = container();
      renderJobState(
        root,
        { kind: "running", steps: runningSteps },
        none,
        0,
        locale,
      );
      assert.deepEqual(
        stepRows(root).map((row) => [
          query(row, ".step-name").textContent,
          query(row, ".step-state").textContent,
        ]),
        [
          [catalog.stepNames.draft, catalog.stepStates[runningSteps.draft]],
          [catalog.stepNames.shorten, catalog.stepStates[runningSteps.shorten]],
          [catalog.stepNames.review, catalog.stepStates[runningSteps.review]],
        ],
      );
    });

    it(`orders each row as the hidden symbol, the name and the visually hidden state word in ${locale}`, () => {
      const root = container();
      renderJobState(
        root,
        { kind: "running", steps: runningSteps },
        none,
        0,
        locale,
      );
      for (const row of stepRows(root)) {
        const [icon, name, word] = Array.from(row.children);
        assert.equal(icon?.tagName.toLowerCase(), "svg");
        assert.equal(icon?.getAttribute("aria-hidden"), "true");
        assert.equal(icon?.textContent, "");
        assert.ok(name?.classList.contains("step-name"));
        assert.ok(word?.classList.contains("visually-hidden"));
      }
    });
  }
});

describe("job state", () => {
  for (const locale of localeList) {
    const viewer = catalogs[locale].viewer;

    it(`shows the elapsed time of a running job outside any live region in ${locale}`, () => {
      const root = container();
      renderJobState(
        root,
        { kind: "running", steps: runningSteps },
        none,
        65_000,
        locale,
      );
      const elapsed = query(root, ".job-elapsed");
      assert.equal(query(elapsed, "dt").textContent, viewer.elapsed);
      assert.equal(
        query(elapsed, "dd").textContent,
        formatDuration(65_000, locale),
      );
      assert.deepEqual(liveRegions(root), []);
    });

    it(`shows a done job's time taken and cost instead of the elapsed time in ${locale}`, () => {
      const root = container();
      renderJobState(
        root,
        doneJob({ state: "not_posted" }),
        none,
        5_000,
        locale,
      );
      assert.equal(root.querySelector(".job-elapsed"), null);
      assert.ok(article.cost.state === "known");
      assert.deepEqual(
        Array.from(root.querySelectorAll(".job-result dd"), (dd) => [
          dd.previousElementSibling?.textContent,
          dd.textContent,
        ]),
        [
          [viewer.duration, formatDuration(article.durationMs, locale)],
          [viewer.cost, formatUsd(article.cost.usd, locale)],
        ],
      );
    });

    it(`shows the unknown cost's wording in ${locale}`, () => {
      const root = container();
      const job: JobDisplay = {
        ...doneJob({ state: "not_posted" }),
        article: { ...article, cost: { state: "unknown" } },
      };
      renderJobState(root, job, none, 0, locale);
      assert.equal(
        query(root, ".job-result dd:last-of-type").textContent,
        viewer.costUnknown,
      );
    });

    it(`shows a failed job's steps and its failure's message in ${locale}`, () => {
      const root = container();
      renderJobState(
        root,
        { kind: "failed", steps: runningSteps, failure: "llm_timeout" },
        none,
        0,
        locale,
      );
      assert.equal(stepRows(root).length, 3);
      expectLocaleMessage(
        query(root, ".failure"),
        locale,
        (l) => catalogs[l].failure.llm_timeout,
      );
    });

    it(`shows the lost article's message and no steps in ${locale}`, () => {
      const root = container();
      renderJobState(root, { kind: "lost" }, none, 0, locale);
      assert.deepEqual(stepRows(root), []);
      expectLocaleMessage(
        query(root, ".failure"),
        locale,
        (l) => catalogs[l].viewer.lost,
      );
    });
  }
});

describe("poll notice", () => {
  const running: JobDisplay = { kind: "running", steps: runningSteps };

  for (const locale of localeList) {
    it(`shows a retrying notice's message without a request id in ${locale}`, () => {
      const root = container();
      const notice: PollNotice = {
        kind: "retrying",
        outcome: { kind: "no-response" },
      };
      renderJobState(root, running, notice, 0, locale);
      expectLocaleMessage(query(root, ".job-notice"), locale, (l) =>
        statusNoticeMessage(notice.outcome, l),
      );
      assert.equal(root.querySelector(".send-request-id"), null);
    });

    it(`shows a stopped rejection's message and its request id in ${locale}`, () => {
      const root = container();
      const notice: PollNotice = {
        kind: "stopped",
        outcome: { kind: "rejected", reason: "invalid_key", requestId },
      };
      renderJobState(root, running, notice, 0, locale);
      expectLocaleMessage(query(root, ".job-notice"), locale, (l) =>
        statusNoticeMessage(notice.outcome, l),
      );
      assert.equal(query(root, ".send-request-id").textContent, requestId);
    });

    it(`shows an unexpected reply's message but not the check that failed in ${locale}`, () => {
      const root = container();
      const notice: PollNotice = {
        kind: "stopped",
        outcome: { kind: "unexpected", check: "member-set" },
      };
      renderJobState(root, running, notice, 0, locale);
      expectLocaleMessage(query(root, ".job-notice"), locale, (l) =>
        statusNoticeMessage(notice.outcome, l),
      );
      assert.equal((root.textContent ?? "").includes("member-set"), false);
    });

    it(`displays a request id with markup as text, not elements, in ${locale}`, () => {
      // A RequestId is validated before it reaches here; the cast checks the
      // rendering alone.
      const root = container();
      renderJobState(
        root,
        running,
        {
          kind: "stopped",
          outcome: {
            kind: "rejected",
            reason: "internal_error",
            requestId: malicious as RequestId,
          },
        },
        0,
        locale,
      );
      assert.equal(root.querySelector("img"), null);
      assert.equal(query(root, ".send-request-id").textContent, malicious);
    });

    it(`replaces the previous content on each call in ${locale}`, () => {
      const root = container();
      renderJobState(
        root,
        running,
        {
          kind: "stopped",
          outcome: { kind: "rejected", reason: "invalid_key", requestId },
        },
        0,
        locale,
      );
      renderJobState(root, running, none, 0, locale);
      assert.equal(root.querySelector(".job-notice"), null);
      assert.equal(root.querySelector(".send-request-id"), null);
      assert.equal(root.querySelectorAll("ol.step-list").length, 1);
    });
  }
});

describe("start failure", () => {
  const doc = container().ownerDocument;

  for (const locale of localeList) {
    it(`shows a rejected start's message and its request id in ${locale}`, () => {
      const failure: StartFailure = {
        kind: "rejected",
        reason: "too_many_requests",
        requestId,
      };
      const block = startFailure(doc, failure, locale);
      expectLocaleMessage(block, locale, (l) => startResultMessage(failure, l));
      assert.equal(query(block, ".send-request-id").textContent, requestId);
    });

    it(`shows a wait without a result as its message and no request id in ${locale}`, () => {
      const failure: StartFailure = { kind: "no-result" };
      const block = startFailure(doc, failure, locale);
      expectLocaleMessage(block, locale, (l) => startResultMessage(failure, l));
      assert.equal(block.querySelector(".send-request-id"), null);
    });

    it(`shows an unexpected start reply's message but not its check in ${locale}`, () => {
      const failure: StartFailure = { kind: "unexpected", check: "syntax" };
      const block = startFailure(doc, failure, locale);
      expectLocaleMessage(block, locale, (l) => startResultMessage(failure, l));
      assert.equal((block.textContent ?? "").includes("syntax"), false);
    });
  }
});

describe("loading indicator", () => {
  for (const locale of localeList) {
    it(`puts the hidden loading mark before the wait's sentence in ${locale}`, () => {
      const doc = container().ownerDocument;
      const indicator = loadingIndicator(doc, catalogs[locale].viewer.loading);
      const [icon] = Array.from(indicator.children);
      assert.ok(icon?.classList.contains("loading-icon"));
      assert.equal(icon?.getAttribute("aria-hidden"), "true");
      expectLocaleMessage(indicator, locale, (l) => catalogs[l].viewer.loading);
    });
  }
});
