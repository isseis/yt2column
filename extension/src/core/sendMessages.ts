// Chooses the message for every way a generation start and a job end. The
// wording lives in the locale's catalog; these functions only decide which
// message applies, so a state shows a fixed text and never an error's, a
// response's or the browser's own text.

import { catalogs } from "./i18n/catalog.ts";
import type { Message } from "./i18n/catalog.ts";
import type { Locale } from "./i18n/locale.ts";
import type { JobDisplay, StartFailure } from "./jobView.ts";

/** Message for a start that gave no job to show. */
export function startFailureMessage(
  failure: StartFailure,
  locale: Locale,
): Message {
  const catalog = catalogs[locale];
  switch (failure.kind) {
    case "rejected":
      return failure.reason === "api_key_missing"
        ? catalog.setup.api_key_missing
        : catalog.startRejected[failure.reason];
    case "unknown":
      return catalog.startUnknown;
    case "input-unavailable":
      return catalog.inputUnavailable;
  }
}

/**
 * Message for a job that did not end with a kept article; none for a running
 * or done job, which show their steps and facts instead.
 */
export function jobMessage(
  job: JobDisplay,
  locale: Locale,
): Message | undefined {
  const catalog = catalogs[locale];
  switch (job.kind) {
    case "running":
    case "done":
      return undefined;
    case "unstored":
      return catalog.viewer.unstored;
    case "setup":
      return catalog.setup[job.failure];
    case "key-rejected":
      return catalog.keyRejected;
    case "failed":
      return catalog.failure[job.failure];
    case "stopped":
      return catalog.stopped;
    case "lost":
      return catalog.viewer.lost;
  }
}
