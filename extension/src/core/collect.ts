import { isAcceptedWatchUrl } from "./acceptedUrl.ts";
import type { LaunchContext, RejectionReason } from "./types.ts";

declare const collectedBrand: unique symbol;

/**
 * The collected input, exactly as obtained; nothing is trimmed or rewritten.
 * Only collect() creates values of this type.
 */
export interface CollectedInput {
  readonly selection: string;
  readonly url: string;
  readonly title: string;
  readonly [collectedBrand]: true;
}

export type CollectOutcome =
  | { readonly kind: "collected"; readonly input: CollectedInput }
  | {
      readonly kind: "rejected";
      readonly reason: Exclude<RejectionReason, "collection-failed">;
    }
  | {
      readonly kind: "rejected";
      readonly reason: "collection-failed";
      readonly cause: unknown;
    };

/** What the injected function observed in the tab's top frame. */
export interface PageSelection {
  readonly text: string; // window.getSelection().toString(), unmodified
  readonly documentUrl: string; // location.href at the moment of reading
}

/** The only browser capability collect() needs. Faked in unit tests. */
export interface SelectionReader {
  /** Reads the selection of the tab's top frame. Rejects on any failure. */
  read(tabId: number): Promise<PageSelection>;
}

// The Unicode White_Space property, not JavaScript's \s: \s includes U+FEFF
// but not U+0085, and only White_Space matches the requirement.
const whitespaceOnly = /^\p{White_Space}*$/u;

/**
 * Decides whether a launch yields a collectable input, in the order the
 * requirements fix: launch present, accepted URL, selection read, the read
 * document matches the launched URL, selection non-blank, title non-blank.
 * Never throws and never rewrites the collected values.
 */
export async function collect(
  launch: LaunchContext | undefined,
  reader: SelectionReader,
): Promise<CollectOutcome> {
  if (launch === undefined) {
    return {
      kind: "rejected",
      reason: "collection-failed",
      cause: new Error("no launch context"),
    };
  }
  if (launch.url === undefined || !isAcceptedWatchUrl(launch.url)) {
    return { kind: "rejected", reason: "not-watch-page" };
  }
  let page: PageSelection;
  try {
    page = await reader.read(launch.tabId);
  } catch (cause) {
    return { kind: "rejected", reason: "collection-failed", cause };
  }
  if (page.documentUrl !== launch.url) {
    return {
      kind: "rejected",
      reason: "collection-failed",
      cause: new Error(
        `the document URL changed while reading the selection: launched ${launch.url}, read ${page.documentUrl}`,
      ),
    };
  }
  if (whitespaceOnly.test(page.text)) {
    return { kind: "rejected", reason: "empty-selection" };
  }
  if (launch.title === undefined || whitespaceOnly.test(launch.title)) {
    return { kind: "rejected", reason: "empty-title" };
  }
  const input = {
    selection: page.text,
    url: launch.url,
    title: launch.title,
  } as CollectedInput;
  return { kind: "collected", input };
}
