import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { serverRejectStatus } from "../src/core/response.ts";
import { readRepoFile } from "./helpers/repoFile.ts";

/** The reason-to-status pairs from README.md's rejection table, as a map. */
function readmeReasons(): Map<string, number> {
  const rows = new Map<string, number>();
  const lines = readRepoFile("README.md").split("\n");
  const start = lines.findIndex((line) =>
    /^\|\s*reason\s*\|\s*status code\s*\|/.test(line),
  );
  if (start < 0) {
    return rows;
  }
  // Skip the header and the separator row.
  for (const line of lines.slice(start + 2)) {
    if (!line.startsWith("|")) {
      break;
    }
    const cells = line
      .split("|")
      .slice(1, -1)
      .map((cell) => cell.trim());
    const reason = cells[0]?.replaceAll("`", "") ?? "";
    const status = Number(cells[1]);
    if (reason !== "" && Number.isInteger(status)) {
      rows.set(reason, status);
    }
  }
  return rows;
}

describe("server reasons match README", () => {
  it("has exactly the README rejection table's pairs", () => {
    const readme = readmeReasons();
    assert.notEqual(readme.size, 0, "README.md has no rejection reason table");
    const sorted = (map: Map<string, number>) =>
      [...map.entries()].sort(([a], [b]) => a.localeCompare(b));
    assert.deepEqual(
      sorted(new Map(Object.entries(serverRejectStatus))),
      sorted(readme),
    );
  });
});
