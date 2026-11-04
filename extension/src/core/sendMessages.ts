// Chooses the message for every way an operation of the job API ends. The
// wording lives in the locale's catalog; these functions only decide which
// message applies. A switch over a type that is not one
// catalog key ends in never, so adding a value without deciding its message
// fails the type check.

import { catalogs } from "./i18n/catalog.ts";
import type { Locale } from "./i18n/locale.ts";
import type { ExchangeMessages, Message } from "./i18n/catalog.ts";
import type { ExchangeFailure, UnexpectedCheck } from "./exchange.ts";
import type {
  RetryingStatus,
  StartFailure,
  StoppingStatus,
} from "./jobView.ts";
import type {
  PublishOutcome,
  ServerRejectReason,
  StartOutcome,
} from "./response.ts";

// The job API's messages. Each operation has its own exchange messages: a
// result that is not known means something different for a generation start,
// which the same input resolves, than for a status query, which repeats by
// itself, or a publish, which may repeat a message. A server rejection has
// one message per reason, whichever operation returned it.

/** Message for a job API rejection; distinct for every reason. */
export function jobRejectMessage(
  reason: ServerRejectReason,
  locale: Locale,
): Message {
  return catalogs[locale].jobReject[reason];
}

/** The one message for every unexpected reply of an operation; the check is for the developer log only. */
function unexpectedExchangeMessage(
  check: UnexpectedCheck,
  messages: ExchangeMessages,
): Message {
  switch (check) {
    case "envelope-shape":
    case "envelope-version":
    case "host-protocol-error":
    case "body-over-limit":
    case "encoding":
    case "bom":
    case "syntax":
    case "duplicate-member":
    case "unpaired-surrogate":
    case "trailing-data":
    case "too-deep":
    case "member-set":
    case "value-shape":
    case "request-id":
    case "status-pairing":
    case "classification-error":
      return messages.unexpected;
    default: {
      const exhaustive: never = check;
      throw new Error(`Unhandled unexpected check: ${String(exhaustive)}`);
    }
  }
}

/** Message for an exchange result that is not a server response, from one operation's messages. */
function exchangeMessage(
  failure: ExchangeFailure,
  messages: ExchangeMessages,
): Message {
  switch (failure.kind) {
    case "not-sent":
      return messages.notSent[failure.cause];
    case "no-response":
      return messages.noResponse;
    case "host-failed":
      return messages.hostFailed;
    case "unexpected":
      return unexpectedExchangeMessage(failure.check, messages);
    default: {
      const exhaustive: never = failure;
      throw new Error(`Unhandled exchange failure: ${String(exhaustive)}`);
    }
  }
}

/**
 * Message for a generation start's result, or a wait that ended without one.
 * A same-video answer is not displayed as a start's result, so it has none.
 */
export function startResultMessage(
  result: Exclude<StartOutcome, { kind: "same-video" }> | StartFailure,
  locale: Locale,
): Message {
  const catalog = catalogs[locale];
  switch (result.kind) {
    case "accepted":
      return catalog.startResult.accepted;
    case "no-result":
      return catalog.startResult.noResult;
    case "rejected":
      return jobRejectMessage(result.reason, locale);
    case "not-sent":
    case "no-response":
    case "host-failed":
    case "unexpected":
      return exchangeMessage(result, catalog.startExchange);
    default: {
      const exhaustive: never = result;
      throw new Error(`Unhandled start result: ${String(exhaustive)}`);
    }
  }
}

/** Message for a status exchange that left the state shown, whether polling continues or stops. */
export function statusNoticeMessage(
  outcome: RetryingStatus | StoppingStatus,
  locale: Locale,
): Message {
  switch (outcome.kind) {
    case "rejected":
      return jobRejectMessage(outcome.reason, locale);
    case "not-sent":
    case "no-response":
    case "host-failed":
    case "unexpected":
      return exchangeMessage(outcome, catalogs[locale].statusExchange);
    default: {
      const exhaustive: never = outcome;
      throw new Error(`Unhandled status notice: ${String(exhaustive)}`);
    }
  }
}

/** Message for a publish operation that was not accepted. */
export function publishResultMessage(
  outcome: Exclude<PublishOutcome, { kind: "accepted" }>,
  locale: Locale,
): Message {
  switch (outcome.kind) {
    case "rejected":
      return jobRejectMessage(outcome.reason, locale);
    case "not-sent":
    case "no-response":
    case "host-failed":
    case "unexpected":
      return exchangeMessage(outcome, catalogs[locale].publishExchange);
    default: {
      const exhaustive: never = outcome;
      throw new Error(`Unhandled publish result: ${String(exhaustive)}`);
    }
  }
}
