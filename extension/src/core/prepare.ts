import type { CollectedInput, CollectOutcome } from "./collect.ts";
import type { RejectionReason } from "./types.ts";

/** Optional members of the request body: the video metadata read from the page. */
export interface ArticleMetadata {
  readonly title?: string | undefined;
  readonly channelName?: string | undefined;
  readonly description?: string | undefined;
}

/** The request body as the object the browser serializes, and its measured UTF-8 size. */
export interface ArticleBody {
  readonly message: Readonly<Record<string, string>>;
  /** UTF-8 byte length of JSON.stringify(message): the extension's own 4 MiB check. */
  readonly byteLength: number;
}

const metadataKeys = ["title", "channelName", "description"] as const;

/**
 * Builds the body from the collected values and the non-empty metadata
 * members. The URL and selection are copied verbatim; empty and absent
 * metadata members are omitted, never written as null. The size is the UTF-8
 * byte length of the body's JSON text.
 */
export function buildArticleBody(
  input: CollectedInput,
  metadata: ArticleMetadata,
): ArticleBody {
  const body: Record<string, string> = {
    videoUrl: input.url,
    transcript: input.selection,
  };
  for (const key of metadataKeys) {
    const value = metadata[key];
    if (value !== undefined && value !== "") {
      body[key] = value;
    }
  }
  const byteLength = new TextEncoder().encode(JSON.stringify(body)).length;
  return { message: body, byteLength };
}

/** The server's body limit: 4 MiB, counted in UTF-8 bytes. */
export const maxBodyBytes = 4_194_304;

declare const preparedBrand: unique symbol;

/** A request that passed every check. Only prepareSend creates it. */
export interface PreparedRequest {
  readonly body: ArticleBody;
  readonly [preparedBrand]: true;
}

/** Why a collected input is not sent; only the size check is left. */
export type PreflightReason = "body-too-large";

/** Runtime set of the preflight reasons; a Record, so it stays in step with the union. */
export const preflightReasons: Record<PreflightReason, true> = {
  "body-too-large": true,
};

/** Every reason a launch ends without a request: collection or preflight. */
export type BlockReason = RejectionReason | PreflightReason;

export type PrepareOutcome =
  | { readonly kind: "ready"; readonly request: PreparedRequest }
  | { readonly kind: "blocked"; readonly reason: BlockReason };

/**
 * Checks the collected outcome, then the body size, and builds the request
 * only when both pass. Synchronous and reads no storage; never throws.
 */
export function prepareSend(
  outcome: CollectOutcome,
  metadata: ArticleMetadata,
): PrepareOutcome {
  if (outcome.kind === "rejected") {
    return { kind: "blocked", reason: outcome.reason };
  }
  const body = buildArticleBody(outcome.input, metadata);
  if (body.byteLength > maxBodyBytes) {
    return { kind: "blocked", reason: "body-too-large" };
  }
  const request = { body } as PreparedRequest;
  return { kind: "ready", request };
}
