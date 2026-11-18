import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import type { Message } from "../src/core/i18n/catalog.ts";
import { formatDuration, formatUsd, locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import type {
  JobDisplay,
  ShownStepState,
  StartFailure,
} from "../src/core/jobView.ts";
import { startFailureMessage } from "../src/core/sendMessages.ts";
import type { StepState, Steps } from "../src/core/wasmResult.ts";
import {
  jobStateSummary,
  loadingIndicator,
  renderJobState,
  startFailure,
} from "../src/ui/renderJob.ts";
import type { ElapsedTime } from "../src/ui/renderJob.ts";
import { container, expectLocaleMessage, query } from "./helpers/dom.ts";
import {
  article,
  doneJob,
  failedSteps,
  runningSteps,
} from "./helpers/record.ts";

const localeList = Object.keys(locales) as Locale[];
/**
 * Every shown step state with a job that shows it: running only while the job
 * runs, and interrupted for a step an ended job's record still holds as
 * running.
 */
const shownStates: readonly (readonly [ShownStepState, JobDisplay])[] = [
  ["pending", failedJob(allSteps("pending"))],
  ["running", { kind: "running", steps: allSteps("running") }],
  ["done", failedJob(allSteps("done"))],
  ["skipped", failedJob(allSteps("skipped"))],
  ["failed", failedJob(allSteps("failed"))],
  ["interrupted", failedJob(allSteps("running"))],
];

/** A job that failed outside the module after steps. */
function failedJob(steps: Steps): JobDisplay {
  return { kind: "failed", steps, failure: "internal" };
}

/** The elapsed time ms, drawn while a job runs. */
function shown(ms: number): ElapsedTime {
  return { kind: "shown", ms };
}

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
      for (const [state, job] of shownStates) {
        const root = container();
        renderJobState(root, job, shown(0), locale);
        const [row] = stepRows(root);
        assert.ok(row !== undefined);
        const icon = query<SVGSVGElement>(row, "svg");
        assert.ok(icon.classList.contains(`step-icon-${state}`), state);
        drawings.add(drawing(icon));
        words.add(query(row, ".step-state").textContent ?? "");
      }
      assert.equal(drawings.size, shownStates.length);
      assert.equal(words.size, shownStates.length);
    });

    it(`shows a step an ended job still holds as running as interrupted, not as running, in ${locale}`, () => {
      const steps: Steps = {
        draft: "running",
        shorten: "pending",
        review: "pending",
      };
      const ended: readonly JobDisplay[] = [
        failedJob(steps),
        { kind: "stopped", steps },
        { kind: "key-rejected", steps },
      ];
      for (const job of ended) {
        const root = container();
        renderJobState(root, job, shown(0), locale);
        const [row] = stepRows(root);
        assert.ok(row !== undefined);
        assert.ok(
          query(row, "svg").classList.contains("step-icon-interrupted"),
          job.kind,
        );
        assert.equal(root.querySelector(".step-icon-running"), null, job.kind);
        assert.equal(
          query(row, ".step-state").textContent,
          catalogs[locale].stepStates.interrupted,
          job.kind,
        );
      }
      const running = container();
      renderJobState(running, { kind: "running", steps }, shown(0), locale);
      assert.equal(running.querySelectorAll(".step-icon-running").length, 1);
    });

    it(`lists the steps in the module's order with each step's own state in ${locale}`, () => {
      const catalog = catalogs[locale];
      const root = container();
      renderJobState(
        root,
        { kind: "running", steps: runningSteps },
        shown(0),
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
        shown(0),
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
        shown(65_000),
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

    it(`draws a running job's steps without its elapsed time, and a done job's time taken, when the elapsed time is hidden in ${locale}`, () => {
      const root = container();
      renderJobState(
        root,
        { kind: "running", steps: runningSteps },
        { kind: "hidden" },
        locale,
      );
      assert.equal(stepRows(root).length, 3);
      assert.equal(root.querySelector(".job-elapsed"), null);
      renderJobState(root, doneJob, { kind: "hidden" }, locale);
      assert.equal(
        query(root, ".job-result dd").textContent,
        formatDuration(article.durationMs, locale),
      );
    });

    it(`shows a done job's time taken and cost instead of the elapsed time in ${locale}`, () => {
      const root = container();
      renderJobState(root, doneJob, shown(5_000), locale);
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
        ...doneJob,
        article: { ...article, cost: { state: "unknown" } },
      };
      renderJobState(root, job, shown(0), locale);
      assert.equal(
        query(root, ".job-result dd:last-of-type").textContent,
        viewer.costUnknown,
      );
    });

    // The jobs that called the LLM API but made no article: each shows its
    // steps, its own message and an unknown cost, never an amount.
    const unknownCostJobs: {
      name: string;
      job: JobDisplay;
      message: (l: Locale) => Message;
    }[] = [
      {
        name: "a failed job",
        job: { kind: "failed", steps: failedSteps, failure: "llm_timeout" },
        message: (l) => catalogs[l].failure.llm_timeout,
      },
      {
        name: "a job whose key was rejected",
        job: { kind: "key-rejected", steps: failedSteps },
        message: (l) => catalogs[l].keyRejected,
      },
      {
        name: "a stopped job",
        job: { kind: "stopped", steps: failedSteps },
        message: (l) => catalogs[l].stopped,
      },
    ];
    for (const { name, job, message } of unknownCostJobs) {
      it(`shows ${name}'s steps, its message and an unknown cost in ${locale}`, () => {
        const root = container();
        renderJobState(root, job, shown(0), locale);
        assert.equal(stepRows(root).length, 3);
        expectLocaleMessage(query(root, ".failure"), locale, message);
        assert.deepEqual(
          Array.from(root.querySelectorAll(".job-result dd"), (dd) => [
            dd.previousElementSibling?.textContent,
            dd.textContent,
          ]),
          [[viewer.cost, viewer.costUnknown]],
        );
        assert.ok(
          jobStateSummary(job, [], locale).includes(
            `${viewer.cost} ${viewer.costUnknown}`,
          ),
        );
      });
    }

    for (const failure of ["api_key_missing", "settings_invalid"] as const) {
      it(`shows a job that needed a setting (${failure}) with its message alone, no steps and no cost, in ${locale}`, () => {
        const root = container();
        const job: JobDisplay = { kind: "setup", failure };
        renderJobState(root, job, shown(0), locale);
        assert.deepEqual(stepRows(root), []);
        assert.equal(root.querySelector(".job-result"), null);
        expectLocaleMessage(
          query(root, ".failure"),
          locale,
          (l) => catalogs[l].setup[failure],
        );
        assert.deepEqual(jobStateSummary(job, [], locale), [
          catalogs[locale].setup[failure].text,
        ]);
      });
    }

    it(`shows an article its record could not hold with its facts and the warning that a reload loses it in ${locale}`, () => {
      const root = container();
      renderJobState(
        root,
        { kind: "unstored", steps: doneJob.steps, article },
        shown(0),
        locale,
      );
      assert.equal(stepRows(root).length, 3);
      assert.equal(
        query(root, ".job-result dd").textContent,
        formatDuration(article.durationMs, locale),
      );
      expectLocaleMessage(
        query(root, ".job-notice"),
        locale,
        (l) => catalogs[l].viewer.unstored,
      );
    });

    it(`shows the lost article's message, no steps and no cost in ${locale}`, () => {
      const root = container();
      renderJobState(root, { kind: "lost" }, shown(0), locale);
      assert.deepEqual(stepRows(root), []);
      assert.equal(root.querySelector(".job-result"), null);
      expectLocaleMessage(
        query(root, ".failure"),
        locale,
        (l) => catalogs[l].viewer.lost,
      );
    });

    it(`replaces the previous content on each call in ${locale}`, () => {
      const root = container();
      renderJobState(
        root,
        { kind: "failed", steps: failedSteps, failure: "llm_failed" },
        shown(0),
        locale,
      );
      renderJobState(
        root,
        { kind: "running", steps: runningSteps },
        shown(0),
        locale,
      );
      assert.equal(root.querySelector(".failure"), null);
      assert.equal(root.querySelector(".job-result"), null);
      assert.equal(root.querySelectorAll("ol.step-list").length, 1);
    });
  }
});

