// Parses an untrusted response body into a JSON tree with the same rules as
// the server side's internal/strictjson. JSON.parse would silently accept a
// repeated member name (the last one wins) and an unpaired surrogate escape,
// so the bytes are parsed here instead of checked around JSON.parse. The
// parse rejects rather than repairs: nothing is skipped, trimmed or replaced.

import type { UnexpectedCheck } from "./exchange.ts";

/** A parsed JSON value. Numbers keep their source text; nothing is converted here. */
export type JsonValue =
  | {
      readonly kind: "object";
      readonly members: ReadonlyMap<string, JsonValue>;
    }
  | { readonly kind: "array"; readonly items: readonly JsonValue[] }
  | { readonly kind: "string"; readonly value: string }
  | { readonly kind: "number"; readonly text: string }
  | { readonly kind: "boolean"; readonly value: boolean }
  | { readonly kind: "null" };

/** Accepts T only when every member of it is also a member of U. */
type Subset<T extends U, U> = T;

/**
 * Why the bytes are not one strict JSON value; for the developer log only.
 * Each value is also an UnexpectedCheck, so a response check logs it as one.
 */
export type StrictJsonError = Subset<
  | "encoding" // not valid UTF-8
  | "bom" // starts with U+FEFF
  | "syntax" // not JSON
  | "duplicate-member" // a name repeated in one object, at any depth
  | "unpaired-surrogate" // a \uD800-\uDFFF escape without its pair
  | "trailing-data" // anything but whitespace after the value
  | "too-deep", // nesting beyond the limit
  UnexpectedCheck
>;

export type StrictJsonResult =
  | { readonly ok: true; readonly value: JsonValue }
  | { readonly ok: false; readonly error: StrictJsonError };

/**
 * The deepest nesting of objects and arrays accepted. A valid response nests
 * four deep; the bound keeps a hostile body from exhausting the stack.
 */
export const maxJsonDepth = 16;

/** Thrown inside the parser only; parseStrictJson turns it into a result. */
class Rejected {
  readonly error: StrictJsonError;

  constructor(error: StrictJsonError) {
    this.error = error;
  }
}

const numberPattern = /-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/y;
const hexPattern = /^[0-9a-fA-F]{4}$/;

/** The character each single-character escape stands for. */
const simpleEscapes: ReadonlyMap<string, string> = new Map([
  ['"', '"'],
  ["\\", "\\"],
  ["/", "/"],
  ["b", "\b"],
  ["f", "\f"],
  ["n", "\n"],
  ["r", "\r"],
  ["t", "\t"],
]);

/** A recursive-descent reader over the decoded text, one value at a time. */
class Reader {
  private readonly text: string;
  private pos = 0;

  constructor(text: string) {
    this.text = text;
  }

  /** Reads the one top-level value and requires only whitespace after it. */
  document(): JsonValue {
    this.skipWhitespace();
    const value = this.value(0);
    this.skipWhitespace();
    if (this.pos !== this.text.length) {
      throw new Rejected("trailing-data");
    }
    return value;
  }

  /** Skips JSON whitespace: space, tab, line feed and carriage return only. */
  private skipWhitespace(): void {
    while (this.pos < this.text.length) {
      const c = this.text[this.pos];
      if (c !== " " && c !== "\t" && c !== "\n" && c !== "\r") {
        return;
      }
      this.pos += 1;
    }
  }

  /** Reads a value whose enclosing containers number depth. */
  private value(depth: number): JsonValue {
    const c = this.text[this.pos];
    switch (c) {
      case "{":
        return this.object(depth + 1);
      case "[":
        return this.array(depth + 1);
      case '"':
        return { kind: "string", value: this.string() };
      case "t":
        this.literal("true");
        return { kind: "boolean", value: true };
      case "f":
        this.literal("false");
        return { kind: "boolean", value: false };
      case "n":
        this.literal("null");
        return { kind: "null" };
      default:
        return { kind: "number", text: this.number() };
    }
  }

  private object(depth: number): JsonValue {
    if (depth > maxJsonDepth) {
      throw new Rejected("too-deep");
    }
    this.pos += 1;
    const members = new Map<string, JsonValue>();
    this.skipWhitespace();
    if (this.text[this.pos] === "}") {
      this.pos += 1;
      return { kind: "object", members };
    }
    for (;;) {
      this.skipWhitespace();
      if (this.text[this.pos] !== '"') {
        throw new Rejected("syntax");
      }
      const name = this.string();
      this.skipWhitespace();
      this.expect(":");
      this.skipWhitespace();
      const value = this.value(depth);
      // Names are compared after their escapes are decoded, so "a" and
      // "a" are the same member.
      if (members.has(name)) {
        throw new Rejected("duplicate-member");
      }
      members.set(name, value);
      this.skipWhitespace();
      if (this.text[this.pos] === ",") {
        this.pos += 1;
        continue;
      }
      this.expect("}");
      return { kind: "object", members };
    }
  }

