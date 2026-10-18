import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  acceptMetadata,
  emptyMetadataResult,
  parsePageMetadata,
  readFailureResult,
} from "../src/core/metadata.ts";
import type {
  MetadataCheck,
  MetadataRead,
  PageMetadata,
} from "../src/core/metadata.ts";

/** Asserts that the read was accepted and returns its page. */
function accepted(read: MetadataRead): PageMetadata {
  assert.ok(read.kind === "accepted", `expected acceptance, got ${read.kind}`);
  return read.page;
}

describe("parsePageMetadata", () => {
  it("accepts the full member set and keeps every value verbatim", () => {
    const page = {
      title: "  A title\n",
      channelName: "　Channel　",
      description: "Line one\nLine two 😀",
      descriptionCollapsed: true,
    };
    assert.deepEqual(accepted(parsePageMetadata(JSON.stringify(page))), page);
  });

  it("accepts a result with only descriptionCollapsed", () => {
    assert.deepEqual(
      accepted(parsePageMetadata('{"descriptionCollapsed":false}')),
      { descriptionCollapsed: false },
    );
  });

  it("accepts values whose text looks like JSON structure", () => {
    // Each value holds quotes, a backslash, braces, commas, and a member name,
    // so a scan that loses track of strings or escapes sees a duplicate.
    const page = {
      title: 'a","title":"b',
      channelName: "ends with a backslash\\",
      description: '{"description":"x"},[\\"channelName\\"]',
      descriptionCollapsed: false,
    };
    assert.deepEqual(accepted(parsePageMetadata(JSON.stringify(page))), page);
  });

  it("decodes a lone surrogate escape for acceptMetadata to judge", () => {
    const page = accepted(
      parsePageMetadata('{"title":"\\ud800","descriptionCollapsed":false}'),
    );
    assert.equal(page.title, "\uD800");
  });

  const rejected: [string, string, MetadataCheck][] = [
    ["invalid JSON", "{title", "json"],
    ["empty text", "", "json"],
    ["two values", "{} {}", "json"],
    ["an array", "[]", "top-level"],
    ["null", "null", "top-level"],
    ["a number", "1", "top-level"],
    ["a string", '"text"', "top-level"],
    ["a boolean", "true", "top-level"],
    [
      "an unknown member",
      '{"descriptionCollapsed":false,"duration":"1:00"}',
      "member-set",
    ],
    [
      "an inherited-looking member",
      '{"descriptionCollapsed":false,"__proto__":"x"}',
      "member-set",
    ],
    ["a missing descriptionCollapsed", '{"title":"a"}', "member-set"],
    ["an empty object", "{}", "member-set"],
    [
      "a duplicate value member",
      '{"title":"a","title":"b","descriptionCollapsed":false}',
      "duplicate-member",
    ],
    [
      "a duplicate spelled with an escape",
      '{"title":"a","\\u0074itle":"b","descriptionCollapsed":false}',
      "duplicate-member",
    ],
    [
      "a duplicate descriptionCollapsed",
      '{"descriptionCollapsed":true,"descriptionCollapsed":false}',
      "duplicate-member",
    ],
    [
      "a duplicate after whitespace",
      '{ "channelName" : "a" ,\n "channelName" : "b", "descriptionCollapsed": false }',
      "duplicate-member",
    ],
    [
      // The nested names repeat top-level names; only the value type is wrong.
      "a nested object reusing member names",
      '{"title":{"title":"x","descriptionCollapsed":true},"descriptionCollapsed":false}',
      "value-type",
    ],
    [
      "a nested array holding a member name",
      '{"title":["x","title"],"descriptionCollapsed":false}',
      "value-type",
    ],
    [
      "a null title",
      '{"title":null,"descriptionCollapsed":false}',
      "value-type",
    ],
    [
      "a number channelName",
      '{"channelName":1,"descriptionCollapsed":false}',
      "value-type",
    ],
    [
      "a boolean description",
      '{"description":true,"descriptionCollapsed":false}',
      "value-type",
    ],
    [
      "an object description",
      '{"description":{},"descriptionCollapsed":false}',
      "value-type",
    ],
    [
      "an array channelName",
      '{"channelName":[],"descriptionCollapsed":false}',
      "value-type",
    ],
    [
      "a string descriptionCollapsed",
      '{"descriptionCollapsed":"true"}',
      "value-type",
    ],
    [
      "a null descriptionCollapsed",
      '{"descriptionCollapsed":null}',
      "value-type",
    ],
    [
      "a number descriptionCollapsed",
      '{"descriptionCollapsed":0}',
      "value-type",
    ],
  ];
  for (const [name, text, check] of rejected) {
    it(`rejects ${name} with check ${check}`, () => {
      assert.deepEqual(parsePageMetadata(text), { kind: "rejected", check });
    });
  }
});

