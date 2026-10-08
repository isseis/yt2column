import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { rejectionMessage } from "../src/core/messages.ts";
import { rejectionReasons } from "../src/core/types.ts";
import type { RejectionReason } from "../src/core/types.ts";

const reasons = Object.keys(rejectionReasons) as RejectionReason[];

describe("rejectionMessage", () => {
  it("gives every reason a distinct non-empty text", () => {
    const texts = reasons.map((reason) => rejectionMessage(reason).text);
    for (const text of texts) {
      assert.notEqual(text, "");
    }
    assert.equal(new Set(texts).size, reasons.length);
  });

  it("gives steps only to not-watch-page and empty-selection", () => {
    for (const reason of reasons) {
      const hasSteps = rejectionMessage(reason).steps.length > 0;
      const wantsSteps =
        reason === "not-watch-page" || reason === "empty-selection";
      assert.equal(hasSteps, wantsSteps, reason);
    }
  });
});
