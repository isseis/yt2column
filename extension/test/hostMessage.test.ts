import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { ArticleKey } from "../src/core/articleKey.ts";
import { startMessage, statusMessage } from "../src/core/hostMessage.ts";
import { prepareStartInput } from "../src/core/prepare.ts";

// Tests may assert a branded type to build fixtures.
const key = "0123456789abcdef0123456789abcdef" as ArticleKey;

describe("host messages", () => {
  for (const mode of ["inquiry", "plain"] as const) {
    it(`builds the generation start from a checked request's input with mode ${mode}`, () => {
      const prepared = prepareStartInput({
        videoUrl: "https://www.youtube.com/watch?v=abc",
        transcript: "hello",
        title: "Video",
      });
      assert.ok(prepared.kind === "ready");
      assert.deepEqual(startMessage(prepared.request, mode), {
        v: 1,
        op: "start",
        input: {
          videoUrl: "https://www.youtube.com/watch?v=abc",
          transcript: "hello",
          title: "Video",
          mode,
        },
      });
    });
  }

  it("builds the status message for a key", () => {
    assert.deepEqual(statusMessage(key), {
      v: 1,
      op: "status",
      key: "0123456789abcdef0123456789abcdef",
    });
  });
});
