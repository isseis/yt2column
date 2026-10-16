import type { CollectedInput, CollectOutcome } from "./collect.ts";
import { checkSettings } from "./settings.ts";
import type {
  ServerPort,
  SettingsBlockReason,
  SettingsReader,
  SharedToken,
  StoredSettings,
} from "./settings.ts";
import type { RejectionReason } from "./types.ts";

/** Optional members of the request body. #170 supplies them; this task passes none. */
export interface ArticleMetadata {
  readonly title?: string | undefined;
  readonly channelName?: string | undefined;
  readonly description?: string | undefined;
}

/** The serialized request body and its UTF-8 size. */
export interface ArticleBody {
  readonly json: string;
  readonly byteLength: number;
}

const metadataKeys = ["title", "channelName", "description"] as const;

/**
 * Serializes the collected values with the non-empty metadata members. The URL
 * and selection are copied verbatim; empty and absent metadata members are
 * omitted, never written as null. The size is counted in UTF-8 bytes.
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
  const json = JSON.stringify(body);
  const byteLength = new TextEncoder().encode(json).length;
  return { json, byteLength };
}

/** The server's body limit: 4 MiB, counted in UTF-8 bytes. */
export const maxBodyBytes = 4_194_304;

declare const preparedBrand: unique symbol;

/** A request that passed every check. Only prepareSend creates it. */
export interface PreparedRequest {
  readonly url: string; // http://127.0.0.1:<port>/articles
  readonly port: ServerPort; // for the developer log
  readonly token: SharedToken;
  readonly body: ArticleBody;
  readonly [preparedBrand]: true;
}

/** Why a collected input is not sent; declared, never inferred from text. */
export type PreflightReason =
  "settings-unreadable" | SettingsBlockReason | "body-too-large";

/** Runtime set of the preflight reasons; a Record, so it stays in step with the union. */
export const preflightReasons: Record<PreflightReason, true> = {
  "settings-unreadable": true,
  "token-missing": true,
  "token-invalid": true,
  "port-invalid": true,
  "body-too-large": true,
};

/** Every reason a launch ends without a request: collection or preflight. */
export type BlockReason = RejectionReason | PreflightReason;

export type PrepareOutcome =
  | { readonly kind: "ready"; readonly request: PreparedRequest }
  | {
      readonly kind: "blocked";
      readonly reason: BlockReason;
      readonly cause?: unknown;
    };

/**
 * Checks the collected outcome, the stored settings and the body size, in the
 * required order, and builds the request only when every check passes.
 * Never throws and never reads settings when the collection was rejected. A
 * stored value that fails its check is refused, never repaired.
 */
export async function prepareSend(
  outcome: CollectOutcome,
  settings: SettingsReader,
  metadata: ArticleMetadata,
): Promise<PrepareOutcome> {
  if (outcome.kind === "rejected") {
    return { kind: "blocked", reason: outcome.reason };
  }
  let stored: StoredSettings;
  try {
    stored = await settings.load();
  } catch (cause) {
    return { kind: "blocked", reason: "settings-unreadable", cause };
  }
  const check = checkSettings(stored);
  if (check.kind === "blocked") {
    return { kind: "blocked", reason: check.reason };
  }
  const body = buildArticleBody(outcome.input, metadata);
  if (body.byteLength > maxBodyBytes) {
    return { kind: "blocked", reason: "body-too-large" };
  }
  const request = {
    url: `http://127.0.0.1:${check.port}/articles`,
    port: check.port,
    token: check.token,
    body,
  } as PreparedRequest;
  return { kind: "ready", request };
}
