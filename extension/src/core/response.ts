// Declares the request id, the one value every server response carries.

declare const requestIdBrand: unique symbol;

/** Sixteen lowercase hexadecimal characters. Only parseRequestId creates it. */
export type RequestId = string & { readonly [requestIdBrand]: true };

const requestIdPattern = /^[0-9a-f]{16}$/;

/** Returns the value unchanged when it has the accepted form; otherwise undefined. */
export function parseRequestId(value: unknown): RequestId | undefined {
  if (typeof value !== "string" || !requestIdPattern.test(value)) {
    return undefined;
  }
  return value as RequestId;
}
