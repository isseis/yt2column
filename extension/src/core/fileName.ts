// The name a saved article is given. A video title with characters a file
// name cannot hold is a valid title, not malformed input, so the name is
// derived from it by fixed rules instead of the title being rejected.

/** A file name's base, before ".md", is at most this many UTF-8 bytes. */
export const maxBaseBytes = 200;

// Each of these becomes "_": control characters, the path separators, the
// characters Windows and the macOS Finder refuse, the characters that change
// the display direction (they can fake the extension) and the zero-width
// characters (they can fake the name). Inclusive code point ranges.
const replacedRanges: readonly (readonly [number, number])[] = [
  [0x0000, 0x001f],
  [0x007f, 0x009f],
  [0x061c, 0x061c],
  [0x200b, 0x200f],
  [0x202a, 0x202e],
  [0x2066, 0x2069],
  [0xfeff, 0xfeff],
];

const replacedCharacters = new Set('/\\:*?"<>|');

/** Whether the code point is one a file name gets "_" for. */
function isReplaced(point: string): boolean {
  const code = point.codePointAt(0) ?? 0;
  return (
    replacedCharacters.has(point) ||
    replacedRanges.some(([low, high]) => code >= low && code <= high)
  );
}

const blank = /^\s*$/u;

/** The UTF-8 length of one code point. */
function utf8Length(point: string): number {
  const code = point.codePointAt(0) ?? 0;
  if (code < 0x80) {
    return 1;
  }
  if (code < 0x800) {
    return 2;
  }
  if (code < 0x10000) {
    return 3;
  }
  return 4;
}

/** Cuts text to at most maxBytes UTF-8 bytes, at a code point boundary. */
function cutToBytes(text: string, maxBytes: number): string {
  let bytes = 0;
  let end = 0;
  for (const point of text) {
    bytes += utf8Length(point);
    if (bytes > maxBytes) {
      break;
    }
    end += point.length;
  }
  return text.slice(0, end);
}

/**
 * The file name for an article: the video title, or the video ID when the
 * title is absent or only white space, with every character above replaced
 * by "_", each leading "." replaced by "_", cut to maxBaseBytes UTF-8 bytes at
 * a code point boundary, and ".md" added. The result never holds a path
 * separator or a control character, never starts with "." and always ends
 * with ".md"; a trailing "." or space before ".md" is harmless, since the
 * extension stays even where Windows strips them. videoId must be non-empty,
 * as watchVideoId returns it for a URL the viewer accepted.
 */
export function articleFileName(
  title: string | undefined,
  videoId: string,
): string {
  const source = title === undefined || blank.test(title) ? videoId : title;
  const replaced = Array.from(source, (point) =>
    isReplaced(point) ? "_" : point,
  ).join("");
  const safe = replaced.replace(/^\.+/u, (dots) => "_".repeat(dots.length));
  return `${cutToBytes(safe, maxBaseBytes)}.md`;
}
