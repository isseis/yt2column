import type { RejectionReason } from "./types.ts";

export interface RejectionMessage {
  /** One-sentence statement of why nothing was collected. */
  readonly text: string;
  /** Steps that let the user succeed next time; empty when none apply. */
  readonly steps: readonly string[];
}

/**
 * Maps each rejection reason to a distinct English message. The default
 * branch receives never, so adding a reason without a message fails the
 * type check.
 */
export function rejectionMessage(reason: RejectionReason): RejectionMessage {
  switch (reason) {
    case "not-watch-page":
      return {
        text: "This is not a YouTube watch page.",
        steps: [
          "Open a video page such as https://www.youtube.com/watch?v=...",
          "Open the transcript panel and select caption lines.",
          "Launch yt2column again.",
        ],
      };
    case "empty-selection":
      return {
        text: "No text is selected.",
        steps: [
          'Open the transcript panel with "Show transcript".',
          "Select the caption lines.",
          "Launch yt2column again.",
        ],
      };
    case "empty-title":
      return { text: "The tab has no title.", steps: [] };
    case "collection-failed":
      return {
        text: "The selection could not be read from this tab (the page may have changed).",
        steps: [],
      };
    default: {
      const exhaustive: never = reason;
      throw new Error(`Unhandled rejection reason: ${String(exhaustive)}`);
    }
  }
}
