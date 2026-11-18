// Builds and reads the query of a viewer URL in the extension-only path. The
// URL holds no input and no result: only the start ID (r) of a viewer that has
// not been reserved yet, or the job ID (k) of one that has, the collected
// video URL (u), for the link to the video when the record is lost, the mark
// e=1 for an existing article of the same video, and the page title (t) that
// keeps a history entry readable after a restart. A URL is untrusted when
// read: a reload, a restored tab or a history entry may hold any text, so
// every rule below rejects and none repairs. The values only select what to
// show and which record to look up; none of them is ever a generation input.

import { isAcceptedWatchUrl } from "./acceptedUrl.ts";
import {
  countCharacters,
  ellipsis,
  hasUnpairedSurrogate,
  truncate,
} from "./summary.ts";

/** How a viewer URL names its run: a start not reserved yet, or a reserved job. */
export type JobViewerTarget =
  | { readonly kind: "start"; readonly startId: string }
  | {
      readonly kind: "job";
      readonly jobId: string;
      /** The viewer shows an existing article of the same video. */
      readonly existing?: true;
    };

/** The values of a viewer URL. */
export interface JobViewerParams {
  readonly target: JobViewerTarget;
  /** The collected video URL; an accepted watch URL. */
  readonly videoUrl: string;
  /** The page title: 1 to titleLimit code points, no control character. */
  readonly title: string;
}

/** Which rule a viewer URL broke; for the developer log only. */
export type JobViewerUrlError =
  | "unknown-name"
  | "duplicate-name"
  | "malformed-pair"
  | "missing-name"
  | "both-targets"
  | "encoding"
  | "id"
  | "url-too-long"
  | "url-not-accepted"
  | "title"
  | "existing"
  | "fragment";

type Name = "r" | "k" | "u" | "e" | "t";

// The names in the order they are written. Names are only ever added.
const names: readonly Name[] = ["r", "k", "u", "e", "t"];

function isName(value: string): value is Name {
  return (names as readonly string[]).includes(value);
}

/** The longest page title a viewer URL holds, in code points. */
export const titleLimit = 64;

/** u's limit in code points: Chrome's URL limit, which holds any tab URL. */
const videoUrlLimit = 2_097_152;

const idPattern = /^[0-9a-f]{32}$/;

/** The one value of e. */
const existingMark = "1";

// The Cc category: C0 controls, DEL and C1 controls.
const controlCharacter = /\p{Cc}/u;

/**
 * The page title a URL carries for value: value itself when it is at most
 * titleLimit code points, else its first titleLimit - 1 code points and the
 * ellipsis, so the result has titleLimit code points. A surrogate pair is
 * never split.
 */
export function urlTitle(value: string): string {
  if (countCharacters(value) <= titleLimit) {
    return value;
  }
  return `${truncate(value, titleLimit - 1).value}${ellipsis}`;
}

/** Whether title passes the URL's rule for t. */
export function isUrlTitle(title: string): boolean {
  const length = countCharacters(title);
  return (
    length >= 1 &&
    length <= titleLimit &&
    !controlCharacter.test(title) &&
    !hasUnpairedSurrogate(title)
  );
}

/**
 * The heading a start shows before a record exists: the collected video
 * title, or the video URL when no title with a visible character was
 * collected.
 */
export function startHeading(input: {
  readonly title?: string;
  readonly videoUrl: string;
}): string {
  return input.title !== undefined && input.title.trim() !== ""
    ? input.title
    : input.videoUrl;
}

/**
 * The page title a viewer URL of a start keeps: the start's heading, else the
 * video URL, each cut by urlTitle; undefined when neither passes isUrlTitle.
 */
export function startUrlTitle(input: {
  readonly title?: string;
  readonly videoUrl: string;
}): string | undefined {
  return [startHeading(input), input.videoUrl].map(urlTitle).find(isUrlTitle);
}

/**
 * Writes the query, without the leading "?": each present value encoded with
 * encodeURIComponent, the names in the table's order. The result passes
 * parseJobViewerUrl when the values pass isUrlTitle and isAcceptedWatchUrl and
 * hold no unpaired surrogate (encodeURIComponent throws on one).
 */
export function jobViewerQuery(params: JobViewerParams): string {
  const { target } = params;
  const values: Record<Name, string | undefined> = {
    r: target.kind === "start" ? target.startId : undefined,
    k: target.kind === "job" ? target.jobId : undefined,
    u: params.videoUrl,
    e:
      target.kind === "job" && target.existing === true
        ? existingMark
        : undefined,
    t: params.title,
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

export type JobViewerUrlResult =
  | { readonly ok: true; readonly params: JobViewerParams }
  | { readonly ok: false; readonly error: JobViewerUrlError };

/** Splits the query into decoded values, rejecting any pair outside the rules. */
function decodePairs(
  query: string,
): Map<Name, string> | { readonly error: JobViewerUrlError } {
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
 * Reads location.search and location.hash. search is "" or "?" followed by the
 * query. Rejects, and never repairs: a name outside the table, a repeated
 * name, a pair without exactly one "=", a malformed percent-encoding, a
 * non-empty fragment, r and k together or neither, an r or k that is not 32
 * lowercase hexadecimal characters, a missing u or t, a u that is not an
 * accepted watch URL or breaks its limit, a t that breaks isUrlTitle, and an e
 * other than "1" or one without k. "+" is read as "+".
 */
export function parseJobViewerUrl(
  search: string,
  hash: string,
): JobViewerUrlResult {
  if (hash !== "") {
    return { ok: false, error: "fragment" };
  }
  const decoded = decodePairs(
    search.startsWith("?") ? search.slice(1) : search,
  );
  if (!(decoded instanceof Map)) {
    return { ok: false, error: decoded.error };
  }
  const startId = decoded.get("r");
  const jobId = decoded.get("k");
  const videoUrl = decoded.get("u");
  const title = decoded.get("t");
  const existingText = decoded.get("e");
  if (startId !== undefined && jobId !== undefined) {
    return { ok: false, error: "both-targets" };
  }
  const id = startId ?? jobId;
  if (id === undefined || videoUrl === undefined || title === undefined) {
    return { ok: false, error: "missing-name" };
  }
  if (!idPattern.test(id)) {
    return { ok: false, error: "id" };
  }
  if (countCharacters(videoUrl) > videoUrlLimit) {
    return { ok: false, error: "url-too-long" };
  }
  if (!isAcceptedWatchUrl(videoUrl)) {
    return { ok: false, error: "url-not-accepted" };
  }
  if (!isUrlTitle(title)) {
    return { ok: false, error: "title" };
  }
  if (startId !== undefined) {
    return existingText === undefined
      ? {
          ok: true,
          params: { target: { kind: "start", startId }, videoUrl, title },
        }
      : { ok: false, error: "existing" };
  }
  if (existingText === undefined) {
    return {
      ok: true,
      params: { target: { kind: "job", jobId: id }, videoUrl, title },
    };
  }
  if (existingText !== existingMark) {
    return { ok: false, error: "existing" };
  }
  return {
    ok: true,
    params: {
      target: { kind: "job", jobId: id, existing: true },
      videoUrl,
      title,
    },
  };
}
