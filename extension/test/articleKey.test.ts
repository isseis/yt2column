import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { maxKeyLength, parseArticleKey } from "../src/core/articleKey.ts";

describe("parseArticleKey", () => {
  const accepted: { name: string; value: string }[] = [
    { name: "a one-character key", value: "k" },
    // Counted in code points: each U+1F600 is two UTF-16 units.
    {
      name: "a key of exactly the limit in code points",
      value: "\u{1F600}".repeat(maxKeyLength),
    },
  ];
  for (const { name, value } of accepted) {
    it(`returns ${name} unchanged`, () => {
      assert.equal(parseArticleKey(value), value);
    });
  }

  const rejected: { name: string; value: unknown }[] = [
    { name: "an empty string", value: "" },
    {
      name: "a key one code point over the limit",
      value: "k".repeat(maxKeyLength + 1),
    },
    { name: "a number", value: 1 },
    { name: "null", value: null },
    { name: "an array of one string", value: ["k"] },
  ];
  for (const { name, value } of rejected) {
    it(`rejects ${name}`, () => {
      assert.equal(parseArticleKey(value), undefined);
    });
  }
});