  private array(depth: number): JsonValue {
    if (depth > maxJsonDepth) {
      throw new Rejected("too-deep");
    }
    this.pos += 1;
    const items: JsonValue[] = [];
    this.skipWhitespace();
    if (this.text[this.pos] === "]") {
      this.pos += 1;
      return { kind: "array", items };
    }
    for (;;) {
      this.skipWhitespace();
      items.push(this.value(depth));
      this.skipWhitespace();
      if (this.text[this.pos] === ",") {
        this.pos += 1;
        continue;
      }
      this.expect("]");
      return { kind: "array", items };
    }
  }

  /** Reads a string starting at its opening quote and returns its decoded value. */
  private string(): string {
    this.pos += 1;
    let result = "";
    let runStart = this.pos;
    for (;;) {
      if (this.pos >= this.text.length) {
        throw new Rejected("syntax");
      }
      const code = this.text.charCodeAt(this.pos);
      if (code === 0x22) {
        result += this.text.slice(runStart, this.pos);
        this.pos += 1;
        return result;
      }
      if (code < 0x20) {
        throw new Rejected("syntax");
      }
      if (code !== 0x5c) {
        this.pos += 1;
        continue;
      }
      result += this.text.slice(runStart, this.pos);
      result += this.escape();
      runStart = this.pos;
    }
  }

  /** Reads one escape starting at its backslash; a high surrogate takes its low pair with it. */
  private escape(): string {
    const c = this.text[this.pos + 1];
    if (c === undefined) {
      throw new Rejected("syntax");
    }
    if (c !== "u") {
      const decoded = simpleEscapes.get(c);
      if (decoded === undefined) {
        throw new Rejected("syntax");
      }
      this.pos += 2;
      return decoded;
    }
    const unit = this.unicodeEscape();
    if (unit >= 0xdc00 && unit <= 0xdfff) {
      throw new Rejected("unpaired-surrogate");
    }
    if (unit < 0xd800 || unit > 0xdbff) {
      return String.fromCharCode(unit);
    }
    if (this.text[this.pos] !== "\\" || this.text[this.pos + 1] !== "u") {
      throw new Rejected("unpaired-surrogate");
    }
    const low = this.unicodeEscape();
    if (low < 0xdc00 || low > 0xdfff) {
      throw new Rejected("unpaired-surrogate");
    }
    return String.fromCharCode(unit, low);
  }

  /** Reads \uXXXX at the current position and returns its code unit. */
  private unicodeEscape(): number {
    const hex = this.text.slice(this.pos + 2, this.pos + 6);
    if (!hexPattern.test(hex)) {
      throw new Rejected("syntax");
    }
    this.pos += 6;
    return Number.parseInt(hex, 16);
  }

  /** Reads a number in the JSON grammar and returns its source text. */
  private number(): string {
    numberPattern.lastIndex = this.pos;
    const match = numberPattern.exec(this.text);
    if (match === null) {
      throw new Rejected("syntax");
    }
    this.pos += match[0].length;
    return match[0];
  }

  private literal(word: string): void {
    if (!this.text.startsWith(word, this.pos)) {
      throw new Rejected("syntax");
    }
    this.pos += word.length;
  }

  private expect(c: string): void {
    if (this.text[this.pos] !== c) {
      throw new Rejected("syntax");
    }
    this.pos += 1;
  }
}

/**
 * Parses bytes as exactly one JSON value under the strict rules. Never throws:
 * every rejection is a result naming the rule the bytes broke.
 */
export function parseStrictJson(bytes: Uint8Array): StrictJsonResult {
  let text: string;
  try {
    // ignoreBOM keeps a leading U+FEFF in the text, so it is rejected below
    // rather than silently removed.
    text = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(
      bytes,
    );
  } catch {
    return { ok: false, error: "encoding" };
  }
  if (text.startsWith("\uFEFF")) {
    return { ok: false, error: "bom" };
  }
  try {
    return { ok: true, value: new Reader(text).document() };
  } catch (cause) {
    if (cause instanceof Rejected) {
      return { ok: false, error: cause.error };
    }
    throw cause;
  }
}
