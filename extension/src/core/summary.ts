import type { CollectOutcome } from "./collect.ts";
import type { OutcomeSummary } from "./types.ts";

const titleLimit = 1000;
const urlLimit = 1000;
const previewLimit = 200;

/** A value shortened for display; the ellipsis is added when it is rendered. */
export interface DisplayText {
  readonly value: string;
  readonly truncated: boolean;
}

/**
 * Cuts text to its first limit code points. Iterating the string yields code
 * points, so a surrogate pair at the boundary is never split. Exported so the
 * popup's metadata display truncates the same way.
 */
export function truncate(text: string, limit: number): DisplayText {
  const points = Array.from(text);
  if (points.length <= limit) {
    return { value: text, truncated: false };
  }
  return { value: points.slice(0, limit).join(""), truncated: true };
}

/**
 * Cuts text to its last limit code points. Like truncate, it walks code
 * points, so a surrogate pair at the boundary is never split.
 */
export function truncateTail(text: string, limit: number): DisplayText {
  const points = Array.from(text);
  if (points.length <= limit) {
    return { value: text, truncated: false };
  }
  return {
    value: points.slice(points.length - limit).join(""),
    truncated: true,
  };
}

/** Counts code points, so a surrogate pair counts as one character. */
export function countCharacters(text: string): number {
  return Array.from(text).length;
}

/** Matches one line break written as \r\n, \r or \n. */
export const lineBreak = /\r\n|\r|\n/g;

/** Counts lines split on \r\n, \r or \n; leading and trailing blanks count. */
export function countLines(text: string): number {
  return text.split(lineBreak).length;
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
