import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  countCharacters,
  countLines,
  replaceUnpairedSurrogates,
  truncate,
  truncateTail,
} from "../src/core/summary.ts";
import type { DisplayText } from "../src/core/summary.ts";

// Both cuts share the at-limit and surrogate-pair behaviour; only the side cut
// one over the limit differs, so one table covers the pair.
const cuts: {
  name: string;
  cut: (text: string, limit: number) => DisplayText;
  over: { text: string; value: string };
  pair: { text: string; value: string };
  lone: { text: string; value: string };
}[] = [
  {
    name: "truncate",
    cut: truncate,
    over: { text: "abcde", value: "abcd" },
    pair: { text: `a\u{1F600}b`, value: `a\u{1F600}` },
    lone: { text: "\uDE00\uD83Da", value: "\uDE00\uD83D" },
  },
  {
    name: "truncateTail",
    cut: truncateTail,
    over: { text: "abcde", value: "bcde" },
    pair: { text: `ab\u{1F600}`, value: `b\u{1F600}` },
    lone: { text: "a\uDE00\uD83D", value: "\uDE00\uD83D" },
  },
];
for (const { name, cut, over, pair, lone } of cuts) {
  describe(name, () => {
    it("keeps text at exactly the limit intact", () => {
      assert.deepEqual(cut("abcde", 5), { value: "abcde", truncated: false });
    });

    it("cuts text one over the limit", () => {
      assert.deepEqual(cut(over.text, 4), {
        value: over.value,
        truncated: true,
      });
    });

    it("does not split a surrogate pair at the boundary", () => {
      assert.deepEqual(cut(pair.text, 2), {
        value: pair.value,
        truncated: true,
      });
    });

    it("counts each lone surrogate as one code point", () => {
      // A low surrogate before a high one is not a pair.
      assert.deepEqual(cut(lone.text, 2), {
        value: lone.value,
        truncated: true,
      });
    });
  });
}

describe("countCharacters", () => {
  it("counts a surrogate pair as one character", () => {
    assert.equal(countCharacters("ab\u{1F600}cd"), 5);
  });

  it("counts each lone surrogate as one character", () => {
    assert.equal(countCharacters("\uDE00\uD83Da"), 3);
  });
});

describe("countLines", () => {
  it("counts lines split on \\r\\n, \\r or \\n", () => {
    assert.equal(countLines("a\r\nb\rc\nd"), 4);
  });

  it("counts leading and trailing blank lines", () => {
    assert.equal(countLines("\na\n"), 3);
  });
});

describe("replaceUnpairedSurrogates", () => {
  const cases: { name: string; text: string; expected: string }[] = [
    { name: "keeps a pair", text: "a\u{1F600}b", expected: "a\u{1F600}b" },
    { name: "keeps U+FFFD itself", text: "a\uFFFDb", expected: "a\uFFFDb" },
    { name: "replaces a lone high", text: "a\uD83Db", expected: "a\uFFFDb" },
    { name: "replaces a lone low", text: "a\uDE00b", expected: "a\uFFFDb" },
    {
      name: "replaces one at each end",
      text: "\uDE00a\uD83D",
      expected: "\uFFFDa\uFFFD",
    },
    {
      name: "replaces a low then a high, which are no pair",
      text: "\uDE00\uD83D",
      expected: "\uFFFD\uFFFD",
    },
    {
      name: "keeps a pair after a lone high",
      text: "\uD83D\uD83D\uDE00",
      expected: "\uFFFD\u{1F600}",
    },
  ];
  for (const { name, text, expected } of cases) {
    it(name, () => {
      assert.equal(replaceUnpairedSurrogates(text), expected);
    });
  }
});
