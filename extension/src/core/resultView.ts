// The value the menu path stores in storage.session and the result window
// reads back. It is read as an untrusted value and accepted only in the exact
// shape below; a value that does not match is refused, never repaired.

import { preflightReasons } from "./prepare.ts";
import type { BlockReason } from "./prepare.ts";
import {
  isServerRejectReason,
  parseRequestId,
  unexpectedChecks,
} from "./response.ts";
import type { SendOutcome, UnexpectedCheck } from "./response.ts";
import { hasExactKeys, isRejectionReason, parseSummary } from "./summary.ts";
import type { OutcomeSummary } from "./types.ts";

/** The collected variant of the display summary. */
export type CollectedSummary = Extract<OutcomeSummary, { kind: "collected" }>;

/** What a result window displays; what the menu path stores in storage.session. */
export type ResultView =
  | { readonly kind: "blocked"; readonly reason: BlockReason }
  | { readonly kind: "sending"; readonly summary: CollectedSummary }
  | {
      readonly kind: "sent";
      readonly summary: CollectedSummary;
      readonly outcome: SendOutcome;
    };

/** True when value is one of the declared preflight reasons. */
function isPreflightReason(value: unknown): boolean {
  return (
    typeof value === "string" &&
    Object.prototype.hasOwnProperty.call(preflightReasons, value)
  );
}

/** True when value is a collection or preflight reason. */
function isBlockReason(value: unknown): value is BlockReason {
  return isRejectionReason(value) || isPreflightReason(value);
}

/** True when value is one of the declared unexpected checks. */
function isUnexpectedCheck(value: unknown): value is UnexpectedCheck {
  return (
    typeof value === "string" &&
    Object.prototype.hasOwnProperty.call(unexpectedChecks, value)
  );
}

/** The collected summary, or undefined when value is not exactly one. */
function parseCollectedSummary(value: unknown): CollectedSummary | undefined {
  const summary = parseSummary(value);
  if (summary === undefined || summary.kind !== "collected") {
    return undefined;
  }
  return summary;
}

/** The send outcome, or undefined when value is not exactly one. */
function parseOutcome(value: unknown): SendOutcome | undefined {
  if (typeof value !== "object" || value === null) {
    return undefined;
  }
  const record = value as Record<string, unknown>;
  switch (record.kind) {
    case "accepted": {
      if (!hasExactKeys(record, ["kind", "requestId"])) {
        return undefined;
      }
      const requestId = parseRequestId(record.requestId);
      if (requestId === undefined) {
        return undefined;
      }
      return { kind: "accepted", requestId };
    }
    case "rejected": {
      if (!hasExactKeys(record, ["kind", "reason", "requestId"])) {
        return undefined;
      }
      if (!isServerRejectReason(record.reason)) {
        return undefined;
      }
      const requestId = parseRequestId(record.requestId);
      if (requestId === undefined) {
        return undefined;
      }
      return { kind: "rejected", reason: record.reason, requestId };
    }
    case "unknown": {
      if (!hasExactKeys(record, ["kind"])) {
        return undefined;
      }
      return { kind: "unknown" };
    }
    case "unexpected": {
      if (!hasExactKeys(record, ["kind", "status", "check"])) {
        return undefined;
      }
      const status = record.status;
      if (
        typeof status !== "number" ||
        !Number.isInteger(status) ||
        status < 100 ||
        status > 599
      ) {
        return undefined;
      }
      if (!isUnexpectedCheck(record.check)) {
        return undefined;
      }
      return { kind: "unexpected", status, check: record.check };
    }
    case "redirected": {
      if (!hasExactKeys(record, ["kind"])) {
        return undefined;
      }
      return { kind: "redirected" };
    }
    default:
      return undefined;
  }
}

/** Accepts only a value of exactly the ResultView shape; otherwise undefined. Never repairs. */
export function parseResultView(value: unknown): ResultView | undefined {
  if (typeof value !== "object" || value === null) {
    return undefined;
  }
  const record = value as Record<string, unknown>;
  switch (record.kind) {
    case "blocked": {
      if (!hasExactKeys(record, ["kind", "reason"])) {
        return undefined;
      }
      if (!isBlockReason(record.reason)) {
        return undefined;
      }
      return { kind: "blocked", reason: record.reason };
    }
    case "sending": {
      if (!hasExactKeys(record, ["kind", "summary"])) {
        return undefined;
      }
      const summary = parseCollectedSummary(record.summary);
      if (summary === undefined) {
        return undefined;
      }
      return { kind: "sending", summary };
    }
    case "sent": {
      if (!hasExactKeys(record, ["kind", "summary", "outcome"])) {
        return undefined;
      }
      const summary = parseCollectedSummary(record.summary);
      if (summary === undefined) {
        return undefined;
      }
      const outcome = parseOutcome(record.outcome);
      if (outcome === undefined) {
        return undefined;
      }
      return { kind: "sent", summary, outcome };
    }
    default:
      return undefined;
  }
}
