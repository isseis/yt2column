// Builds and reads the viewer URL's query. The URL is the only place a
// viewer keeps its send: the heading and channel name the popup showed, the
// video title, the collected URL, the send time and, once accepted, the
// article key. It is untrusted when read: a reload, a restored tab or a
// history entry may hold any text, so every rule below rejects and none
// repairs.

import { isAcceptedWatchUrl } from "./acceptedUrl.ts";
import { parseArticleKey } from "./articleKey.ts";
import type { ArticleKey } from "./articleKey.ts";
import { countCharacters } from "./summary.ts";

/** The values a viewer URL holds before the key is known; the popup builds it. */
export interface ViewerSeed {
  readonly heading: string;
  readonly channelName?: string;
  /** The video title from the read metadata, not the heading. */
  readonly title?: string;
  readonly url: string;
  /** The send time, in UNIX milliseconds. */
  readonly sentAt: number;
}

export interface ViewerParams extends ViewerSeed {
  readonly key?: ArticleKey;
}

/** Which rule a viewer URL broke; for the developer log only. */
export type ViewerUrlError =
  | "unknown-name"
  | "duplicate-name"
  | "malformed-pair"
  | "missing-name"
  | "encoding"
  | "empty-value"
  | "too-long"
  | "url-not-accepted"
  | "sent-at"
  | "key"
  | "fragment";

type Name = "h" | "c" | "t" | "u" | "s" | "k";

// The query's names in the order they are written. Names are only ever
// added, as optional ones: a URL an earlier version wrote must stay readable
// from history and restored tabs.
const names: readonly Name[] = ["h", "c", "t", "u", "s", "k"];

function isName(value: string): value is Name {
  return (names as readonly string[]).includes(value);
}

// Each text value's limit in code points. The limits of h, c and t are the
// ones the popup already applies when it builds the values (the display
// limits plus the ellipsis, and the title cut), and u's is Chrome's URL limit,
// which holds any tab URL: a seed built from an input that passed the
// pre-send checks always fits. k's limit is parseArticleKey's own.
const textLimits = { h: 1_001, c: 201, t: 1_000, u: 2_097_152 } as const;

/** The send time's limit in digits: 2^53 - 1 has sixteen. */
const sentAtDigits = 16;

const sentAtPattern = /^(0|[1-9][0-9]*)$/;

/**
 * Checks a seed's values against the URL's rules: h, c and t non-empty and
 * within their limits, u an accepted watch URL within its limit, and sentAt a
 * non-negative safe integer no later than now. Shared by parseViewerUrl and
 * the service worker's check of a start request, so both apply one rule set.
 */
export function viewerSeedError(
  seed: ViewerSeed,
  now: number,
): ViewerUrlError | undefined {
  const texts: [keyof typeof textLimits, string | undefined][] = [
    ["h", seed.heading],
    ["c", seed.channelName],
    ["t", seed.title],
    ["u", seed.url],
  ];
  for (const [name, value] of texts) {
    if (value === undefined) {
      continue;
    }
    if (value === "") {
      return "empty-value";
    }
    if (countCharacters(value) > textLimits[name]) {
      return "too-long";
    }
  }
  if (!isAcceptedWatchUrl(seed.url)) {
    return "url-not-accepted";
  }
  if (
    !Number.isSafeInteger(seed.sentAt) ||
    seed.sentAt < 0 ||
    seed.sentAt > now
  ) {
    return "sent-at";
  }
  return undefined;
}

/**
 * Writes the query, without the leading "?": each present value encoded with
 * encodeURIComponent, the names in the table's order. The result always
 * passes parseViewerUrl. The values must already pass viewerSeedError and hold
 * no unpaired surrogate (encodeURIComponent throws on one); the popup and the
 * service worker see to both before a URL is built.
 */
export function viewerQuery(params: ViewerParams): string {
  const values: Record<Name, string | undefined> = {
    h: params.heading,
    c: params.channelName,
    t: params.title,
    u: params.url,
    s: String(params.sentAt),
    k: params.key,
  };
  const pairs: string[] = [];
  for (const name of names) {
    const value = values[name];
    if (value !== undefined) {
      pairs.push(`${name}=${encodeURIComponent(value)}`);
    }
  }
  return pairs.join("&");
}

export type ViewerUrlResult =
  | { readonly ok: true; readonly params: ViewerParams }
  | { readonly ok: false; readonly error: ViewerUrlError };

/** Splits the query into decoded values, rejecting any pair outside the rules. */
function decodePairs(
  query: string,
): Map<Name, string> | { readonly error: ViewerUrlError } {
  const decoded = new Map<Name, string>();
  if (query === "") {
    return decoded;
  }
  for (const pair of query.split("&")) {
    const parts = pair.split("=");
    if (parts.length !== 2) {
      return { error: "malformed-pair" };
    }
    const [name = "", raw = ""] = parts;
    if (!isName(name)) {
      return { error: "unknown-name" };
    }
    if (decoded.has(name)) {
      return { error: "duplicate-name" };
    }
    try {
      decoded.set(name, decodeURIComponent(raw));
    } catch {
      return { error: "encoding" };
    }
  }
  return decoded;
}

/**
 * Reads location.search and location.hash of a viewer page loaded at
 * loadedAt. search is "" or "?" followed by the query. Rejects, and never
 * repairs: a name outside the table, a repeated name, a pair without exactly
 * one "=" (an empty pair has none), a missing h, u or s, a malformed
 * percent-encoding or one that decodes to invalid UTF-8 (decodeURIComponent
 * throws on both, so a decoded value is always well-formed), a non-empty
 * fragment, and a value that breaks its limit or check. "+" is read as "+",
 * never as a space: the builder writes "%2B".
 */
export function parseViewerUrl(
  search: string,
  hash: string,
  loadedAt: number,
): ViewerUrlResult {
  if (hash !== "") {
    return { ok: false, error: "fragment" };
  }
  const decoded = decodePairs(
    search.startsWith("?") ? search.slice(1) : search,
  );
  if (!(decoded instanceof Map)) {
    return { ok: false, error: decoded.error };
  }

  const heading = decoded.get("h");
  const url = decoded.get("u");
  const sentAtText = decoded.get("s");
  if (heading === undefined || url === undefined || sentAtText === undefined) {
    return { ok: false, error: "missing-name" };
  }
  if (sentAtText === "") {
    return { ok: false, error: "empty-value" };
  }
  if (sentAtText.length > sentAtDigits) {
    return { ok: false, error: "too-long" };
  }
  if (!sentAtPattern.test(sentAtText)) {
    return { ok: false, error: "sent-at" };
  }

  const seed: { -readonly [K in keyof ViewerSeed]: ViewerSeed[K] } = {
    heading,
    url,
    sentAt: Number(sentAtText),
  };
  const channelName = decoded.get("c");
  if (channelName !== undefined) {
    seed.channelName = channelName;
  }
  const title = decoded.get("t");
  if (title !== undefined) {
    seed.title = title;
  }
  const error = viewerSeedError(seed, loadedAt);
  if (error !== undefined) {
    return { ok: false, error };
  }

  const keyText = decoded.get("k");
  if (keyText === undefined) {
    return { ok: true, params: seed };
  }
  const key = parseArticleKey(keyText);
  if (key === undefined) {
    return { ok: false, error: "key" };
  }
  return { ok: true, params: { ...seed, key } };
}
