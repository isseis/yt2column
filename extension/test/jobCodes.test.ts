import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  failureCodes,
  jobStates,
  publishReasons,
  publishStates,
  stepNames,
  stepStates,
} from "../src/core/response.ts";
import { developmentDoc } from "./helpers/developmentDoc.ts";
import { cells, markdownSection, readRepoFile } from "./helpers/repoFile.ts";

/**
 * The codes in the first column of the table, in the job API code section,
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

describe("job codes match the development guide", () => {
  const section = markdownSection(
    readRepoFile(developmentDoc.path),
    developmentDoc.jobCodesHeading,
  );
  const { jobCodeTables } = developmentDoc;
  const sets: { header: string; codes: Record<string, true> }[] = [
    { header: jobCodeTables.jobState, codes: jobStates },
    { header: jobCodeTables.step, codes: stepNames },
    { header: jobCodeTables.stepState, codes: stepStates },
    { header: jobCodeTables.failure, codes: failureCodes },
    { header: jobCodeTables.publishState, codes: publishStates },
    { header: jobCodeTables.publishReason, codes: publishReasons },
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
