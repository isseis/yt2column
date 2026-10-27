import type { MetadataField } from "../metadata.ts";
import type { PreflightReason } from "../prepare.ts";
import type { NotSentCause, ServerRejectReason } from "../response.ts";
import type { RejectionReason, SummaryField } from "../types.ts";
import { en } from "./en.ts";
import { ja } from "./ja.ts";
import type { Locale } from "./locale.ts";

/** The shape of a message: a sentence and optional numbered steps. */
export interface Message {
  /** One-sentence statement of the situation. */
  readonly text: string;
  /** Steps that let the user succeed next time; empty when none apply. */
  readonly steps: readonly string[];
}

/**
 * Every string the popup displays, for one locale. Each section keyed by a
 * reason or field type is a Record over that type, so a catalog missing a
 * message -- in any locale, or after a value is added to the type -- fails
 * the type check.
 */
export interface Catalog {
  /** Collection rejection reasons. */
  readonly rejection: Readonly<Record<RejectionReason, Message>>;
  /** Block reasons that are not collection reasons. */
  readonly preflight: Readonly<Record<PreflightReason, Message>>;
  /** Server rejection reasons. */
  readonly serverReject: Readonly<Record<ServerRejectReason, Message>>;
  /** Causes of a request known not to have reached the server; no Slack check. */
  readonly notSent: Readonly<Record<NotSentCause, Message>>;
  /** The other send outcomes. Every result that is not known asks to check Slack. */
  readonly accepted: Message;
  readonly noResponse: Message;
  readonly hostFailed: Message;
  /** The one message for every unexpected reply; the check is for the developer log only. */
  readonly unexpected: Message;
  /** The notice shown while a send is in flight. */
  readonly sending: Message;
  /**
   * The prompt shown when the video description is collapsed. A fixed text:
   * it never holds page text such as the description itself.
   */
  readonly expandPrompt: Message;
  /** Field labels. */
  readonly summaryLabels: Readonly<Record<SummaryField, string>>;
  readonly metadataLabels: Readonly<Record<MetadataField, string>>;
  /** The send button's label. */
  readonly sendButton: string;
}

/** The catalog of every locale. */
export const catalogs: Readonly<Record<Locale, Catalog>> = { en, ja };
