import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { preflightReasons } from "../src/core/prepare.ts";
import type { BlockReason } from "../src/core/prepare.ts";
import { parseResultView } from "../src/core/resultView.ts";
import type { CollectedSummary } from "../src/core/resultView.ts";
import { serverRejectStatus } from "../src/core/response.ts";
import type {
  RequestId,
  SendOutcome,
  ServerRejectReason,
} from "../src/core/response.ts";
import { rejectionReasons } from "../src/core/types.ts";
import type { RejectionReason } from "../src/core/types.ts";

// Tests may assert a branded type to build fixtures.
const requestId = "0123456789abcdef" as RequestId;

const collected: CollectedSummary = {
  kind: "collected",
  title: "A video",
  titleTruncated: false,
  url: "https://www.youtube.com/watch?v=abc",
  urlTruncated: false,
  characterCount: 2,
  lineCount: 1,
  preview: "hi",
  previewTruncated: false,
};

/** A sent view with the given outcome, typed as unknown so malformed values pass. */
function sent(outcome: unknown): unknown {
  return { kind: "sent", summary: collected, outcome };
}

describe("parseResultView", () => {
  it("accepts a blocked view for every block reason", () => {
    const reasons: BlockReason[] = [
      ...(Object.keys(rejectionReasons) as RejectionReason[]),
      ...(Object.keys(preflightReasons) as (keyof typeof preflightReasons)[]),
    ];
    for (const reason of reasons) {
      const view = { kind: "blocked", reason };
      assert.deepEqual(parseResultView(view), view, reason);
    }
  });

  it("accepts a sending view", () => {
    const view = { kind: "sending", summary: collected };
    assert.deepEqual(parseResultView(view), view);
  });

  it("accepts a sent view for every outcome kind", () => {
    // "rejected" is covered exhaustively by its own test below.
    const outcomes: SendOutcome[] = [
      { kind: "accepted", requestId },
      { kind: "unknown" },
      { kind: "unexpected", status: 500, check: "json-shape" },
      { kind: "redirected" },
    ];
    for (const outcome of outcomes) {
      const view = sent(outcome);
      assert.deepEqual(parseResultView(view), view, JSON.stringify(outcome));
    }
  });

  it("rejects a sent view whose unknown outcome carries a cause", () => {
    assert.equal(
      parseResultView(sent({ kind: "unknown", cause: "boom" })),
      undefined,
    );
  });

  const badStatuses = [99, 600, 200.5, "202"];
  for (const status of badStatuses) {
    it(`rejects an unexpected outcome with status ${String(status)}`, () => {
      assert.equal(
        parseResultView(
          sent({ kind: "unexpected", status, check: "json-shape" }),
        ),
        undefined,
      );
    });
  }

  for (const status of [100, 599]) {
    it(`accepts an unexpected outcome with status ${status}`, () => {
      const view = sent({ kind: "unexpected", status, check: "json-shape" });
      assert.deepEqual(parseResultView(view), view);
    });
  }

  it("rejects an unknown unexpected check", () => {
    assert.equal(
      parseResultView(
        sent({ kind: "unexpected", status: 500, check: "bogus" }),
      ),
      undefined,
    );
  });

  it("rejects a non-object value", () => {
    for (const value of [undefined, null, 1, "x", true]) {
      assert.equal(parseResultView(value), undefined);
    }
  });

  it("rejects an unknown kind", () => {
    assert.equal(parseResultView({ kind: "other" }), undefined);
  });

  it("rejects a blocked view with an unknown reason", () => {
    assert.equal(
      parseResultView({ kind: "blocked", reason: "bogus" }),
      undefined,
    );
  });

  it("rejects a sending view whose summary is a rejection", () => {
    assert.equal(
      parseResultView({
        kind: "sending",
        summary: { kind: "rejected", reason: "empty-selection" },
      }),
      undefined,
    );
  });

  it("rejects a blocked view with an extra field", () => {
    assert.equal(
      parseResultView({ kind: "blocked", reason: "empty-title", extra: 1 }),
      undefined,
    );
  });

  it("rejects a sending view missing the summary", () => {
    assert.equal(parseResultView({ kind: "sending" }), undefined);
  });

  it("rejects an accepted outcome with an unparseable requestId", () => {
    assert.equal(
      parseResultView(sent({ kind: "accepted", requestId: "NOPE" })),
      undefined,
    );
  });

  it("rejects a rejected outcome with an unknown reason", () => {
    const outcome = { kind: "rejected", reason: "bogus", requestId };
    assert.equal(parseResultView(sent(outcome)), undefined);
  });

  it("accepts a rejected outcome for every server reason", () => {
    const reasons = Object.keys(serverRejectStatus) as ServerRejectReason[];
    for (const reason of reasons) {
      const view = sent({ kind: "rejected", reason, requestId });
      assert.deepEqual(parseResultView(view), view, reason);
    }
  });
});
