import { whitespaceOnly } from "./collect.ts";
import type { ArticleMetadata } from "./prepare.ts";

/** The video page values the injected reader returned, before acceptance. */
export interface PageMetadata {
  readonly title?: string | undefined;
  readonly channelName?: string | undefined;
  readonly description?: string | undefined;
  readonly descriptionCollapsed: boolean;
}

/** The accepted metadata; the same type as the request body's optional members. */
export type VideoMetadata = ArticleMetadata;

/** The fields of the metadata; the unit of an omission. */
export type MetadataField = "title" | "channelName" | "description";

/** Why one field was omitted; declared, never inferred from text. */
export type OmissionReason =
  | "absent" // the field was not in the read result
  | "blank" // the value is empty or whitespace only
  | "unpaired-surrogate" // the value has an unpaired surrogate
  | "read-failure"; // the whole read failed (injection error or a rejected shape)

export interface MetadataOmission {
  readonly field: MetadataField;
  readonly reason: OmissionReason;
}

/** Which check a rejected read failed; for the developer log only, never a page value. */
export type MetadataCheck =
  "json" | "top-level" | "member-set" | "duplicate-member" | "value-type";

/** The result of parsing the read's JSON text. */
export type MetadataRead =
  | { readonly kind: "accepted"; readonly page: PageMetadata }
  | { readonly kind: "rejected"; readonly check: MetadataCheck };

/** The result of one page read: the accepted values, the prompt flag, and what to log. */
export interface MetadataResult {
  readonly values: VideoMetadata;
  readonly showExpandPrompt: boolean;
  readonly omissions: readonly MetadataOmission[];
  readonly check?: MetadataCheck;
}

/** The value fields, in the order they are accepted and logged. */
const metadataFields: readonly MetadataField[] = [
  "title",
  "channelName",
  "description",
];

/** The closed member set of a read result; nothing else is accepted. */
const pageMembers: ReadonlySet<string> = new Set([
  ...metadataFields,
  "descriptionCollapsed",
]);

// With the u flag a surrogate pair is one code point, so only a lone (unpaired)
// surrogate has the general category Cs.
const unpairedSurrogate = /\p{Cs}/u;

/**
 * Parses the injected reader's JSON text strictly, in a fixed order: valid
 * JSON, one top-level object, the closed member set with descriptionCollapsed
 * required, no duplicate member, and the value types. Never throws.
 */
export function parsePageMetadata(text: string): MetadataRead {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return { kind: "rejected", check: "json" };
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return { kind: "rejected", check: "top-level" };
  }
  const record = parsed as Record<string, unknown>;
  const keys = Object.keys(record);
  if (
    !keys.every((key) => pageMembers.has(key)) ||
    !Object.hasOwn(record, "descriptionCollapsed")
  ) {
    return { kind: "rejected", check: "member-set" };
  }
  if (hasDuplicateTopLevelMember(text)) {
    return { kind: "rejected", check: "duplicate-member" };
  }
  const collapsed = record.descriptionCollapsed;
  if (typeof collapsed !== "boolean") {
    return { kind: "rejected", check: "value-type" };
  }
  const page: { -readonly [K in keyof PageMetadata]: PageMetadata[K] } = {
    descriptionCollapsed: collapsed,
  };
  for (const field of metadataFields) {
    if (!Object.hasOwn(record, field)) {
      continue;
    }
    const value = record[field];
    if (typeof value !== "string") {
      return { kind: "rejected", check: "value-type" };
    }
    page[field] = value;
  }
  return { kind: "accepted", page };
}

/**
 * Reports whether the top-level object of valid JSON text names a member
 * twice. JSON.parse keeps only the last of duplicate members, so the parsed
 * value cannot show them. Counting quotes does not work either, because a
 * value may contain escaped quotes; this scan follows strings and escapes,
 * and compares the decoded names so "a" and "\u0061" are the same member.
 * The text must already have parsed as one JSON object.
 */
function hasDuplicateTopLevelMember(text: string): boolean {
  const names = new Set<string>();
  let depth = 0;
  let expectName = false;
  let i = 0;
  while (i < text.length) {
    const char = text[i];
    if (char === '"') {
      const end = stringEnd(text, i);
      if (expectName) {
        const name = JSON.parse(text.slice(i, end)) as string;
        if (names.has(name)) {
          return true;
        }
        names.add(name);
        expectName = false;
      }
      i = end;
      continue;
    }
    if (char === "{" || char === "[") {
      depth += 1;
      expectName = depth === 1;
    } else if (char === "}" || char === "]") {
      depth -= 1;
    } else if (char === "," && depth === 1) {
      expectName = true;
    }
    i += 1;
  }
  return false;
}

/** The index just past the JSON string literal that opens at start. */
function stringEnd(text: string, start: number): number {
  let i = start + 1;
  while (i < text.length) {
    const char = text[i];
    if (char === "\\") {
      i += 2;
      continue;
    }
    if (char === '"') {
      return i + 1;
    }
    i += 1;
  }
  return i;
}

/**
 * Drops absent, blank, and ill-formed values. A blank value is empty or only
 * White_Space characters, the same rule the collection applies. Never throws
 * and never repairs: kept values are passed on unchanged, untrimmed and
 * untruncated.
 */
export function acceptMetadata(page: PageMetadata): MetadataResult {
  const values: { -readonly [K in keyof VideoMetadata]: VideoMetadata[K] } = {};
  const omissions: MetadataOmission[] = [];
  for (const field of metadataFields) {
    const value = page[field];
    const reason = omissionReason(value);
    if (reason === undefined) {
      values[field] = value;
    } else {
      omissions.push({ field, reason });
    }
  }
  // The prompt is about the description that will be sent; with no accepted
  // description there is nothing to expand.
  const showExpandPrompt =
    page.descriptionCollapsed && values.description !== undefined;
  return { values, showExpandPrompt, omissions };
}

/** Why a read value is omitted, or undefined when it is kept. */
function omissionReason(value: string | undefined): OmissionReason | undefined {
  if (value === undefined) {
    return "absent";
  }
  if (whitespaceOnly.test(value)) {
    return "blank";
  }
  if (unpairedSurrogate.test(value)) {
    return "unpaired-surrogate";
  }
  return undefined;
}

/** The result of a read that failed; every field omitted with reason "read-failure". */
export function readFailureResult(check?: MetadataCheck): MetadataResult {
  const omissions = metadataFields.map((field): MetadataOmission => ({
    field,
    reason: "read-failure",
  }));
  const result = { values: {}, showExpandPrompt: false, omissions };
  return check === undefined ? result : { ...result, check };
}

/** The empty result used when the collection was rejected: no read is attempted. */
export function emptyMetadataResult(): MetadataResult {
  return { values: {}, showExpandPrompt: false, omissions: [] };
}
