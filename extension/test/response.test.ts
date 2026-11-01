import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { parseRequestId } from "../src/core/response.ts";
import type { RequestId } from "../src/core/response.ts";

// Tests may assert a branded type to build fixtures.
const requestId = "0123456789abcdef" as RequestId;

describe("parseRequestId", () => {
  it("accepts 16 lowercase hex characters", () => {
    assert.equal(parseRequestId(requestId), requestId);
  });

  const rejected = ["0123456789ABCDEF", "0123456789abcde", "0123456789abcdef0"];
  for (const value of rejected) {
    it(`rejects ${value}`, () => {
      assert.equal(parseRequestId(value), undefined);
    });
  }
});
