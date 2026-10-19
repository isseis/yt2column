import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  classifyResponse,
  isServerRejectReason,
  parseRequestId,
  serverRejectStatus,
  toSendOutcome,
} from "../src/core/response.ts";
import type {
  ReceivedResponse,
  RequestId,
  SendResult,
  ServerRejectReason,
} from "../src/core/response.ts";

// Tests may assert a branded type to build fixtures.
const requestId = "0123456789abcdef" as RequestId;

type BodyResponse = Extract<ReceivedResponse, { kind: "body" }>;

/** A UTF-8 encoded response body at status. */
function body(status: number, text: string): BodyResponse {
  return { kind: "body", status, bytes: new TextEncoder().encode(text) };
}

/** Raw bytes as a response body, for the invalid-UTF-8 cases. */
function bytes(status: number, value: number[]): BodyResponse {
  return { kind: "body", status, bytes: new Uint8Array(value) };
}

/** The reason values, from the runtime record. */
const reasons = Object.keys(serverRejectStatus) as ServerRejectReason[];

describe("serverRejectStatus", () => {
  it("declares eight reasons", () => {
    assert.equal(reasons.length, 8);
  });

  it("is queried through isServerRejectReason", () => {
    assert.equal(isServerRejectReason("too_many_requests"), true);
    assert.equal(isServerRejectReason("bogus"), false);
    assert.equal(isServerRejectReason(1), false);
  });

  it("pins the reason-to-status pairs", () => {
    const pairs: [ServerRejectReason, number][] = [
      ["too_many_requests", 429],
      ["body_too_large", 413],
      ["invalid_body", 400],
      ["invalid_video_url", 400],
      ["invalid_transcript", 400],
      ["internal_error", 500],
      ["not_found", 404],
      ["method_not_allowed", 405],
    ];
    for (const [reason, status] of pairs) {
      assert.equal(serverRejectStatus[reason], status, reason);
    }
  });
});

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

