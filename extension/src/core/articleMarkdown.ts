// The parse of the display Markdown into the article tree the viewer draws.
// The rules are the small fixed set the requirements define, not CommonMark:
// a line that matches no rule is kept as text, never read more widely. The
// tree holds only the kinds of block and inline the viewer draws, so the
// builder chooses each element by the kind alone.
//
// The parse runs in time proportional to the input: no search for a closing
// mark or a closing fence line starts again over a range an earlier search
// already covered, and nothing recurses.

declare const linkUrlBrand: unique symbol;

/** A URL string that meets every link condition. Only parseArticleMarkdown creates it. */
export type LinkUrl = string & { readonly [linkUrlBrand]: true };

/** One piece of a line's text. */
export type ArticleInline =
  | { readonly kind: "text"; readonly text: string }
  | { readonly kind: "strong"; readonly text: string }
  | { readonly kind: "emphasis"; readonly text: string }
  | { readonly kind: "code"; readonly text: string }
  | { readonly kind: "link"; readonly url: LinkUrl };

/** One source line of a heading, a paragraph or a list item, as inline pieces. */
export type ArticleLine = readonly ArticleInline[];

/** The lines of one list item: its first line and its continuation lines. */
export type ArticleItem = readonly ArticleLine[];

export type ArticleBlock =
  | {
      readonly kind: "heading";
      readonly level: "title" | "section";
      readonly line: ArticleLine;
    }
  | { readonly kind: "paragraph"; readonly lines: readonly ArticleLine[] }
  | { readonly kind: "bulletList"; readonly items: readonly ArticleItem[] }
  | {
      readonly kind: "orderedList";
      readonly start: number;
      readonly items: readonly ArticleItem[];
    }
  | { readonly kind: "codeBlock"; readonly lines: readonly string[] };

const tab = 0x09;
const space = 0x20;
const backslash = 0x5c;
const backtick = 0x60;
const star = 0x2a;
const lessThan = 0x3c;
const greaterThan = 0x3e;

/** Space and tab only: U+3000, U+00A0 and the rest are ordinary characters. */
function isSpace(code: number): boolean {
  return code === space || code === tab;
}

/** Unicode general category Cc. */
function isControl(code: number): boolean {
  return code <= 0x1f || (code >= 0x7f && code <= 0x9f);
}

/** The 32 ASCII punctuation characters. NaN (past the end) is not one. */
function isAsciiPunctuation(code: number): boolean {
  return (
    (code >= 0x21 && code <= 0x2f) ||
    (code >= 0x3a && code <= 0x40) ||
    (code >= 0x5b && code <= 0x60) ||
    (code >= 0x7b && code <= 0x7e)
  );
}

/** The end of the run of code starting at from. */
function runEnd(line: string, from: number, code: number): number {
  let end = from;
  while (line.charCodeAt(end) === code) {
    end++;
  }
  return end;
}

/** Drops each backslash that precedes an ASCII punctuation character. */
function unescape(text: string): string {
  if (!text.includes("\\")) {
    return text;
  }
  let out = "";
  let start = 0;
  for (let k = 0; k < text.length; k++) {
    if (
      text.charCodeAt(k) === backslash &&
      isAsciiPunctuation(text.charCodeAt(k + 1))
    ) {
      out += text.slice(start, k);
      start = k + 1;
      k++;
    }
  }
  return out + text.slice(start);
}

/**
 * The URL string as a LinkUrl when it meets every link condition: it parses,
 * its scheme is https written in lower case, it has a host and no user name or
 * password, and it equals its own serialization character for character.
 */
function linkUrl(text: string): LinkUrl | undefined {
  let parsed: URL;
  try {
    parsed = new URL(text);
  } catch {
    return undefined;
  }
  // The scheme, prefix and host checks mirror the design's table; the
  // autolink rule and the URL parser already imply them.
  if (
    parsed.protocol === "https:" &&
    text.startsWith("https://") &&
    parsed.hostname !== "" &&
    parsed.username === "" &&
    parsed.password === "" &&
    parsed.href === text
  ) {
    return text as LinkUrl;
  }
  return undefined;
}

