// Reads a JSON text whose shape is closed: every object has exactly the
// members its reader names, every code is in its closed set, and nothing is
// repaired. The readers of the article generation module's results
// (wasmResult.ts) and of the job records read back from chrome.storage.session
// (jobRecord.ts) share these steps; a text that fails any of them is refused
// as a whole.

import { parseStrictJson } from "./strictJson.ts";
import { hasUnpairedSurrogate } from "./summary.ts";
import type { JsonValue } from "./strictJson.ts";

/** Thrown by the member readers below; readShaped turns it into undefined. */
export class ShapeMismatch {}

type JsonObject = Extract<JsonValue, { kind: "object" }>;

/** The members of an object read through exactMembers. */
export type Members = JsonObject["members"];

/** True when value is an own key of set; a Record<..., true> holds only own keys. */
export function isIn<K extends string>(
  set: Readonly<Record<K, unknown>>,
  value: string,
): value is K {
  return Object.hasOwn(set, value);
}

/** The members of value, which must be an object with exactly names. */
export function exactMembers(
  value: JsonValue,
  names: readonly string[],
): Members {
  if (value.kind !== "object") {
    throw new ShapeMismatch();
  }
  const members = value.members;
  if (
    members.size !== names.length ||
    !names.every((name) => members.has(name))
  ) {
    throw new ShapeMismatch();
  }
  return members;
}

/** The member name of members, which a caller has required. */
export function member(members: Members, name: string): JsonValue {
  const value = members.get(name);
  if (value === undefined) {
    throw new ShapeMismatch();
  }
  return value;
}

/** The string value of a member. */
export function stringMember(members: Members, name: string): string {
  const value = member(members, name);
  if (value.kind !== "string") {
    throw new ShapeMismatch();
  }
  return value.value;
}

/** The string value of a member, which must match pattern. */
export function patternMember(
  members: Members,
  name: string,
  pattern: RegExp,
): string {
  const value = stringMember(members, name);
  if (!pattern.test(value)) {
    throw new ShapeMismatch();
  }
  return value;
}

/** The value of a member, which must be a string in set. */
export function codeMember<K extends string>(
  members: Members,
  name: string,
  set: Readonly<Record<K, true>>,
): K {
  const value = stringMember(members, name);
  if (!isIn(set, value)) {
    throw new ShapeMismatch();
  }
  return value;
}

/** The boolean value of a member. */
export function booleanMember(members: Members, name: string): boolean {
  const value = member(members, name);
  if (value.kind !== "boolean") {
    throw new ShapeMismatch();
  }
  return value.value;
}

// A decimal integer: no sign, fraction or exponent, and no leading zero.
const integerPattern = /^(0|[1-9][0-9]*)$/;

/** A JSON number written as a decimal integer no greater than 2^53-1. */
export function safeInteger(value: JsonValue): number {
  if (value.kind !== "number" || !integerPattern.test(value.text)) {
    throw new ShapeMismatch();
  }
  // The text is checked first, so Number reads only digits; a longer run than
  // a safe integer converts to a larger value and is refused here.
  const n = Number(value.text);
  if (n > Number.MAX_SAFE_INTEGER) {
    throw new ShapeMismatch();
  }
  return n;
}

/** The string member name of an object whose member set depends on it, read before that set is checked. */
export function discriminant(value: JsonValue, name: string): string {
  if (value.kind !== "object") {
    throw new ShapeMismatch();
  }
  const found = value.members.get(name);
  if (found === undefined || found.kind !== "string") {
    throw new ShapeMismatch();
  }
  return found.value;
}

const encoder = new TextEncoder();

/**
 * Reads text with read, or returns undefined when text has an unpaired
 * surrogate (its UTF-8 would replace it), its UTF-8 is longer than maxBytes,
 * it is not one strict JSON value (strictJson.ts: no duplicate member, no
 * unpaired surrogate escape), or read refuses it. Never throws for a refusal;
 * another error from read is a bug and propagates.
 */
export function readShaped<T>(
  text: string,
  maxBytes: number,
  read: (value: JsonValue) => T,
): T | undefined {
  if (hasUnpairedSurrogate(text)) {
    return undefined;
  }
  const bytes = encoder.encode(text);
  if (bytes.length > maxBytes) {
    return undefined;
  }
  const parsed = parseStrictJson(bytes);
  if (!parsed.ok) {
    return undefined;
  }
  try {
    return read(parsed.value);
  } catch (cause) {
    if (cause instanceof ShapeMismatch) {
      return undefined;
    }
    throw cause;
  }
}
