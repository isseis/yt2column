import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { maxJsonDepth, parseStrictJson } from "../src/core/strictJson.ts";
import type {
  JsonValue,
  StrictJsonError,
  StrictJsonResult,
} from "../src/core/strictJson.ts";

/** The UTF-8 bytes of text. */
function utf8(text: string): Uint8Array {
  return new TextEncoder().encode(text);
}

/** The error of a rejected result, asserting it was rejected. */
function errorOf(result: StrictJsonResult): StrictJsonError {
  assert.ok(!result.ok, JSON.stringify(result));
  return result.error;
}

/** The value of an accepted result, asserting it was accepted. */
function valueOf(result: StrictJsonResult): JsonValue {
  assert.ok(result.ok, JSON.stringify(result));
  return result.value;
}

interface Case {
  readonly name: string;
  readonly bytes: Uint8Array;
}

interface RejectedCase extends Case {
  readonly error: StrictJsonError;
}

/** Declares one test per case, asserting the parse rejects it with the error. */
function expectRejected(cases: readonly RejectedCase[]): void {
  for (const { name, bytes, error } of cases) {
    it(`rejects ${name} as ${error}`, () => {
      assert.equal(errorOf(parseStrictJson(bytes)), error);
    });
  }
}

/** Declares one test per case, asserting the parse accepts it. */
function expectAccepted(cases: readonly Case[]): void {
  for (const { name, bytes } of cases) {
    it(`accepts ${name}`, () => {
      valueOf(parseStrictJson(bytes));
    });
  }
}

/** depth nested arrays around an empty innermost one. */
function nestedArrays(depth: number): Uint8Array {
  return utf8("[".repeat(depth) + "]".repeat(depth));
}

/** depth nested objects around an empty innermost one. */
function nestedObjects(depth: number): Uint8Array {
  return utf8('{"a":'.repeat(depth - 1) + "{}" + "}".repeat(depth - 1));
}

