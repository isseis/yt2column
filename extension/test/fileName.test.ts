import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { articleFileName, maxBaseBytes } from "../src/core/fileName.ts";

const videoId = "abc";

describe("articleFileName", () => {
  const cases: { name: string; title: string | undefined; file: string }[] = [
    { name: "a plain title", title: "Title", file: "Title.md" },
    {
      name: "a title with a path and a colon",
      title: "a/b:c",
      file: "a_b_c.md",
    },
    { name: "a backslash", title: "a\\b", file: "a_b.md" },
    {
      name: "the characters Windows refuses",
      title: 'a*b?c"d<e>f|g',
      file: "a_b_c_d_e_f_g.md",
    },
    {
      name: "C0 controls and DEL",
      title: "a\u0000b\tc\nd\u001fe\u007ff",
      file: "a_b_c_d_e_f.md",
    },
    { name: "C1 controls", title: "a\u0080b\u009fc", file: "a_b_c.md" },
    {
      name: "the direction characters",
      title: "a\u061cb\u200ec\u200fd\u202ae\u202ef\u2066g\u2069h",
      file: "a_b_c_d_e_f_g_h.md",
    },
    {
      name: "the zero-width characters",
      title: "a\u200bb\u200cc\u200dd\ufeffe",
      file: "a_b_c_d_e.md",
    },
    {
      name: "the characters next to the replaced ranges",
      title: "\u00a0\u061b\u061d\u200a\u2010\u2029\u202f\u2065\u206a\ufefe",
      file: "\u00a0\u061b\u061d\u200a\u2010\u2029\u202f\u2065\u206a\ufefe.md",
    },
    { name: "leading dots", title: "..hidden", file: "__hidden.md" },
    { name: "dots inside and at the end", title: "a.b.", file: "a.b..md" },
    { name: "a title of only dots", title: "...", file: "___.md" },
    { name: "a slash after leading dots", title: "./x", file: "__x.md" },
    { name: "no title", title: undefined, file: "abc.md" },
    { name: "an empty title", title: "", file: "abc.md" },
    {
      name: "a title of only white space",
      title: " \t\u3000\n",
      file: "abc.md",
    },
    {
      name: "a title with white space around it",
      title: " a ",
      file: " a .md",
    },
    {
      name: "ASCII of exactly the limit",
      title: "a".repeat(200),
      file: `${"a".repeat(200)}.md`,
    },
    {
      name: "ASCII one byte over the limit",
      title: "a".repeat(201),
      file: `${"a".repeat(200)}.md`,
    },
    // U+3042 is three UTF-8 bytes: 66 of them are 198 bytes, a 67th would be 201.
    {
      name: "three-byte characters over the limit",
      title: "\u3042".repeat(67),
      file: `${"\u3042".repeat(66)}.md`,
    },
    // 199 bytes, and the next code point would end past the limit.
    {
      name: "a code point across the limit",
      title: `${"a".repeat(199)}\u3042`,
      file: `${"a".repeat(199)}.md`,
    },
    // U+1F600 is four bytes and two UTF-16 units: 50 of them are 200 bytes.
    {
      name: "four-byte characters at the limit",
      title: "\u{1F600}".repeat(51),
      file: `${"\u{1F600}".repeat(50)}.md`,
    },
    // Replacement comes first: a replaced three-byte character counts as one byte.
    {
      name: "replaced characters before the cut",
      title: "\u200b".repeat(201),
      file: `${"_".repeat(200)}.md`,
    },
  ];
  for (const { name, title, file } of cases) {
    it(`names ${name}`, () => {
      assert.equal(articleFileName(title, videoId), file);
    });
  }

  it("applies the same rules to the video ID", () => {
    assert.equal(articleFileName(undefined, ".a/b"), "_a_b.md");
  });

  it("never yields a separator, a control, direction or zero-width character, a leading dot, or another extension", () => {
    // Inputs no exact row covers: every BMP code point (surrogates aside)
    // after leading dots, a few astral ones, and video IDs a URL may decode to.
    const titles: (string | undefined)[] = [];
    for (let code = 0; code <= 0xffff; code += 1) {
      if (code < 0xd800 || code > 0xdfff) {
        titles.push(`..${String.fromCodePoint(code)}x`);
      }
    }
    titles.push("\u{1F600}\u{E0001}\u{10FFFF}", "...", ". .", undefined);
    const ids = ["..", "/", "\\", "‮", "a\u0000"];
    const forbidden = (code: number) =>
      code <= 0x1f ||
      (code >= 0x7f && code <= 0x9f) ||
      code === 0x061c ||
      (code >= 0x200b && code <= 0x200f) ||
      (code >= 0x202a && code <= 0x202e) ||
      (code >= 0x2066 && code <= 0x2069) ||
      code === 0xfeff;
    const inputs = [
      ...titles.map((title) => [title, videoId] as const),
      ...ids.map((id) => [undefined, id] as const),
    ];
    for (const [title, id] of inputs) {
      const file = articleFileName(title, id);
      for (const point of file) {
        const code = point.codePointAt(0) ?? 0;
        assert.ok(!forbidden(code), JSON.stringify(file));
        assert.ok(!'/\\:*?"<>|'.includes(point), JSON.stringify(file));
      }
      assert.equal(file.startsWith("."), false, JSON.stringify(file));
      assert.equal(file.endsWith(".md"), true, JSON.stringify(file));
      assert.ok(
        new TextEncoder().encode(file).length <= maxBaseBytes + ".md".length,
        JSON.stringify(file),
      );
    }
  });
});