describe("acceptMetadata", () => {
  it("keeps every value unchanged when all are acceptable", () => {
    const page: PageMetadata = {
      title: " 　Title with spaces　 ",
      channelName: "\nChannel\n",
      description: "Smile 😀 and <img src=x onerror=alert(1)>",
      descriptionCollapsed: false,
    };
    assert.deepEqual(acceptMetadata(page), {
      values: {
        title: page.title,
        channelName: page.channelName,
        description: page.description,
      },
      showExpandPrompt: false,
      omissions: [],
    });
  });

  const omitted: [string, string | undefined, string][] = [
    ["an absent value", undefined, "absent"],
    ["an empty value", "", "blank"],
    ["spaces only", "  ", "blank"],
    ["an ideographic space only", "　", "blank"],
    ["line breaks and tabs only", "\n\t\r\n", "blank"],
    ["a next-line character only", "\u0085", "blank"],
    ["a lone high surrogate", "\uD800", "unpaired-surrogate"],
    ["a lone low surrogate between text", "a\uDC00b", "unpaired-surrogate"],
    ["a reversed surrogate pair", "\uDE00\uD83D", "unpaired-surrogate"],
  ];
  for (const field of ["title", "channelName", "description"] as const) {
    for (const [name, value, reason] of omitted) {
      it(`omits ${field} for ${name} with reason ${reason}`, () => {
        const page: { -readonly [K in keyof PageMetadata]: PageMetadata[K] } = {
          title: "T",
          channelName: "C",
          description: "D",
          descriptionCollapsed: false,
        };
        if (value === undefined) {
          delete page[field];
        } else {
          page[field] = value;
        }
        const result = acceptMetadata(page);
        assert.deepEqual(result.omissions, [{ field, reason }]);
        assert.equal(Object.hasOwn(result.values, field), false);
        const kept: Record<string, string> = {
          title: "T",
          channelName: "C",
          description: "D",
        };
        delete kept[field];
        assert.deepEqual(result.values, kept);
      });
    }
  }

  it("lists every omission in field order", () => {
    const result = acceptMetadata({
      title: "\uD800",
      description: " ",
      descriptionCollapsed: true,
    });
    assert.deepEqual(result.omissions, [
      { field: "title", reason: "unpaired-surrogate" },
      { field: "channelName", reason: "absent" },
      { field: "description", reason: "blank" },
    ]);
    assert.deepEqual(result.values, {});
  });

  it("keeps a large value without truncating or omitting it", () => {
    // Larger than the 4 MiB body limit: the size is judged only when the body
    // is built, never here.
    const description = "x".repeat(5 * 1024 * 1024);
    const result = acceptMetadata({ description, descriptionCollapsed: false });
    assert.equal(result.values.description, description);
    assert.equal(
      result.omissions.some((omission) => omission.field === "description"),
      false,
    );
  });

  const prompts: [string, PageMetadata, boolean][] = [
    [
      "a collapsed, accepted description",
      { description: "Beginning", descriptionCollapsed: true },
      true,
    ],
    [
      "an expanded description",
      { description: "Full text", descriptionCollapsed: false },
      false,
    ],
    ["a collapsed, absent description", { descriptionCollapsed: true }, false],
    [
      "a collapsed, blank description",
      { description: "　", descriptionCollapsed: true },
      false,
    ],
    [
      "a collapsed description with an unpaired surrogate",
      { description: "a\uD800", descriptionCollapsed: true },
      false,
    ],
  ];
  for (const [name, page, expected] of prompts) {
    it(`sets showExpandPrompt to ${String(expected)} for ${name}`, () => {
      assert.equal(acceptMetadata(page).showExpandPrompt, expected);
    });
  }
});

describe("readFailureResult", () => {
  const failures = [
    { field: "title", reason: "read-failure" },
    { field: "channelName", reason: "read-failure" },
    { field: "description", reason: "read-failure" },
  ];

  it("omits every field as a read failure with no check", () => {
    const result = readFailureResult();
    assert.deepEqual(result, {
      values: {},
      showExpandPrompt: false,
      omissions: failures,
    });
    assert.equal(Object.hasOwn(result, "check"), false);
  });

  it("carries the failed check", () => {
    assert.deepEqual(readFailureResult("duplicate-member"), {
      values: {},
      showExpandPrompt: false,
      omissions: failures,
      check: "duplicate-member",
    });
  });
});

describe("emptyMetadataResult", () => {
  it("has no values, no prompt, no omissions, and no check", () => {
    assert.deepEqual(emptyMetadataResult(), {
      values: {},
      showExpandPrompt: false,
      omissions: [],
    });
  });
});
