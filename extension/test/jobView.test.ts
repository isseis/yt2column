import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { publishControl } from "../src/core/jobView.ts";
import type { JobDisplay, PublishControl } from "../src/core/jobView.ts";
import { article, doneJob } from "./helpers/job.ts";

type Publishable = Extract<JobDisplay, { kind: "done" | "lost" }>;

describe("publishControl", () => {
  const rows: { name: string; job: Publishable; control: PublishControl }[] = [
    {
      name: "a lost job",
      job: { kind: "lost" },
      control: { enabled: false, row: { row: "lost" } },
    },
    {
      name: "a lost job that kept its article",
      job: { kind: "lost", kept: article },
      control: { enabled: false, row: { row: "lost" } },
    },
    {
      name: "a posted article",
      job: doneJob({ state: "posted" }),
      control: { enabled: false, row: { row: "posted" } },
    },
    {
      name: "a posting article",
      job: doneJob({ state: "posting" }),
      control: { enabled: false, row: { row: "posting" } },
    },
    {
      name: "an unpostable article",
      job: doneJob({ state: "unpostable", reason: "mention" }),
      control: {
        enabled: false,
        row: { row: "unpostable", reason: "mention" },
      },
    },
    {
      name: "a Webhook that is not configured",
      job: doneJob({ state: "not_posted" }, false),
      control: { enabled: false, row: { row: "webhook-not-configured" } },
    },
    {
      name: "a failed post",
      job: doneJob({ state: "failed", reason: "webhook" }),
      control: { enabled: true, row: { row: "failed", reason: "webhook" } },
    },
    {
      name: "an article not posted",
      job: doneJob({ state: "not_posted" }),
      control: { enabled: true, row: { row: "not-posted" } },
    },
    // Two rows apply at once: the upper row decides.
    {
      name: "a posted article without a Webhook",
      job: doneJob({ state: "posted" }, false),
      control: { enabled: false, row: { row: "posted" } },
    },
    {
      name: "a posting article without a Webhook",
      job: doneJob({ state: "posting" }, false),
      control: { enabled: false, row: { row: "posting" } },
    },
    {
      name: "an unpostable article without a Webhook",
      job: doneJob({ state: "unpostable", reason: "other" }, false),
      control: { enabled: false, row: { row: "unpostable", reason: "other" } },
    },
    {
      name: "a failed post without a Webhook",
      job: doneJob({ state: "failed", reason: "internal" }, false),
      control: { enabled: false, row: { row: "webhook-not-configured" } },
    },
  ];
  for (const { name, job, control } of rows) {
    it(`decides ${name}`, () => {
      assert.deepEqual(publishControl(job, false), control);
    });
  }

  it("disables every row while publishing, keeping the row", () => {
    for (const { name, job, control } of rows) {
      assert.deepEqual(
        publishControl(job, true),
        { enabled: false, row: control.row },
        name,
      );
    }
  });
});
