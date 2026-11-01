// Declares the results of one exchange with the host that are not a server
// response, shared by the host reply check and every operation's response
// check. Imports no module, so both sides depend on it and not on each other.

/** Why a request is known not to have reached the server. */
export type NotSentCause = "server-unavailable" | "message-too-large";

/**
 * Which check an unexpected reply failed; for the developer log only. The
 * first four are the host reply's outside, the next seven are the strict JSON
 * parse's errors, and the rest are the operation's response checks.
 */
export type UnexpectedCheck =
  | "envelope-shape"
  | "envelope-version"
  | "host-protocol-error"
  | "body-over-limit"
  | "encoding"
  | "bom"
  | "syntax"
  | "duplicate-member"
  | "unpaired-surrogate"
  | "trailing-data"
  | "too-deep"
  | "member-set"
  | "value-shape"
  | "request-id"
  | "status-pairing"
  | "classification-error";

/**
 * An exchange result that is not a server response, as displayed and as
 * passed between extension pages. "not-sent" is known not to have reached the
 * server; the others may have reached it.
 */
export type ExchangeFailure =
  | { readonly kind: "not-sent"; readonly cause: NotSentCause }
  | { readonly kind: "no-response" }
  // Any browser error: the host did not start, or exited without a reply.
  | { readonly kind: "host-failed" }
  | { readonly kind: "unexpected"; readonly check: UnexpectedCheck };

/**
 * As logged: "unexpected" also carries the server's status when a response
 * was relayed, and "host-failed" the browser's error text. The text is never
 * compared, displayed or passed to another page.
 */
export type ExchangeFailureLog =
  | Exclude<ExchangeFailure, { kind: "unexpected" | "host-failed" }>
  | {
      readonly kind: "unexpected";
      readonly check: UnexpectedCheck;
      readonly status?: number;
    }
  | { readonly kind: "host-failed"; readonly lastError: string | undefined };

/** What the outside of one host reply became: a relayed response, or a failure. */
export type HostOutcome =
  | {
      readonly kind: "response";
      readonly status: number;
      readonly bytes: Uint8Array;
    }
  | ExchangeFailureLog;

/** The upper bound for a relayed response body: the host's own relay limit (internal/nmhost). */
export const maxResponseBytes = 524_288;

/** True when record has exactly the given keys, with no extras and none missing. */
export function hasExactKeys(
  record: Record<string, unknown>,
  keys: readonly string[],
): boolean {
  const actual = Object.keys(record);
  return (
    actual.length === keys.length &&
    keys.every((key) => Object.prototype.hasOwnProperty.call(record, key))
  );
}