describe("classifyResponse", () => {
  it("accepts a well-formed accepted response", () => {
    const text = JSON.stringify({ status: "accepted", requestId });
    assert.deepEqual(classifyResponse(body(202, text)), {
      kind: "accepted",
      requestId,
    });
  });

  for (const reason of reasons) {
    it(`accepts a well-formed ${reason} rejection`, () => {
      const status = serverRejectStatus[reason];
      const text = JSON.stringify({ status: "rejected", reason, requestId });
      assert.deepEqual(classifyResponse(body(status, text)), {
        kind: "rejected",
        reason,
        requestId,
      });
    });
  }

  it("carries a redirect", () => {
    assert.deepEqual(classifyResponse({ kind: "redirected" }), {
      kind: "redirected",
    });
  });

  type UnexpectedSource = Extract<
    ReceivedResponse,
    { kind: "body" | "body-over-limit" }
  >;
  const unexpecteds: {
    name: string;
    received: UnexpectedSource;
    check: string;
  }[] = [
    {
      name: "a body over the limit",
      received: { kind: "body-over-limit", status: 202 },
      check: "body-over-limit",
    },
    {
      name: "invalid UTF-8",
      received: bytes(202, [0x22, 0xff, 0x22]),
      check: "encoding",
    },
    {
      name: "an escape",
      received: body(
        202,
        '{"status":"accepted","requestId":"0123456789abcdef\\n"}',
      ),
      check: "escape",
    },
    { name: "an empty body", received: body(202, ""), check: "json-shape" },
    { name: "a JSON array", received: body(202, "[]"), check: "json-shape" },
    { name: "a JSON string", received: body(202, '"x"'), check: "json-shape" },
    { name: "JSON null", received: body(202, "null"), check: "json-shape" },
    {
      name: "a non-string member",
      received: body(202, '{"status":1,"requestId":"0123456789abcdef"}'),
      check: "json-shape",
    },
    {
      name: "a leading BOM",
      received: body(202, "\uFEFF{}"),
      check: "json-shape",
    },
    {
      name: "a duplicate member (string)",
      received: body(
        202,
        '{"status":"accepted","status":"rejected","requestId":"0123456789abcdef"}',
      ),
      check: "duplicate-member",
    },
    {
      name: "a duplicate member (number)",
      received: body(
        202,
        '{"status":1,"status":"accepted","requestId":"0123456789abcdef"}',
      ),
      check: "duplicate-member",
    },
    {
      name: "a duplicate member (null)",
      received: body(
        202,
        '{"status":null,"status":"accepted","requestId":"0123456789abcdef"}',
      ),
      check: "duplicate-member",
    },
    {
      name: "a duplicate member (object)",
      received: body(
        202,
        '{"status":{},"status":"accepted","requestId":"0123456789abcdef"}',
      ),
      check: "duplicate-member",
    },
    {
      name: "an extra member",
      received: body(
        202,
        '{"status":"accepted","requestId":"0123456789abcdef","extra":"x"}',
      ),
      check: "member-set",
    },
    {
      name: "a missing member",
      received: body(202, '{"status":"accepted"}'),
      check: "member-set",
    },
    {
      name: "an unknown status value",
      received: body(202, '{"status":"other","requestId":"0123456789abcdef"}'),
      check: "member-set",
    },
    {
      // parseRequestId owns the requestId grammar cases; this row only
      // exercises the classification branch.
      name: "a malformed requestId",
      received: body(
        202,
        '{"status":"accepted","requestId":"0123456789abcde"}',
      ),
      check: "request-id",
    },
    {
      name: "202 with a rejected body",
      received: body(
        202,
        '{"status":"rejected","reason":"too_many_requests","requestId":"0123456789abcdef"}',
      ),
      check: "status-pairing",
    },
    {
      name: "200 with an accepted body",
      received: body(
        200,
        '{"status":"accepted","requestId":"0123456789abcdef"}',
      ),
      check: "status-pairing",
    },
    {
      name: "401 with an accepted body",
      received: body(
        401,
        '{"status":"accepted","requestId":"0123456789abcdef"}',
      ),
      check: "status-pairing",
    },
    {
      name: "an unknown reason",
      received: body(
        400,
        '{"status":"rejected","reason":"bogus","requestId":"0123456789abcdef"}',
      ),
      check: "status-pairing",
    },
    {
      name: "an inherited-name reason (__proto__)",
      received: body(
        400,
        '{"status":"rejected","reason":"__proto__","requestId":"0123456789abcdef"}',
      ),
      check: "status-pairing",
    },
    {
      name: "an inherited-name reason (toString)",
      received: body(
        400,
        '{"status":"rejected","reason":"toString","requestId":"0123456789abcdef"}',
      ),
      check: "status-pairing",
    },
    {
      name: "too_many_requests with the wrong status",
      received: body(
        403,
        '{"status":"rejected","reason":"too_many_requests","requestId":"0123456789abcdef"}',
      ),
      check: "status-pairing",
    },
  ];
  for (const { name, received, check } of unexpecteds) {
    it(`rejects ${name} as unexpected with check ${check}`, () => {
      const result = classifyResponse(received);
      assert.ok(result.kind === "unexpected");
      assert.equal(result.check, check);
      assert.equal(result.status, received.status);
    });
  }

  const specialMembers = [
    "__proto__",
    "constructor",
    "toString",
    "hasOwnProperty",
  ];
  for (const member of specialMembers) {
    it(`rejects a body with a ${member} member without throwing`, () => {
      const text = `{"status":"accepted","requestId":"0123456789abcdef","${member}":"x"}`;
      const result = classifyResponse(body(202, text));
      assert.ok(result.kind === "unexpected");
      assert.equal(result.check, "member-set");
    });
  }

  it("does not carry a markup reason into the result", () => {
    const markup = "<img src=x onerror=alert(1)>";
    const text = JSON.stringify({
      status: "rejected",
      reason: markup,
      requestId,
    });
    const result = classifyResponse(body(400, text));
    assert.ok(result.kind === "unexpected");
    assert.equal(JSON.stringify(result).includes("img"), false);
  });

  it("does not carry a markup requestId into the result", () => {
    const markup = "<img src=x onerror=alert(1)>";
    const text = JSON.stringify({ status: "accepted", requestId: markup });
    const result = classifyResponse(body(202, text));
    assert.ok(result.kind === "unexpected");
    assert.equal(result.check, "request-id");
    assert.equal(JSON.stringify(result).includes("img"), false);
  });
});

describe("toSendOutcome", () => {
  it("drops the cause of an unknown result", () => {
    const result: SendResult = { kind: "unknown", cause: new Error("boom") };
    assert.deepEqual(toSendOutcome(result), { kind: "unknown" });
  });

  it("passes a known outcome through", () => {
    const outcome: SendResult = { kind: "accepted", requestId };
    assert.deepEqual(toSendOutcome(outcome), outcome);
  });
});
