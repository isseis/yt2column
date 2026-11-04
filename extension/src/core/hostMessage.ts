// The only messages the extension sends to the native messaging host: the
// host's versioned form, one per operation. A generation start is built only
// from a PreparedRequest, so its input has passed the size check.

import type { ArticleKey } from "./articleKey.ts";
import type { SendMode } from "./extensionMessages.ts";
import type { PreparedRequest, StartInput } from "./prepare.ts";

/** What POST /jobs reads: the collected values and the declared send mode. */
export type StartBody = StartInput & { readonly mode: SendMode };

export type HostMessage =
  | { readonly v: 1; readonly op: "start"; readonly input: StartBody }
  | { readonly v: 1; readonly op: "status"; readonly key: ArticleKey }
  | { readonly v: 1; readonly op: "publish"; readonly key: ArticleKey };

/** The generation start for a checked request, with its declared send mode. */
export function startMessage(
  request: PreparedRequest,
  mode: SendMode,
): HostMessage {
  return { v: 1, op: "start", input: { ...request.body.message, mode } };
}

/** The status query for an article key. */
export function statusMessage(key: ArticleKey): HostMessage {
  return { v: 1, op: "status", key };
}

/** The publish operation for an article key. */
export function publishMessage(key: ArticleKey): HostMessage {
  return { v: 1, op: "publish", key };
}
