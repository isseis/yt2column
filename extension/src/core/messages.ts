import type { Message } from "./i18n/catalog.ts";
import type { RejectionReason } from "./types.ts";

/** A rejection reason's message. The name is kept; the shape is Message. */
export type RejectionMessage = Message;

/**
 * The prompt shown when the video description is collapsed. A fixed text:
 * it never holds page text such as the description itself.
 */
export const expandPromptMessage: Message = {
  text: "The video description is collapsed, so only its beginning will be sent.",
  steps: [
    'Click "...more" below the video to show the full description.',
    "Reopen this popup, then send.",
  ],
};

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
