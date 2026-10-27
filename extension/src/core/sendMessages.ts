// Chooses the message for every way a send path ends: a launch that sends
// nothing, a server rejection, and a send outcome. The wording lives in the
// locale's catalog; these functions only decide which message applies. A
// switch over a type that is not one catalog key ends in never, so adding a
// value without deciding its message fails the type check.

import { catalogs } from "./i18n/catalog.ts";
import type { Locale } from "./i18n/locale.ts";
import { rejectionMessage } from "./messages.ts";
import type { RejectionMessage } from "./messages.ts";
import type { BlockReason } from "./prepare.ts";
import type {
  NotSentCause,
  SendOutcome,
  ServerRejectReason,
  UnexpectedCheck,
} from "./response.ts";

/** Message for a launch that ends without a request. Collection reasons use rejectionMessage. */
export function blockMessage(
  reason: BlockReason,
  locale: Locale,
): RejectionMessage {
  switch (reason) {
    case "not-watch-page":
    case "collection-failed":
    case "empty-selection":
    case "empty-title":
      return rejectionMessage(reason, locale);
    case "body-too-large":
      return catalogs[locale].preflight[reason];
    default: {
      const exhaustive: never = reason;
      throw new Error(`Unhandled block reason: ${String(exhaustive)}`);
    }
  }
}

/** Message for a server rejection; distinct for every reason. */
export function serverRejectMessage(
  reason: ServerRejectReason,
  locale: Locale,
): RejectionMessage {
  return catalogs[locale].serverReject[reason];
}

/** Message for a request known not to have reached the server; no Slack check. */
function notSentMessage(cause: NotSentCause, locale: Locale): RejectionMessage {
  return catalogs[locale].notSent[cause];
}

/** The one message for every unexpected reply; the check is for the developer log only. */
function unexpectedMessage(
  check: UnexpectedCheck,
  locale: Locale,
): RejectionMessage {
  switch (check) {
    case "extension-timeout":
    case "envelope-shape":
    case "envelope-version":
    case "host-protocol-error":
    case "body-over-limit":
    case "encoding":
    case "escape":
    case "json-shape":
    case "duplicate-member":
    case "member-set":
    case "request-id":
    case "status-pairing":
    case "classification-error":
      return catalogs[locale].unexpected;
    default: {
      const exhaustive: never = check;
      throw new Error(`Unhandled unexpected check: ${String(exhaustive)}`);
    }
  }
}

/** Message for every send outcome; rejected delegates to serverRejectMessage. */
export function sendOutcomeMessage(
  outcome: SendOutcome,
  locale: Locale,
): RejectionMessage {
  switch (outcome.kind) {
    case "accepted":
      return catalogs[locale].accepted;
    case "rejected":
      return serverRejectMessage(outcome.reason, locale);
    case "not-sent":
      return notSentMessage(outcome.cause, locale);
    case "no-response":
      return catalogs[locale].noResponse;
    case "host-failed":
      return catalogs[locale].hostFailed;
    case "unexpected":
      return unexpectedMessage(outcome.check, locale);
    default: {
      const exhaustive: never = outcome;
      throw new Error(`Unhandled send outcome: ${String(exhaustive)}`);
    }
  }
}
