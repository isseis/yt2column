import { catalogs } from "./i18n/catalog.ts";
import type { Message } from "./i18n/catalog.ts";
import type { Locale } from "./i18n/locale.ts";
import type { RejectionReason } from "./types.ts";

/** A rejection reason's message. The name is kept; the shape is Message. */
export type RejectionMessage = Message;

/** The prompt shown when the video description is collapsed, in the locale. */
export function expandPromptMessage(locale: Locale): Message {
  return catalogs[locale].expandPrompt;
}

/**
 * The locale's message for a rejection reason. The catalog's rejection
 * section is a Record over RejectionReason, so a reason without a message
 * fails the type check.
 */
export function rejectionMessage(
  reason: RejectionReason,
  locale: Locale,
): RejectionMessage {
  return catalogs[locale].rejection[reason];
}