describe("start failure", () => {
  const doc = container().ownerDocument;
  const failures: StartFailure[] = [
    { kind: "rejected", reason: "too_many_requests" },
    { kind: "rejected", reason: "api_key_missing" },
    { kind: "unknown" },
    { kind: "input-unavailable" },
  ];

  for (const locale of localeList) {
    for (const failure of failures) {
      it(`shows the message of ${JSON.stringify(failure)} in ${locale}`, () => {
        const block = startFailure(doc, failure, locale);
        assert.ok(block.classList.contains("failure"));
        expectLocaleMessage(block, locale, (l) =>
          startFailureMessage(failure, l),
        );
      });
    }
  }
});

describe("loading indicator", () => {
  for (const locale of localeList) {
    it(`puts the hidden loading mark before the wait's sentence in ${locale}`, () => {
      const doc = container().ownerDocument;
      const [indicator, ...steps] = loadingIndicator(
        doc,
        catalogs[locale].viewer.loading,
      );
      const [icon] = Array.from(indicator.children);
      assert.ok(icon?.classList.contains("loading-icon"));
      assert.equal(icon?.getAttribute("aria-hidden"), "true");
      expectLocaleMessage(indicator, locale, (l) => catalogs[l].viewer.loading);
      assert.deepEqual(steps, [], "a message without steps adds none");
    });
  }

  it("puts a message's numbered steps after the mark-and-sentence row", () => {
    const doc = container().ownerDocument;
    const [indicator, ...steps] = loadingIndicator(
      doc,
      catalogs.en.popup.sending,
    );
    assert.equal(indicator.querySelector("ol"), null);
    assert.equal(steps.length, 1);
    assert.deepEqual(
      Array.from(
        steps[0]?.querySelectorAll("li") ?? [],
        (item) => item.textContent,
      ),
      [...catalogs.en.popup.sending.steps],
    );
  });
});
