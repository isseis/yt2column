import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { CollectedInput, CollectOutcome } from "../src/core/collect.ts";
import {
  buildArticleBody,
  maxBodyBytes,
  prepareSend,
} from "../src/core/prepare.ts";
import type { PrepareOutcome } from "../src/core/prepare.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";

/** Builds a collected input; tests may assert the branded type. */
function collectedInput(
  selection: string,
  url = watchUrl,
  title = "a tab title",
): CollectedInput {
  return { selection, url, title } as CollectedInput;
}

/** A collected outcome from a selection. */
function collected(selection: string): CollectOutcome {
  return { kind: "collected", input: collectedInput(selection) };
}

/** The blocked reason of an outcome, asserting it is blocked. */
function blockedReason(outcome: PrepareOutcome): string {
  assert.ok(outcome.kind === "blocked");
  return outcome.reason;
}

describe("buildArticleBody", () => {
  it("has exactly videoUrl and transcript with the collected values", () => {
    const input = collectedInput("hello\nworld", watchUrl);
    const body = buildArticleBody(input, {});
    assert.deepEqual(body.message, {
      videoUrl: watchUrl,
      transcript: "hello\nworld",
    });
  });

  const roundTrips: { name: string; selection: string; url: string }[] = [
    {
      name: "selection whitespace, line breaks and surrogate pairs",
      selection: "\n \r\n\u3000a\u{1F600}b\n\n",
      url: watchUrl,
    },
    {
      name: "URL parameters and fragment",
      selection: "x",
      url: "https://www.youtube.com/watch?v=abc&t=42&list=PL#frag",
    },
  ];
  for (const { name, selection, url } of roundTrips) {
    it(`preserves the ${name}`, () => {
      const body = buildArticleBody(collectedInput(selection, url), {});
      assert.deepEqual(body.message, {
        videoUrl: url,
        transcript: selection,
      });
    });
  }

  it("includes non-empty metadata members", () => {
    const body = buildArticleBody(collectedInput("x"), {
      title: "Video",
      channelName: "Channel",
      description: "Text",
    });
    assert.deepEqual(body.message, {
      videoUrl: watchUrl,
      transcript: "x",
      title: "Video",
      channelName: "Channel",
      description: "Text",
    });
  });

  it("omits empty and absent metadata members and never writes null", () => {
    const body = buildArticleBody(collectedInput("x"), {
      title: "",
      description: undefined,
    });
    const parsed: Record<string, unknown> = body.message;
    assert.deepEqual(Object.keys(parsed), ["videoUrl", "transcript"]);
    assert.equal("title" in parsed, false);
    assert.equal("description" in parsed, false);
  });

  it("does not include the tab title", () => {
    const body = buildArticleBody(
      collectedInput("x", watchUrl, "SECRET-TAB-TITLE"),
      {},
    );
    assert.equal(
      JSON.stringify(body.message).includes("SECRET-TAB-TITLE"),
      false,
    );
  });

  it("writes markup in a metadata value as a JSON string", () => {
    const markup = "<img src=x onerror=alert(1)>";
    const body = buildArticleBody(collectedInput("x"), {
      title: markup,
      description: markup,
    });
    assert.equal(body.message["title"], markup);
    assert.equal(body.message["description"], markup);
  });

  it("counts the body's JSON text in UTF-8 bytes, not UTF-16 length", () => {
    const body = buildArticleBody(collectedInput("\u3042"), {});
    const json = JSON.stringify(body.message);
    assert.ok(body.byteLength > json.length);
  });
});

describe("prepareSend", () => {
  it("builds the request from the collected values without reading anything else", () => {
    const outcome = prepareSend(collected("x"), {});
    // Synchronous: a send never waits on a stored setting.
    assert.equal(outcome instanceof Promise, false);
    assert.ok(outcome.kind === "ready");
    assert.deepEqual(Object.keys(outcome.request), ["body"]);
    assert.deepEqual(outcome.request.body.message, {
      videoUrl: watchUrl,
      transcript: "x",
    });
  });

  it("passes a collection reason through without its cause", () => {
    const outcome: CollectOutcome = {
      kind: "rejected",
      reason: "collection-failed",
      cause: new Error("boom"),
    };
    assert.deepEqual(prepareSend(outcome, {}), {
      kind: "blocked",
      reason: "collection-failed",
    });
  });

  it("checks the collection before the body size", () => {
    // Metadata alone over the limit: a rejected collection still wins.
    const outcome: CollectOutcome = {
      kind: "rejected",
      reason: "empty-selection",
    };
    const metadata = { description: "d".repeat(maxBodyBytes) };
    assert.equal(
      blockedReason(prepareSend(outcome, metadata)),
      "empty-selection",
    );
  });

  it("counts the metadata toward the body limit", () => {
    const base = buildArticleBody(collectedInput(""), {}).byteLength;
    const input = collectedInput("a".repeat(maxBodyBytes - base));
    assert.equal(prepareSend({ kind: "collected", input }, {}).kind, "ready");
    const withMetadata = prepareSend(
      { kind: "collected", input },
      { description: "d" },
    );
    assert.equal(blockedReason(withMetadata), "body-too-large");
  });

  it("sends at exactly the body limit and blocks one byte over", () => {
    const base = buildArticleBody(collectedInput(""), {}).byteLength;
    const at = (size: number): CollectedInput => {
      const remaining = size - base;
      const multibyte = Math.floor(remaining / 3);
      const ascii = remaining - multibyte * 3;
      return collectedInput("\u3042".repeat(multibyte) + "a".repeat(ascii));
    };
    assert.equal(
      buildArticleBody(at(maxBodyBytes), {}).byteLength,
      maxBodyBytes,
    );
    assert.equal(
      buildArticleBody(at(maxBodyBytes + 1), {}).byteLength,
      maxBodyBytes + 1,
    );

    assert.equal(
      prepareSend({ kind: "collected", input: at(maxBodyBytes) }, {}).kind,
      "ready",
    );
    const over = prepareSend(
      { kind: "collected", input: at(maxBodyBytes + 1) },
      {},
    );
    assert.equal(blockedReason(over), "body-too-large");
  });
});
