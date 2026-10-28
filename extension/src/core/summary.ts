/** A value shortened for display; the ellipsis is added when it is rendered. */
export interface DisplayText {
  readonly value: string;
  readonly truncated: boolean;
}

/**
 * Cuts text to its first limit code points. Iterating the string yields code
 * points, so a surrogate pair at the boundary is never split; it stops at the
 * limit instead of walking the whole text.
 */
export function truncate(text: string, limit: number): DisplayText {
  let count = 0;
  let end = 0;
  for (const point of text) {
    if (count === limit) {
      return { value: text.slice(0, end), truncated: true };
    }
    count += 1;
    end += point.length;
  }
  return { value: text, truncated: false };
}

/**
 * Cuts text to its last limit code points. It walks code points back from the
 * end, stepping over a surrogate pair as one, so the pair is never split; a
 * lone surrogate counts as one code point, as it does for truncate.
 */
export function truncateTail(text: string, limit: number): DisplayText {
  let count = 0;
  let start = text.length;
  while (start > 0) {
    if (count === limit) {
      return { value: text.slice(start), truncated: true };
    }
    count += 1;
    start -= endsWithSurrogatePair(text, start) ? 2 : 1;
  }
  return { value: text, truncated: false };
}

/** Whether the code units just before end are a high and a low surrogate. */
function endsWithSurrogatePair(text: string, end: number): boolean {
  if (end < 2) {
    return false;
  }
  const low = text.charCodeAt(end - 1);
  const high = text.charCodeAt(end - 2);
  return low >= 0xdc00 && low <= 0xdfff && high >= 0xd800 && high <= 0xdbff;
}

/** Counts code points, so a surrogate pair counts as one character. */
export function countCharacters(text: string): number {
  // A unit cannot be both a high and a low surrogate, so pairs never overlap.
  let count = text.length;
  for (let end = 2; end <= text.length; end += 1) {
    if (endsWithSurrogatePair(text, end)) {
      count -= 1;
    }
  }
  return count;
}

/** Matches one line break written as \r\n, \r or \n. */
export const lineBreak = /\r\n|\r|\n/g;

/** Counts lines split on \r\n, \r or \n; leading and trailing blanks count. */
export function countLines(text: string): number {
  return text.split(lineBreak).length;
}
