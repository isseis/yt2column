import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { articlesRejectStatus } from "../src/core/articlesResponse.ts";
import { developmentDoc } from "./helpers/developmentDoc.ts";
import { cells, readRepoFile } from "./helpers/repoFile.ts";

/**
 * The reason-to-status pairs of the development guide's rejection table whose
 * display cell is a fixed message (also when it turns into an unexpected
 * reply for an empty requestId), as a map. The other rows (the server's
 * answers to a wrong path or method, and to the job API) are shown as an
 * unexpected reply, so the extension has no reason for them. A display cell
 * in no category fails the test instead of being taken for one.
 */
function documentedFixedMessageReasons(): Map<string, number> {
  const rows = new Map<string, number>();
  const lines = readRepoFile(developmentDoc.path).split("\n");
  const start = lines.findIndex(
    (line) =>
      line.trim().startsWith("|") &&
      cells(line).join("|") === developmentDoc.rejectionColumns.join("|"),
  );
  if (start < 0) {
    return rows;
  }
  const { display } = developmentDoc;
  const seen = new Set<string>();
  // Skip the header and the separator row.
  for (const line of lines.slice(start + 2)) {
    if (!line.startsWith("|")) {
      break;
    }
    const [reasonCell = "", statusCell = "", displayCell = ""] = cells(line);
    const reason = reasonCell.replaceAll("`", "");
    assert.ok(
      !seen.has(reason),
      `${developmentDoc.path} lists ${reason} twice`,
    );
    seen.add(reason);
    switch (displayCell) {
      case display.fixedMessage:
      case display.fixedMessageUnlessNoRequestId:
        rows.set(reason, Number(statusCell));
        break;
      case display.unexpectedReply:
        break;
      default:
        assert.fail(`${developmentDoc.path}: ${reason} shows "${displayCell}"`);
    }
  }
  return rows;
}

describe("server reasons match the development guide", () => {
  it("has exactly the rejection table's fixed-message pairs", () => {
    const documented = documentedFixedMessageReasons();
    assert.notEqual(
      documented.size,
      0,
      `${developmentDoc.path} has no rejection reason table with fixed-message rows`,
    );
    const sorted = (map: Map<string, number>) =>
      [...map.entries()].sort(([a], [b]) => a.localeCompare(b));
    assert.deepEqual(
      sorted(new Map(Object.entries(articlesRejectStatus))),
      sorted(documented),
    );
  });
});
