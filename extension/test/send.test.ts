import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { CollectedInput } from "../src/core/collect.ts";
import type { NativeReply } from "../src/core/native.ts";
import { buildArticleBody } from "../src/core/prepare.ts";
import type { PreparedRequest } from "../src/core/prepare.ts";
import type { RequestId } from "../src/core/response.ts";
import { sendArticle, sendTimeoutMs } from "../src/send.ts";
import {
  deferredNative,
  manualSchedule,
  recordingNative,
  responseReply,
  testSendDeps,
} from "./helpers/native.ts";

const requestId = "0123456789abcdef" as RequestId;
const accepted = JSON.stringify({ status: "accepted", requestId });

// Tests may assert a branded type to build fixtures.
const request: PreparedRequest = {
  body: buildArticleBody(
    {
      selection: "hello",
      url: "https://www.youtube.com/watch?v=abc",
      title: "a tab title",
    } as CollectedInput,
    {},
  ),
} as PreparedRequest;

/** Waits for the queued microtasks and one macrotask to settle. */
async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

describe("sendArticle", () => {
  it("sends the body once, bounds the exchange and cancels the bound", async () => {
    const native = recordingNative(async () => responseReply(202, accepted));
    const timer = manualSchedule();
    const result = await sendArticle(
      request,
      testSendDeps(native.sendNative, timer.schedule),
    );

    assert.deepEqual(result, { kind: "accepted", requestId });
    assert.equal(native.messages.length, 1);
    assert.equal(native.messages[0], request.body.message);
    assert.deepEqual(timer.delays, [sendTimeoutMs]);
    assert.equal(timer.cancelled(), 1);
  });

  it("returns extension-timeout when the bound passes first and ignores a later reply", async () => {
    const native = deferredNative();
    const timer = manualSchedule();
    const pending = sendArticle(
      request,
      testSendDeps(native.sendNative, timer.schedule),
    );
    await flush();
    timer.fire();
    native.resolve(responseReply(202, accepted));
    assert.deepEqual(await pending, {
      kind: "unexpected",
      check: "extension-timeout",
    });
    await flush();
    assert.equal(native.messages.length, 1);
  });

  it("does not resend after a host failure", async () => {
    const native = recordingNative(async (): Promise<NativeReply> => ({
      kind: "failed",
      lastError: "Native host has exited.",
    }));
    assert.deepEqual(
      await sendArticle(request, testSendDeps(native.sendNative)),
      { kind: "host-failed", lastError: "Native host has exited." },
    );
    assert.equal(native.messages.length, 1);
  });

  it("returns host-failed when sendNative rejects against its contract", async () => {
    const native = recordingNative(async () => {
      throw new Error("broken");
    });
    assert.deepEqual(
      await sendArticle(request, testSendDeps(native.sendNative)),
      { kind: "host-failed", lastError: "Error: broken" },
    );
  });

  it("returns host-failed when sendNative throws against its contract", async () => {
    const deps = testSendDeps(() => {
      throw new Error("thrown");
    });
    assert.deepEqual(await sendArticle(request, deps), {
      kind: "host-failed",
      lastError: "Error: thrown",
    });
  });

  it("returns classification-error when the reply throws while it is read", async () => {
    // A getter cannot come from the browser's JSON parse; it stands in for
    // any classifier failure, which must not escape as a thrown error.
    const value = {
      get v(): number {
        throw new Error("getter");
      },
    };
    const native = recordingNative(async (): Promise<NativeReply> => ({
      kind: "message",
      value,
    }));
    assert.deepEqual(
      await sendArticle(request, testSendDeps(native.sendNative)),
      { kind: "unexpected", check: "classification-error" },
    );
  });
});
