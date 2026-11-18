import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import type { Message } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import type {
  ArticleFailure,
  JobDisplay,
  StartFailure,
} from "../src/core/jobView.ts";
import { jobMessage, startFailureMessage } from "../src/core/sendMessages.ts";
import { wasmFailures } from "../src/core/wasmResult.ts";
import { article, failedSteps, runningSteps } from "./helpers/record.ts";

const localeList = Object.keys(locales) as Locale[];

/** Every article failure: the record failures that are neither a setting nor a rejected key. */
const articleFailures = [
  ...Object.keys(wasmFailures).filter(
    (failure) =>
      failure !== "api_key_rejected" && failure !== "settings_invalid",
  ),
  "run_timeout",
] as ArticleFailure[];

describe("startFailureMessage", () => {
  for (const locale of localeList) {
    const catalog = catalogs[locale];
    const cases: { failure: StartFailure; want: Message }[] = [
      {
        failure: { kind: "rejected", reason: "api_key_missing" },
        want: catalog.setup.api_key_missing,
      },
      {
        failure: { kind: "rejected", reason: "invalid_body" },
        want: catalog.startRejected.invalid_body,
      },
      {
        failure: { kind: "rejected", reason: "too_many_requests" },
        want: catalog.startRejected.too_many_requests,
      },
      {
        failure: { kind: "rejected", reason: "internal" },
        want: catalog.startRejected.internal,
      },
      { failure: { kind: "unknown" }, want: catalog.startUnknown },
      {
        failure: { kind: "input-unavailable" },
        want: catalog.inputUnavailable,
      },
    ];

    it(`shows a missing API key as the setup message, and every other failure by its kind, in ${locale}`, () => {
      for (const { failure, want } of cases) {
        assert.equal(
          startFailureMessage(failure, locale),
          want,
          JSON.stringify(failure),
        );
      }
    });

    it(`gives every start failure a distinct text in ${locale}`, () => {
      const texts = cases.map(
        ({ failure }) => startFailureMessage(failure, locale).text,
      );
      assert.equal(new Set(texts).size, cases.length);
    });
  }
});

describe("jobMessage", () => {
  for (const locale of localeList) {
    const catalog = catalogs[locale];

    it(`has no message for a running or done job in ${locale}`, () => {
      assert.equal(
        jobMessage({ kind: "running", steps: runningSteps }, locale),
        undefined,
      );
      assert.equal(
        jobMessage({ kind: "done", steps: runningSteps, article }, locale),
        undefined,
      );
    });

    it(`shows each ended job's own message in ${locale}`, () => {
      const cases: { job: JobDisplay; want: Message }[] = [
        {
          job: { kind: "unstored", steps: failedSteps, article },
          want: catalog.viewer.unstored,
        },
        {
          job: { kind: "setup", failure: "api_key_missing" },
          want: catalog.setup.api_key_missing,
        },
        {
          job: { kind: "setup", failure: "settings_invalid" },
          want: catalog.setup.settings_invalid,
        },
        {
          job: { kind: "key-rejected", steps: failedSteps },
          want: catalog.keyRejected,
        },
        { job: { kind: "stopped", steps: failedSteps }, want: catalog.stopped },
        { job: { kind: "lost" }, want: catalog.viewer.lost },
        ...articleFailures.map((failure) => ({
          job: { kind: "failed", steps: failedSteps, failure } as const,
          want: catalog.failure[failure],
        })),
      ];
      for (const { job, want } of cases) {
        assert.equal(jobMessage(job, locale), want, JSON.stringify(job));
      }
      const texts = cases.map(({ want }) => want.text);
      assert.equal(new Set(texts).size, cases.length);
    });
  }
});
