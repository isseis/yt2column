import type { CollectedInput } from "./collect.ts";
import type { MetadataResult } from "./metadata.ts";
import type {
  PrepareOutcome,
  PreflightReason,
  PreparedRequest,
} from "./prepare.ts";
import {
  countCharacters,
  countLines,
  truncate,
  truncateTail,
} from "./summary.ts";
import type { DisplayText } from "./summary.ts";
import type { RejectionReason } from "./types.ts";

/** The display limit for one metadata value, in code points. */
export const metadataDisplayLimit = 200;

/** The display limit for the tab title, in code points. */
const titleLimit = 1000;

/** The display limit for the URL, in code points. */
const urlLimit = 1000;

/** The length of each selection preview, in code points. */
const previewLimit = 100;

/** Matches one line break written as \r\n, \r or \n. */
const lineBreak = /\r\n|\r|\n/g;

/** Shows every line break as one space; used only after the text is cut. */
function withoutLineBreaks(text: string): string {
  return text.replace(lineBreak, " ");
}

export interface VideoView {
  /** The video title when it is sent, otherwise the tab title. */
  readonly heading: DisplayText;
  /** Present only when the channel name is sent. */
  readonly channelName?: DisplayText;
}

/** The selection row's reason when the body can be sent. */
export interface SendableSelection {
  readonly kind: "sendable";
  readonly state: "ok";
  readonly request: PreparedRequest;
}

/** The selection row's reason when the body is over the limit. */
export interface BlockedSelection {
  readonly kind: "blocked";
  readonly state: "warning";
  readonly reason: PreflightReason;
}

/** Why the selection row is OK or wants attention; declared, never inferred from text. */
export type SelectionStatus = SendableSelection | BlockedSelection;

export interface SelectionCheck {
  readonly status: SelectionStatus;
  readonly characterCount: number;
  readonly lineCount: number;
  /** The first 100 code points, each line break shown as one space. */
  readonly head: DisplayText;
  /** The last 100 code points; absent when the selection is 100 code points or fewer. */
  readonly tail?: DisplayText;
}

/** Why the description row is OK or wants attention; declared, never inferred from text. */
export type DescriptionStatus =
  | {
      readonly kind: "full";
      readonly state: "ok";
      readonly characterCount: number;
    }
  | { readonly kind: "collapsed"; readonly state: "warning" }
  | { readonly kind: "not-sent"; readonly state: "warning" };

export interface DescriptionCheck {
  readonly status: DescriptionStatus;
}

export interface DetailsView {
  readonly url: DisplayText;
  readonly tabTitle: DisplayText;
}

export interface CollectedView {
  readonly kind: "collected";
  readonly video: VideoView;
  readonly selection: SelectionCheck;
  readonly description: DescriptionCheck;
  readonly details: DetailsView;
}

export interface RejectedView {
  readonly kind: "rejected";
  readonly reason: RejectionReason;
}

/** What the popup displays and whether the primary button can be pressed. */
export type PopupView = CollectedView | RejectedView;

/**
 * Builds what the popup displays from a collected input, the read metadata and
 * the size check's outcome. Pure: no DOM, no browser API, and every decision
 * about state, previews and the heading's substitute is made here. Never throws.
 */
export function buildCollectedView(
  input: CollectedInput,
  metadata: MetadataResult,
  prepared: PrepareOutcome,
): CollectedView {
  const status: SelectionStatus =
    prepared.kind === "ready"
      ? { kind: "sendable", state: "ok", request: prepared.request }
      : { kind: "blocked", state: "warning", reason: prepared.reason };
  const description = metadata.values.description;
  return {
    kind: "collected",
    video: videoView(input, metadata),
    selection: selectionCheck(input, status),
    description: {
      status: descriptionStatus(description, metadata.showExpandPrompt),
    },
    details: {
      url: truncate(input.url, urlLimit),
      tabTitle: truncate(input.title, titleLimit),
    },
  };
}

/** The heading is the video title when it is sent, otherwise the tab title. */
function videoView(input: CollectedInput, metadata: MetadataResult): VideoView {
  const title = metadata.values.title;
  const channelName = metadata.values.channelName;
  const heading =
    title === undefined
      ? truncate(input.title, titleLimit)
      : truncate(title, metadataDisplayLimit);
  if (channelName === undefined) {
    return { heading };
  }
  return { heading, channelName: truncate(channelName, metadataDisplayLimit) };
}

/**
 * The selection row: the counts of the whole selection, and the first and last
 * 100 code points cut before their line breaks become spaces.
 */
function selectionCheck(
  input: CollectedInput,
  status: SelectionStatus,
): SelectionCheck {
  const head = truncate(input.selection, previewLimit);
  const tail = truncateTail(input.selection, previewLimit);
  const check: SelectionCheck = {
    status,
    characterCount: countCharacters(input.selection),
    lineCount: countLines(input.selection),
    head: { value: withoutLineBreaks(head.value), truncated: head.truncated },
  };
  if (!tail.truncated) {
    return check;
  }
  return {
    ...check,
    tail: { value: withoutLineBreaks(tail.value), truncated: tail.truncated },
  };
}

/** Why the description row shows what it shows; the text itself is never shown. */
function descriptionStatus(
  description: string | undefined,
  collapsed: boolean,
): DescriptionStatus {
  if (description === undefined) {
    return { kind: "not-sent", state: "warning" };
  }
  if (collapsed) {
    return { kind: "collapsed", state: "warning" };
  }
  return {
    kind: "full",
    state: "ok",
    characterCount: countCharacters(description),
  };
}
