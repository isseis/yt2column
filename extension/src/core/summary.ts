import type { CollectOutcome } from "./collect.ts";
import { rejectionReasons } from "./types.ts";
import type { OutcomeSummary, RejectionReason } from "./types.ts";

const titleLimit = 1000;
const urlLimit = 1000;
const previewLimit = 200;

const collectedKeys: readonly string[] = [
  "kind",
  "title",
  "titleTruncated",
  "url",
  "urlTruncated",
  "characterCount",
  "lineCount",
  "preview",
  "previewTruncated",
];
const rejectedKeys: readonly string[] = ["kind", "reason"];

/**
 * Cuts text to at most limit code points. Iterating the string yields code
 * points, so a surrogate pair at the boundary is never split.
 */
function truncate(
  text: string,
  limit: number,
): { value: string; truncated: boolean } {
  const points = Array.from(text);
  if (points.length <= limit) {
    return { value: text, truncated: false };
  }
  return { value: points.slice(0, limit).join(""), truncated: true };
}

/** Counts code points, so a surrogate pair counts as one character. */
function countCharacters(text: string): number {
  return Array.from(text).length;
}

/** Counts lines split on \r\n, \r or \n; leading and trailing blanks count. */
function countLines(text: string): number {
  return text.split(/\r\n|\r|\n/).length;
}

/** Reduces a collect outcome to the display-only digest. */
export function summarize(outcome: CollectOutcome): OutcomeSummary {
  if (outcome.kind === "rejected") {
    return { kind: "rejected", reason: outcome.reason };
  }
  const { selection, url, title } = outcome.input;
  const shortenedTitle = truncate(title, titleLimit);
  const shortenedUrl = truncate(url, urlLimit);
  const shortenedPreview = truncate(selection, previewLimit);
  return {
    kind: "collected",
    title: shortenedTitle.value,
    titleTruncated: shortenedTitle.truncated,
    url: shortenedUrl.value,
    urlTruncated: shortenedUrl.truncated,
    characterCount: countCharacters(selection),
    lineCount: countLines(selection),
    preview: shortenedPreview.value,
    previewTruncated: shortenedPreview.truncated,
  };
}

function hasExactKeys(
  record: Record<string, unknown>,
  keys: readonly string[],
): boolean {
  const actual = Object.keys(record);
  return (
    actual.length === keys.length &&
    keys.every((key) => Object.prototype.hasOwnProperty.call(record, key))
  );
}

function isRejectionReason(value: unknown): value is RejectionReason {
  return (
    typeof value === "string" &&
    Object.prototype.hasOwnProperty.call(rejectionReasons, value)
  );
}

/**
 * Accepts only a value of exactly the OutcomeSummary shape; otherwise
 * undefined. The value is not repaired to fit.
 */
export function parseSummary(value: unknown): OutcomeSummary | undefined {
  if (typeof value !== "object" || value === null) {
    return undefined;
  }
  const record = value as Record<string, unknown>;
  if (record.kind === "rejected") {
    if (
      !hasExactKeys(record, rejectedKeys) ||
      !isRejectionReason(record.reason)
    ) {
      return undefined;
    }
    return { kind: "rejected", reason: record.reason };
  }
  if (record.kind === "collected") {
    if (
      !hasExactKeys(record, collectedKeys) ||
      typeof record.title !== "string" ||
      typeof record.titleTruncated !== "boolean" ||
      typeof record.url !== "string" ||
      typeof record.urlTruncated !== "boolean" ||
      typeof record.characterCount !== "number" ||
      typeof record.lineCount !== "number" ||
      typeof record.preview !== "string" ||
      typeof record.previewTruncated !== "boolean"
    ) {
      return undefined;
    }
    return {
      kind: "collected",
      title: record.title,
      titleTruncated: record.titleTruncated,
      url: record.url,
      urlTruncated: record.urlTruncated,
      characterCount: record.characterCount,
      lineCount: record.lineCount,
      preview: record.preview,
      previewTruncated: record.previewTruncated,
    };
  }
  return undefined;
}
