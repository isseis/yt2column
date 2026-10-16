import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { CollectedInput } from "../src/core/collect.ts";
import { buildArticleBody } from "../src/core/prepare.ts";
import type { PreparedRequest } from "../src/core/prepare.ts";
import { maxResponseBytes } from "../src/core/response.ts";
import type { RequestId } from "../src/core/response.ts";
import type { ServerPort, SharedToken } from "../src/core/settings.ts";
import { sendArticle, sendTimeoutMs } from "../src/send.ts";
import {
  endlessStream,
  failingStream,
  recordingFetch,
  redirectResponse,
  rejectingFetch,
  streamOf,
  testSendDeps,
  textResponse,
} from "./helpers/fetch.ts";
import { identifiableToken } from "./helpers/settings.ts";

const requestId = "0123456789abcdef" as RequestId;
const accepted = JSON.stringify({ status: "accepted", requestId });

// Tests may assert a branded type to build fixtures.
const request: PreparedRequest = {
  url: "http://127.0.0.1:8765/articles",
  port: "8765" as ServerPort,
  token: identifiableToken as SharedToken,
  body: buildArticleBody(
    {
      selection: "hello",
      url: "https://www.youtube.com/watch?v=abc",
      title: "a tab title",
    } as CollectedInput,
    {},
  ),
} as PreparedRequest;

describe("sendArticle", () => {
  it("posts once with the fixed RequestInit and only the two headers", async () => {
    const { fetchFn, calls } = recordingFetch(async () =>
      textResponse(202, accepted),
    );
    const timeouts: number[] = [];
    const deps = {
      fetch: fetchFn,
      timeoutSignal: (ms: number) => {
        timeouts.push(ms);
        return new AbortController().signal;
      },
    };
    const result = await sendArticle(request, deps);

    assert.deepEqual(result, { kind: "accepted", requestId });
    assert.equal(calls.length, 1);
    const call = calls[0];
    assert.ok(call !== undefined);
    assert.equal(call.url, "http://127.0.0.1:8765/articles");
    assert.equal(call.init.method, "POST");
    assert.deepEqual(call.init.headers, {
      "Content-Type": "application/json",
      Authorization: `Bearer ${identifiableToken}`,
    });
    assert.equal(call.init.body, request.body.json);
    assert.equal(call.init.redirect, "manual");
    assert.equal(call.init.credentials, "omit");
    assert.equal(call.init.cache, "no-store");
    assert.ok(call.init.signal instanceof AbortSignal);
    assert.deepEqual(timeouts, [sendTimeoutMs]);
  });

  it("classifies a body delivered in several chunks", async () => {
    const bytes = new TextEncoder().encode(accepted);
    const body = streamOf([bytes.slice(0, 10), bytes.slice(10)]);
    const { fetchFn } = recordingFetch(async () => ({
      type: "basic",
      status: 202,
      body,
    }));
    assert.deepEqual(await sendArticle(request, testSendDeps(fetchFn)), {
      kind: "accepted",
      requestId,
    });
  });

  const rejections: { name: string; cause: unknown }[] = [
    { name: "a TypeError", cause: new TypeError("Failed to fetch") },
    {
      name: "a TimeoutError",
      cause: new DOMException("timed out", "TimeoutError"),
    },
  ];
  for (const { name, cause } of rejections) {
    it(`returns unknown and calls fetch once when fetch rejects with ${name}`, async () => {
      const { fetchFn, calls } = rejectingFetch(cause);
      assert.deepEqual(await sendArticle(request, testSendDeps(fetchFn)), {
        kind: "unknown",
        cause,
      });
      assert.equal(calls.length, 1);
    });
  }

  it("reads a body exactly at the limit and classifies it", async () => {
    const padded = accepted + " ".repeat(maxResponseBytes - accepted.length);
    const { fetchFn } = recordingFetch(async () => textResponse(202, padded));
    assert.deepEqual(await sendArticle(request, testSendDeps(fetchFn)), {
      kind: "accepted",
      requestId,
    });
  });

  it("reports a body one byte over the limit as unexpected", async () => {
    const padded =
      accepted + " ".repeat(maxResponseBytes + 1 - accepted.length);
    const { fetchFn } = recordingFetch(async () => textResponse(202, padded));
    const result = await sendArticle(request, testSendDeps(fetchFn));
    assert.ok(result.kind === "unexpected");
    assert.equal(result.check, "body-over-limit");
  });

  it("cancels an endless body and stops reading", async () => {
    const tracked = endlessStream(512);
    const { fetchFn } = recordingFetch(async () => ({
      type: "basic",
      status: 202,
      body: tracked.body,
    }));
    const result = await sendArticle(request, testSendDeps(fetchFn));
    assert.ok(result.kind === "unexpected");
    assert.equal(result.check, "body-over-limit");
    assert.equal(tracked.cancelled(), true);
    // Bounded reading: ~3 reads to exceed 1,024 bytes in 512-byte chunks,
    // plus at most one prefetch by the stream.
    assert.ok(tracked.pulls() <= 5, `pulled ${String(tracked.pulls())} times`);
  });

  it("treats a null body as empty", async () => {
    const { fetchFn } = recordingFetch(async () => ({
      type: "basic",
      status: 200,
      body: null,
    }));
    const result = await sendArticle(request, testSendDeps(fetchFn));
    assert.ok(result.kind === "unexpected");
    assert.equal(result.check, "json-shape");
  });

  it("returns unknown when the body cannot be read", async () => {
    const cause = new Error("connection lost");
    const { fetchFn } = recordingFetch(async () => ({
      type: "basic",
      status: 200,
      body: failingStream([new TextEncoder().encode("{")], cause),
    }));
    assert.deepEqual(await sendArticle(request, testSendDeps(fetchFn)), {
      kind: "unknown",
      cause,
    });
  });

  it("does not read the body of an opaqueredirect response", async () => {
    const { fetchFn, calls } = recordingFetch(async () => redirectResponse());
    assert.deepEqual(await sendArticle(request, testSendDeps(fetchFn)), {
      kind: "redirected",
    });
    assert.equal(calls.length, 1);
  });
});
