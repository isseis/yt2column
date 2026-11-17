import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { recordFailures } from "../src/core/jobRecord.ts";
import type { JobRecord, StopReason } from "../src/core/jobRecord.ts";
import { internalDetails } from "../src/core/wasmResult.ts";
import type { StepName, StepState } from "../src/core/wasmResult.ts";
import { developmentDoc } from "./helpers/developmentDoc.ts";
import { cells, markdownSection, readRepoFile } from "./helpers/repoFile.ts";

// The value sets the record's types declare without a runtime set of their
// own. `satisfies` makes each one exactly the type's values: a value added to
// or removed from the type fails the type check here until the set follows.
const jobStates = {
  running: true,
  done: true,
  failed: true,
  stopped: true,
} as const satisfies Record<JobRecord["state"], true>;
const stepNames = {
  draft: true,
  shorten: true,
  review: true,
} as const satisfies Record<StepName, true>;
const stepStates = {
  pending: true,
  running: true,
  done: true,
  skipped: true,
  failed: true,
} as const satisfies Record<StepState, true>;
const stopReasons = {
  tab_closed: true,
  page_closed: true,
  page_reloaded: true,
  no_contact: true,
} as const satisfies Record<StopReason, true>;

/**
 * The codes in the first column of the table, in the job record section,
 * whose first header cell is header: the backquoted value of each row. A
 * code listed twice fails instead of being counted once.
 */
function documentedCodes(section: string, header: string): string[] {
  const lines = section.split("\n");
  const start = lines.findIndex(
    (line) => line.startsWith("|") && cells(line)[0] === header,
  );
  assert.ok(start >= 0, `${developmentDoc.path} has no "${header}" table`);
  const codes: string[] = [];
  // Skip the header and the separator row.
  for (const line of lines.slice(start + 2)) {
    if (!line.startsWith("|")) {
      break;
    }
    const [cell = ""] = cells(line);
    const match = /^`([^`]+)`$/u.exec(cell);
    assert.ok(match !== null, `${header}: "${cell}" is not one code`);
    const code = match[1] ?? "";
    assert.ok(!codes.includes(code), `${header} lists ${code} twice`);
    codes.push(code);
  }
  return codes;
}

describe("job record values match the development guide", () => {
  const section = markdownSection(
    readRepoFile(developmentDoc.path),
    developmentDoc.jobCodesHeading,
  );
  const { jobCodeTables } = developmentDoc;
  const sets: { header: string; codes: Readonly<Record<string, true>> }[] = [
    { header: jobCodeTables.jobState, codes: jobStates },
    { header: jobCodeTables.step, codes: stepNames },
    { header: jobCodeTables.stepState, codes: stepStates },
    { header: jobCodeTables.failure, codes: recordFailures },
    { header: jobCodeTables.internalDetail, codes: internalDetails },
    { header: jobCodeTables.stopReason, codes: stopReasons },
  ];

  for (const { header, codes } of sets) {
    it(`has exactly the extension's set in the "${header}" table`, () => {
      assert.deepEqual(
        documentedCodes(section, header).sort(),
        Object.keys(codes).sort(),
      );
    });
  }
});