/**
 * Finds the closing run of a code span: the first run of exactly n backticks
 * at or after a position, with no escape applied. The runs are indexed once
 * per line, and the scan asks with positions that only grow, so each length's
 * list is walked once.
 */
class BacktickRuns {
  private readonly starts = new Map<number, number[]>();
  private readonly next = new Map<number, number>();

  constructor(line: string) {
    let k = line.indexOf("`");
    while (k !== -1) {
      const end = runEnd(line, k, backtick);
      const length = end - k;
      const list = this.starts.get(length);
      if (list === undefined) {
        this.starts.set(length, [k]);
      } else {
        list.push(k);
      }
      k = line.indexOf("`", end);
    }
  }

  /** The start of the first run of exactly length backticks at or after from, or -1. */
  closer(length: number, from: number): number {
    const list = this.starts.get(length);
    if (list === undefined) {
      return -1;
    }
    let index = this.next.get(length) ?? 0;
    while (index < list.length && (list[index] ?? -1) < from) {
      index++;
    }
    this.next.set(length, index);
    return list[index] ?? -1;
  }
}

/**
 * Parses one line by the inline rules. One left-to-right scan; at each
 * position the escape, the code span, the autolink, the strong emphasis and
 * the emphasis are tried in that order, and the first that applies takes the
 * part. Inside strong emphasis and emphasis only the escape applies.
 */
function parseInline(line: string): ArticleLine {
  const out: ArticleInline[] = [];
  const length = line.length;
  let backticks: BacktickRuns | undefined;
  // A search for a closing star run that reached the end of the line without
  // one fails again from any later start, so it is not repeated. This holds
  // because every search starts right after an opening star run, never after
  // a backslash: the earlier search passed that start in the same escape
  // state, so it saw every run a later search would see.
  const starSearchFailedFrom: Record<1 | 2, number> = {
    1: Infinity,
    2: Infinity,
  };

  // The text not yet emitted: pending, then line[segmentStart, position).
  let pending = "";
  let segmentStart = 0;
  const emitText = (end: number): void => {
    pending += line.slice(segmentStart, end);
    if (pending !== "") {
      out.push({ kind: "text", text: pending });
      pending = "";
    }
  };

  /** The start of the run that closes a star run of runLength opened before from, or -1. */
  const starCloser = (runLength: 1 | 2, from: number): number => {
    if (from >= starSearchFailedFrom[runLength]) {
      return -1;
    }
    let k = from;
    while (k < length) {
      const code = line.charCodeAt(k);
      if (code === backslash && isAsciiPunctuation(line.charCodeAt(k + 1))) {
        k += 2;
        continue;
      }
      if (code === star) {
        const end = runEnd(line, k, star);
        if (end - k === runLength && !isSpace(line.charCodeAt(k - 1))) {
          return k;
        }
        k = end;
        continue;
      }
      k++;
    }
    starSearchFailedFrom[runLength] = from;
    return -1;
  };

  /** The position of the > that ends an autolink whose URL starts at from, or -1. */
  const autolinkEnd = (from: number): number => {
    for (let k = from; k < length; k++) {
      const code = line.charCodeAt(k);
      if (code === greaterThan) {
        return k;
      }
      if (isSpace(code) || isControl(code) || code === lessThan) {
        return -1;
      }
    }
    return -1;
  };

  let position = 0;
  while (position < length) {
    const code = line.charCodeAt(position);
    if (code === backslash) {
      if (isAsciiPunctuation(line.charCodeAt(position + 1))) {
        // The backslash is dropped; the character after it starts the next
        // text and is never a mark.
        pending += line.slice(segmentStart, position);
        segmentStart = position + 1;
        position += 2;
      } else {
        position++;
      }
      continue;
    }
    if (code === backtick) {
      const end = runEnd(line, position, backtick);
      backticks ??= new BacktickRuns(line);
      const closer = backticks.closer(end - position, end);
      if (closer === -1) {
        position = end;
        continue;
      }
      emitText(position);
      out.push({ kind: "code", text: line.slice(end, closer) });
      position = closer + (end - position);
      segmentStart = position;
      continue;
    }
    if (code === lessThan && line.startsWith("https://", position + 1)) {
      const end = autolinkEnd(position + 1);
      const url =
        end === -1 ? undefined : linkUrl(line.slice(position + 1, end));
      if (url === undefined) {
        position++;
        continue;
      }
      emitText(position);
      out.push({ kind: "link", url });
      position = end + 1;
      segmentStart = position;
      continue;
    }
    if (code === star) {
      const end = runEnd(line, position, star);
      const runLength = end - position;
      if (
        (runLength === 1 || runLength === 2) &&
        end < length &&
        !isSpace(line.charCodeAt(end))
      ) {
        const closer = starCloser(runLength, end);
        if (closer !== -1) {
          emitText(position);
          out.push({
            kind: runLength === 2 ? "strong" : "emphasis",
            text: unescape(line.slice(end, closer)),
          });
          position = closer + runLength;
          segmentStart = position;
          continue;
        }
      }
      position = end;
      continue;
    }
    position++;
  }
  emitText(length);
  return out;
}

