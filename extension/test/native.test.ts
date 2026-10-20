import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { classifyNativeReply } from "../src/core/native.ts";
import type { NativeReply } from "../src/core/native.ts";
import { maxResponseBytes } from "../src/core/response.ts";
import type { RequestId, SendResult } from "../src/core/response.ts";
import { bytesReply, errorReply, responseReply } from "./helpers/native.ts";

// Tests may assert a branded type to build fixtures.
const requestId = "0123456789abcdef" as RequestId;
const acceptedBody = JSON.stringify({ status: "accepted", requestId });
const acceptedBase64 = Buffer.from(acceptedBody).toString("base64");

/** A reply whose parsed value is value, as the browser delivers it. */
function message(value: unknown): NativeReply {
  return { kind: "message", value };
}

/** A response reply with the given members, starting from a valid accepted one. */
function response(members: Record<string, unknown>): NativeReply {
  return message({
    v: 1,
    kind: "response",
    status: 202,
    body: acceptedBase64,
    ...members,
  });
}

/** The check of an unexpected result, asserting the result is unexpected. */
function unexpectedCheck(result: SendResult): string {
  assert.ok(result.kind === "unexpected", JSON.stringify(result));
  return result.check;
}

describe("classifyNativeReply", () => {
  it("returns extension-timeout when the limit passed first", () => {
    assert.deepEqual(classifyNativeReply({ kind: "timed-out" }), {
      kind: "unexpected",
      check: "extension-timeout",
    });
  });

  // The texts are what Chrome and Brave gave for a missing manifest or binary,
  // a refused origin, and a host that exited without a reply, plus texts no
  // browser gives: the result must not depend on the text at all.
  const lastErrors = [
    "Specified native messaging host not found.",
    "Access to the specified native messaging host is forbidden.",
    "Native host has exited.",
    "Error when communicating with the native messaging host.",
    "",
    "server_unavailable",
    undefined,
  ];
  for (const lastError of lastErrors) {
    it(`returns host-failed for the browser error ${JSON.stringify(lastError)}`, () => {
      assert.deepEqual(classifyNativeReply({ kind: "failed", lastError }), {
        kind: "host-failed",
        lastError,
      });
    });
  }

  const hostErrors: { error: string; expected: SendResult }[] = [
    {
      error: "server_unavailable",
      expected: { kind: "not-sent", cause: "server-unavailable" },
    },
    {
      error: "message_too_large",
      expected: { kind: "not-sent", cause: "message-too-large" },
    },
    { error: "server_timeout", expected: { kind: "no-response" } },
    {
      error: "protocol_error",
      expected: { kind: "unexpected", check: "host-protocol-error" },
    },
  ];
  for (const { error, expected } of hostErrors) {
    it(`maps the host error ${error}`, () => {
      assert.deepEqual(classifyNativeReply(errorReply(error)), expected);
    });
  }

  it("classifies a wrapped accepted response", () => {
    assert.deepEqual(classifyNativeReply(responseReply(202, acceptedBody)), {
      kind: "accepted",
      requestId,
    });
  });

  it("classifies a wrapped rejection", () => {
    const text = JSON.stringify({
      status: "rejected",
      reason: "invalid_transcript",
      requestId,
    });
    assert.deepEqual(classifyNativeReply(responseReply(400, text)), {
      kind: "rejected",
      reason: "invalid_transcript",
      requestId,
    });
  });

  const envelopes: { name: string; reply: NativeReply; check: string }[] = [
    { name: "a null value", reply: message(null), check: "envelope-shape" },
    { name: "an array", reply: message([]), check: "envelope-shape" },
    { name: "a string", reply: message("x"), check: "envelope-shape" },
    { name: "a number", reply: message(1), check: "envelope-shape" },
    {
      name: "a reply without v",
      reply: message({ kind: "error", error: "server_timeout" }),
      check: "envelope-version",
    },
    {
      name: "version 2",
      reply: message({ v: 2, kind: "error", error: "server_timeout" }),
      check: "envelope-version",
    },
    {
      name: "a string version",
      reply: message({ v: "1", kind: "error", error: "server_timeout" }),
      check: "envelope-version",
    },
    {
      name: "an unknown kind",
      reply: message({ v: 1, kind: "other" }),
      check: "envelope-shape",
    },
    {
      name: "an inherited name as the kind",
      reply: message({ v: 1, kind: "constructor" }),
      check: "envelope-shape",
    },
    {
      name: "a reply without kind",
      reply: message({ v: 1, error: "server_timeout" }),
      check: "envelope-shape",
    },
    {
      name: "an unknown host error",
      reply: errorReply("server_down"),
      check: "envelope-shape",
    },
    {
      name: "an inherited name as the host error",
      reply: errorReply("toString"),
      check: "envelope-shape",
    },
    {
      name: "a non-string host error",
      reply: message({ v: 1, kind: "error", error: 1 }),
      check: "envelope-shape",
    },
    {
      name: "an error reply with an extra member",
      reply: message({
        v: 1,
        kind: "error",
        error: "server_timeout",
        status: 202,
      }),
      check: "envelope-shape",
    },
    {
      name: "an error reply without error",
      reply: message({ v: 1, kind: "error" }),
      check: "envelope-shape",
    },
    {
      name: "a response reply with an extra member",
      reply: response({ error: "server_timeout" }),
      check: "envelope-shape",
    },
    {
      name: "a response reply without body",
      reply: message({ v: 1, kind: "response", status: 202 }),
      check: "envelope-shape",
    },
    {
      name: "a response reply without status",
      reply: message({ v: 1, kind: "response", body: acceptedBase64 }),
      check: "envelope-shape",
    },
    {
      name: "a string status",
      reply: response({ status: "202" }),
      check: "envelope-shape",
    },
    {
      name: "a fractional status",
      reply: response({ status: 202.5 }),
      check: "envelope-shape",
    },
    {
      name: "status 99",
      reply: response({ status: 99 }),
      check: "envelope-shape",
    },
    {
      name: "status 600",
      reply: response({ status: 600 }),
      check: "envelope-shape",
    },
    {
      name: "a non-string body",
      reply: response({ body: [1, 2] }),
      check: "envelope-shape",
    },
    {
      name: "a body that is not base64",
      reply: response({ body: "not base64!" }),
      check: "envelope-shape",
    },
    {
      name: "a body without its padding",
      reply: response({ body: acceptedBase64.replace(/=+$/u, "") }),
      check: "envelope-shape",
    },
    {
      name: "a body with extra padding",
      reply: response({ body: `${acceptedBase64}==` }),
      check: "envelope-shape",
    },
    {
      name: "a body with a line break",
      reply: response({
        body: `${acceptedBase64.slice(0, 8)}\n${acceptedBase64.slice(8)}`,
      }),
      check: "envelope-shape",
    },
    {
      name: "a body with a space",
      reply: response({ body: ` ${acceptedBase64}` }),
      check: "envelope-shape",
    },
    {
      // "YR==" decodes to "a" like "YQ==", but its unused bits are not zero.
      name: "a body with non-zero trailing bits",
      reply: response({ body: "YR==" }),
      check: "envelope-shape",
    },
  ];
  for (const { name, reply, check } of envelopes) {
    it(`rejects ${name} with ${check}`, () => {
      assert.equal(unexpectedCheck(classifyNativeReply(reply)), check);
    });
  }

  // Precondition for the base64 rows above: each wrong form differs from the
  // canonical one only in its base64 text, and the canonical one is accepted.
  it("accepts the canonical base64 of the same body", () => {
    assert.equal(acceptedBase64.endsWith("="), true);
    assert.equal(classifyNativeReply(response({})).kind, "accepted");
  });

  const statusBounds = [100, 599];
  for (const status of statusBounds) {
    it(`passes status ${status} on to the body checks`, () => {
      // Status 100 and 599 pass the envelope; no reason pairs with them.
      const result = classifyNativeReply(response({ status }));
      assert.deepEqual(result, {
        kind: "unexpected",
        check: "status-pairing",
        status,
      });
    });
  }

  it("accepts a decoded body exactly at the limit", () => {
    const padded =
      acceptedBody + " ".repeat(maxResponseBytes - acceptedBody.length);
    assert.deepEqual(classifyNativeReply(responseReply(202, padded)), {
      kind: "accepted",
      requestId,
    });
  });

  it("passes a decoded body one byte over the limit as body-over-limit", () => {
    const padded =
      acceptedBody + " ".repeat(maxResponseBytes + 1 - acceptedBody.length);
    assert.deepEqual(classifyNativeReply(responseReply(202, padded)), {
      kind: "unexpected",
      check: "body-over-limit",
      status: 202,
    });
  });

  // Base64 keeps the server's bytes as they are, so the body checks see what
  // the server sent: an invalid UTF-8 body would not survive a JSON string.
  it("passes the server's bytes unchanged to the body checks", () => {
    const reply = bytesReply(202, new Uint8Array([0x22, 0xff, 0x22]));
    assert.equal(unexpectedCheck(classifyNativeReply(reply)), "encoding");
  });
});
