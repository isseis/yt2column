import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { CollectedInput, CollectOutcome } from "../src/core/collect.ts";
import {
  buildArticleBody,
  maxBodyBytes,
  prepareSend,
} from "../src/core/prepare.ts";
import type { PrepareOutcome } from "../src/core/prepare.ts";
import { defaultPort } from "../src/core/settings.ts";
import {
  failingSettings,
  fixedSettings,
  identifiableToken,
} from "./helpers/settings.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";
const validToken = "a".repeat(64);

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
    assert.deepEqual(JSON.parse(body.json), {
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
      assert.deepEqual(JSON.parse(body.json), {
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
    assert.deepEqual(JSON.parse(body.json), {
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
    const parsed = JSON.parse(body.json) as Record<string, unknown>;
    assert.deepEqual(Object.keys(parsed), ["videoUrl", "transcript"]);
    assert.equal("title" in parsed, false);
    assert.equal("description" in parsed, false);
  });

  it("does not include the tab title", () => {
    const body = buildArticleBody(
      collectedInput("x", watchUrl, "SECRET-TAB-TITLE"),
      {},
    );
    assert.equal(body.json.includes("SECRET-TAB-TITLE"), false);
  });

  it("counts the JSON in UTF-8 bytes, not UTF-16 length", () => {
    const body = buildArticleBody(collectedInput("\u3042"), {});
    assert.ok(body.byteLength > body.json.length);
  });
});

describe("prepareSend", () => {
  it("builds a request to the default port when none is stored", async () => {
    const settings = fixedSettings({ token: validToken, port: undefined });
    const outcome = await prepareSend(collected("x"), settings.reader, {});
    assert.ok(outcome.kind === "ready");
    assert.equal(outcome.request.url, "http://127.0.0.1:8765/articles");
    assert.equal(outcome.request.port, defaultPort);
    assert.deepEqual(JSON.parse(outcome.request.body.json), {
      videoUrl: watchUrl,
      transcript: "x",
    });
  });

  it("uses the stored port in the URL", async () => {
    const settings = fixedSettings({ token: validToken, port: "1024" });
    const outcome = await prepareSend(collected("x"), settings.reader, {});
    assert.ok(outcome.kind === "ready");
    assert.equal(outcome.request.url, "http://127.0.0.1:1024/articles");
    assert.equal(outcome.request.port, "1024");
  });

  it("does not read settings when the collection was rejected", async () => {
    const settings = fixedSettings({ token: validToken, port: undefined });
    const outcome: CollectOutcome = {
      kind: "rejected",
      reason: "empty-selection",
    };
    assert.equal(
      blockedReason(await prepareSend(outcome, settings.reader, {})),
      "empty-selection",
    );
    assert.equal(settings.loadCalls(), 0);
  });

  it("passes a collection-failed reason through without its cause", async () => {
    const settings = fixedSettings({ token: validToken, port: undefined });
    const outcome: CollectOutcome = {
      kind: "rejected",
      reason: "collection-failed",
      cause: new Error("boom"),
    };
    const prepared = await prepareSend(outcome, settings.reader, {});
    assert.deepEqual(prepared, {
      kind: "blocked",
      reason: "collection-failed",
    });
  });

  it("blocks with settings-unreadable when loading fails", async () => {
    const cause = new Error("storage failed");
    const settings = failingSettings(cause);
    const outcome = await prepareSend(collected("x"), settings.reader, {});
    assert.deepEqual(outcome, {
      kind: "blocked",
      reason: "settings-unreadable",
      cause,
    });
    assert.equal(settings.loadCalls(), 1);
  });

  it("blocks with token-missing when no token is stored", async () => {
    const settings = fixedSettings({ token: undefined, port: undefined });
    assert.equal(
      blockedReason(await prepareSend(collected("x"), settings.reader, {})),
      "token-missing",
    );
  });

  it("blocks with token-invalid when the stored token has the wrong form", async () => {
    const settings = fixedSettings({ token: "nope", port: undefined });
    assert.equal(
      blockedReason(await prepareSend(collected("x"), settings.reader, {})),
      "token-invalid",
    );
  });

  it("blocks with port-invalid when the stored port has the wrong form", async () => {
    const settings = fixedSettings({ token: validToken, port: "80" });
    assert.equal(
      blockedReason(await prepareSend(collected("x"), settings.reader, {})),
      "port-invalid",
    );
  });

  it("does not put the shared token in the body", async () => {
    const settings = fixedSettings({
      token: identifiableToken,
      port: undefined,
    });
    const outcome = await prepareSend(collected("x"), settings.reader, {});
    assert.ok(outcome.kind === "ready");
    assert.equal(outcome.request.body.json.includes(identifiableToken), false);
  });

  it("sends at exactly the body limit and blocks one byte over", async () => {
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

    const settings = fixedSettings({ token: validToken, port: undefined });
    assert.equal(
      (
        await prepareSend(
          { kind: "collected", input: at(maxBodyBytes) },
          settings.reader,
          {},
        )
      ).kind,
      "ready",
    );
    const over = await prepareSend(
      { kind: "collected", input: at(maxBodyBytes + 1) },
      settings.reader,
      {},
    );
    assert.equal(blockedReason(over), "body-too-large");
  });
});