function isBlank(line: string): boolean {
  for (let k = 0; k < line.length; k++) {
    if (!isSpace(line.charCodeAt(k))) {
      return false;
    }
  }
  return true;
}

/** The heading a line opens, or undefined when it is not a heading. */
function heading(
  line: string,
): { level: "title" | "section"; text: string } | undefined {
  let level: "title" | "section";
  let k: number;
  if (line.startsWith("##") && isSpace(line.charCodeAt(2))) {
    level = "section";
    k = 3;
  } else if (line.startsWith("#") && isSpace(line.charCodeAt(1))) {
    level = "title";
    k = 2;
  } else {
    return undefined;
  }
  while (isSpace(line.charCodeAt(k))) {
    k++;
  }
  if (k === line.length) {
    return undefined;
  }
  return { level, text: line.slice(k) };
}

interface Fence {
  readonly mark: number;
  readonly length: number;
}

/** The fence a line could open: three or more backticks or tildes at its start. */
function openingFence(line: string): Fence | undefined {
  const mark = line.charCodeAt(0);
  if (mark !== backtick && mark !== 0x7e) {
    return undefined;
  }
  const end = runEnd(line, 0, mark);
  if (end < 3) {
    return undefined;
  }
  if (mark === backtick && line.includes("`", end)) {
    return undefined;
  }
  return { mark, length: end };
}

/**
 * The fence a line would close: zero to three spaces, three or more of one
 * mark, then only spaces and tabs. A fence of the same mark and at most this
 * length is closed by it.
 */
function closingFence(line: string): Fence | undefined {
  let k = 0;
  while (k < 3 && line.charCodeAt(k) === space) {
    k++;
  }
  const mark = line.charCodeAt(k);
  if (mark !== backtick && mark !== 0x7e) {
    return undefined;
  }
  const end = runEnd(line, k, mark);
  if (end - k < 3 || !isBlank(line.slice(end))) {
    return undefined;
  }
  return { mark, length: end - k };
}

type ListItemLine =
  | { readonly kind: "bullet"; readonly marker: number; readonly text: string }
  | {
      readonly kind: "ordered";
      readonly delimiter: number;
      readonly value: number;
      readonly text: string;
    };

/** The list item a line starts, or undefined when it starts none. */
function listItem(line: string): ListItemLine | undefined {
  const first = line.charCodeAt(0);
  if (
    (first === 0x2d || first === star || first === 0x2b) &&
    line.charCodeAt(1) === space
  ) {
    return { kind: "bullet", marker: first, text: line.slice(2) };
  }
  let digits = 0;
  while (digits < line.length) {
    const code = line.charCodeAt(digits);
    if (code < 0x30 || code > 0x39) {
      break;
    }
    digits++;
  }
  if (digits < 1 || digits > 9) {
    return undefined;
  }
  const delimiter = line.charCodeAt(digits);
  if (
    (delimiter !== 0x2e && delimiter !== 0x29) ||
    line.charCodeAt(digits + 1) !== space
  ) {
    return undefined;
  }
  return {
    kind: "ordered",
    delimiter,
    value: Number(line.slice(0, digits)),
    text: line.slice(digits + 2),
  };
}