describe("parseStrictJson", () => {
  it("builds the tree with every value kind and keeps number text", () => {
    const value = valueOf(
      parseStrictJson(
        utf8('{"o":{"n":1.0},"a":[-2.5e+3,"s",true,false,null],"e":{}}'),
      ),
    );
    assert.deepEqual(value, {
      kind: "object",
      members: new Map<string, JsonValue>([
        [
          "o",
          {
            kind: "object",
            members: new Map([["n", { kind: "number", text: "1.0" }]]),
          },
        ],
        [
          "a",
          {
            kind: "array",
            items: [
              { kind: "number", text: "-2.5e+3" },
              { kind: "string", value: "s" },
              { kind: "boolean", value: true },
              { kind: "boolean", value: false },
              { kind: "null" },
            ],
          },
        ],
        ["e", { kind: "object", members: new Map() }],
      ]),
    });
  });

  // The rows of internal/strictjson's TestParseObject reject table that are
  // not about the top-level kind, with the same bytes. The top-level rows
  // (null, array, string, number) are valid JSON values here; the response
  // checks reject them.
  describe("the server side's rejected inputs", () => {
    expectRejected([
      {
        name: "invalid utf8",
        bytes: new Uint8Array([...utf8('{"a":"'), 0xff, ...utf8('"}')]),
        error: "encoding",
      },
      {
        name: "unpaired high",
        bytes: utf8('{"a":"\\ud800"}'),
        error: "unpaired-surrogate",
      },
      {
        name: "unpaired low",
        bytes: utf8('{"a":"\\udc00"}'),
        error: "unpaired-surrogate",
      },
      {
        name: "unpaired in ignored",
        bytes: utf8('{"a":"ok","b":"\\ud800"}'),
        error: "unpaired-surrogate",
      },
      {
        name: "trailing object",
        bytes: utf8('{"a":1} {}'),
        error: "trailing-data",
      },
      {
        name: "trailing text",
        bytes: utf8('{"a":1} x'),
        error: "trailing-data",
      },
      {
        name: "trailing char",
        bytes: utf8('{"a":1}x'),
        error: "trailing-data",
      },
      { name: "empty", bytes: new Uint8Array(), error: "syntax" },
      { name: "truncated", bytes: utf8("{"), error: "syntax" },
    ]);
  });

  describe("encoding", () => {
    expectRejected([
      {
        name: "an overlong encoding of /",
        bytes: new Uint8Array([...utf8('{"a":"'), 0xc0, 0xaf, ...utf8('"}')]),
        error: "encoding",
      },
      {
        name: "an encoded surrogate",
        bytes: new Uint8Array([
          ...utf8('{"a":"'),
          0xed,
          0xa0,
          0x80,
          ...utf8('"}'),
        ]),
        error: "encoding",
      },
      {
        name: "a leading byte-order mark",
        bytes: new Uint8Array([0xef, 0xbb, 0xbf, ...utf8("{}")]),
        error: "bom",
      },
    ]);
    expectAccepted([
      { name: "multibyte characters", bytes: utf8('{"a":"\u3042\u{1F600}"}') },
      { name: "U+FEFF inside a string", bytes: utf8('{"a":"\uFEFF"}') },
    ]);
  });

  describe("top level", () => {
    expectRejected([
      {
        name: "a no-break space after the value",
        bytes: utf8("{}\u00A0"),
        error: "trailing-data",
      },
      {
        name: "a second value after whitespace",
        bytes: utf8("1 2"),
        error: "trailing-data",
      },
    ]);
    expectAccepted([
      {
        name: "JSON whitespace around the value",
        bytes: utf8(" \t\r\n{} \t\r\n"),
      },
      { name: "a value that is not an object", bytes: utf8("[]") },
    ]);
  });

  describe("objects", () => {
    expectRejected([
      {
        name: "a repeated member",
        bytes: utf8('{"a":1,"a":1}'),
        error: "duplicate-member",
      },
      {
        name: "a repeated member in a nested object",
        bytes: utf8('{"x":{"a":1,"a":2}}'),
        error: "duplicate-member",
      },
      {
        name: "a repeated member in an object in an array",
        bytes: utf8('[{"a":1,"a":2}]'),
        error: "duplicate-member",
      },
      {
        name: "a member repeated through an escape",
        bytes: utf8('{"a":1,"\\u0061":2}'),
        error: "duplicate-member",
      },
      {
        name: "a member without a value",
        bytes: utf8('{"a"}'),
        error: "syntax",
      },
      { name: "a trailing comma", bytes: utf8('{"a":1,}'), error: "syntax" },
      { name: "an unquoted name", bytes: utf8("{a:1}"), error: "syntax" },
    ]);
    expectAccepted([
      {
        name: "the same name in different objects",
        bytes: utf8('{"a":{"a":1}}'),
      },
      { name: "names that differ only in case", bytes: utf8('{"a":1,"A":2}') },
    ]);
  });

  describe("strings", () => {
    it("decodes the eight escapes and \\u escapes", () => {
      const value = valueOf(
        parseStrictJson(
          utf8('"\\"\\\\\\/\\b\\f\\n\\r\\t\\u0041\\ud83d\\ude00"'),
        ),
      );
      assert.deepEqual(value, {
        kind: "string",
        value: '"\\/\b\f\n\r\tA\u{1F600}',
      });
    });

    expectRejected([
      { name: "an unknown escape", bytes: utf8('"\\x41"'), error: "syntax" },
      { name: "an escaped quote mark", bytes: utf8('"\\\'"'), error: "syntax" },
      { name: "a short \\u escape", bytes: utf8('"\\u12"'), error: "syntax" },
      {
        name: "a raw control character",
        bytes: utf8('"a\u0001b"'),
        error: "syntax",
      },
      { name: "a raw line feed", bytes: utf8('"a\nb"'), error: "syntax" },
      {
        name: "a high surrogate followed by a non-surrogate escape",
        bytes: utf8('"\\ud800\\u0041"'),
        error: "unpaired-surrogate",
      },
      {
        name: "a high surrogate followed by text",
        bytes: utf8('"\\ud800a"'),
        error: "unpaired-surrogate",
      },
      {
        name: "two high surrogates",
        bytes: utf8('"\\ud800\\ud800"'),
        error: "unpaired-surrogate",
      },
      { name: "an unterminated string", bytes: utf8('"abc'), error: "syntax" },
    ]);
  });

  describe("numbers", () => {
    expectRejected([
      { name: "a leading plus", bytes: utf8("[+1]"), error: "syntax" },
      { name: "a leading zero", bytes: utf8("[01]"), error: "syntax" },
      { name: "a bare fraction", bytes: utf8("[.5]"), error: "syntax" },
      { name: "a trailing point", bytes: utf8("[1.]"), error: "syntax" },
      { name: "NaN", bytes: utf8("[NaN]"), error: "syntax" },
    ]);
    expectAccepted([
      { name: "zero", bytes: utf8("0") },
      { name: "an exponent without a fraction", bytes: utf8("[1E9]") },
    ]);
  });

  describe("depth", () => {
    expectAccepted([
      {
        name: `${maxJsonDepth} nested arrays`,
        bytes: nestedArrays(maxJsonDepth),
      },
      {
        name: `${maxJsonDepth} nested objects`,
        bytes: nestedObjects(maxJsonDepth),
      },
    ]);
    expectRejected([
      {
        name: `${maxJsonDepth + 1} nested arrays`,
        bytes: nestedArrays(maxJsonDepth + 1),
        error: "too-deep",
      },
      {
        name: `${maxJsonDepth + 1} nested objects`,
        bytes: nestedObjects(maxJsonDepth + 1),
        error: "too-deep",
      },
    ]);
  });
});
