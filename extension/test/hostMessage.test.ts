import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { ArticleKey } from "../src/core/articleKey.ts";
import {
  publishMessage,
  startMessage,
  statusMessage,
} from "../src/core/hostMessage.ts";
import { prepareStartInput } from "../src/core/prepare.ts";

// Tests may assert a branded type to build fixtures.
const key = "0123456789abcdef0123456789abcdef" as ArticleKey;

describe("host messages", () => {
  it("builds the generation start from a checked request's input", () => {
    const prepared = prepareStartInput({
      videoUrl: "https://www.youtube.com/watch?v=abc",
      transcript: "hello",
      title: "Video",
    });
    assert.ok(prepared.kind === "ready");
    assert.deepEqual(startMessage(prepared.request), {
      v: 1,
      op: "start",
      input: {
        videoUrl: "https://www.youtube.com/watch?v=abc",
        transcript: "hello",
        title: "Video",
      },
    });
  });

  it("builds the status query for a key", () => {
    assert.deepEqual(statusMessage(key), {
      v: 1,
      op: "status",
      key: "0123456789abcdef0123456789abcdef",
    });
  });

  it("builds the publish operation for a key", () => {
    assert.deepEqual(publishMessage(key), {
      v: 1,
      op: "publish",
      key: "0123456789abcdef0123456789abcdef",
    });
  });
});