/** For each line, the longest closing fence of mark on that line or any later one. */
function longestLaterClosers(lines: readonly string[], mark: number): number[] {
  const longest = new Array<number>(lines.length + 1).fill(0);
  for (let i = lines.length - 1; i >= 0; i--) {
    const fence = closingFence(lines[i] ?? "");
    const here = fence !== undefined && fence.mark === mark ? fence.length : 0;
    longest[i] = Math.max(here, longest[i + 1] ?? 0);
  }
  return longest;
}

interface OpenList {
  readonly kind: "bullet" | "ordered";
  /** The marker of a bullet list or the delimiter of an ordered list. */
  readonly mark: number;
  readonly start: number;
  readonly items: ArticleLine[][];
}

/** Parses the display Markdown by the block and inline rules. Never throws. */
export function parseArticleMarkdown(
  markdown: string,
): readonly ArticleBlock[] {
  const lines = markdown.split("\n");
  // Whether a fence opens is decided from these, so no opening line searches
  // the later lines again.
  const backtickClosers = longestLaterClosers(lines, backtick);
  const tildeClosers = longestLaterClosers(lines, 0x7e);

  const blocks: ArticleBlock[] = [];
  let paragraph: ArticleLine[] | undefined;
  let list: OpenList | undefined;
  // The previous line was a list item or its continuation, with no blank line since.
  let inItem = false;

  const closeParagraph = (): void => {
    if (paragraph !== undefined) {
      blocks.push({ kind: "paragraph", lines: paragraph });
      paragraph = undefined;
    }
  };
  const closeList = (): void => {
    if (list === undefined) {
      return;
    }
    if (list.kind === "bullet") {
      blocks.push({ kind: "bulletList", items: list.items });
    } else {
      blocks.push({
        kind: "orderedList",
        start: list.start,
        items: list.items,
      });
    }
    list = undefined;
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i] ?? "";
    if (isBlank(line)) {
      closeParagraph();
      inItem = false;
      continue;
    }
    const title = heading(line);
    if (title !== undefined) {
      closeParagraph();
      closeList();
      inItem = false;
      blocks.push({
        kind: "heading",
        level: title.level,
        line: parseInline(title.text),
      });
      continue;
    }
    const fence = openingFence(line);
    const closers = fence?.mark === backtick ? backtickClosers : tildeClosers;
    if (fence !== undefined && (closers[i + 1] ?? 0) >= fence.length) {
      closeParagraph();
      closeList();
      inItem = false;
      const code: string[] = [];
      i++;
      for (; i < lines.length; i++) {
        const inside = lines[i] ?? "";
        const closing = closingFence(inside);
        if (
          closing !== undefined &&
          closing.mark === fence.mark &&
          closing.length >= fence.length
        ) {
          break;
        }
        code.push(inside);
      }
      blocks.push({ kind: "codeBlock", lines: code });
      continue;
    }
    const item = listItem(line);
    if (item !== undefined) {
      closeParagraph();
      const mark = item.kind === "bullet" ? item.marker : item.delimiter;
      if (list === undefined || list.kind !== item.kind || list.mark !== mark) {
        closeList();
        list = {
          kind: item.kind,
          mark,
          start: item.kind === "ordered" ? item.value : 1,
          items: [],
        };
      }
      list.items.push([parseInline(item.text)]);
      inItem = true;
      continue;
    }
    if (inItem && list !== undefined) {
      list.items[list.items.length - 1]?.push(parseInline(line));
      continue;
    }
    closeList();
    paragraph ??= [];
    paragraph.push(parseInline(line));
  }
  closeParagraph();
  closeList();
  return blocks;
}
