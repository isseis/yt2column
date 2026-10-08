/** Why a launch did not produce a collectable input. Declared, never inferred from text. */
export type RejectionReason =
  | "not-watch-page" // the URL is not an accepted watch-page URL (or is unavailable)
  | "collection-failed" // the tab or its selection could not be read
  | "empty-selection" // the selection is empty or whitespace only
  | "empty-title"; // the tab title is empty, whitespace only, or unavailable

/**
 * Runtime iteration order of the reasons. Typed as Record<RejectionReason, true>
 * so it stays in step with the union: a reason added to one must be added to
 * the other, or the type check fails.
 */
export const rejectionReasons: Record<RejectionReason, true> = {
  "not-watch-page": true,
  "collection-failed": true,
  "empty-selection": true,
  "empty-title": true,
};

/** What a launch path knows about the tab at the moment of the launch. */
export interface LaunchContext {
  readonly tabId: number;
  readonly url: string | undefined;
  readonly title: string | undefined;
}

/** Display-only digest of an outcome; what the result window receives. */
export type OutcomeSummary =
  | {
      readonly kind: "collected";
      readonly title: string;
      readonly titleTruncated: boolean;
      readonly url: string;
      readonly urlTruncated: boolean;
      readonly characterCount: number;
      readonly lineCount: number;
      readonly preview: string;
      readonly previewTruncated: boolean;
    }
  | { readonly kind: "rejected"; readonly reason: RejectionReason };
