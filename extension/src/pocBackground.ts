/* eslint-disable no-restricted-syntax -- PoC: chrome.storage.session holds the run records. */
// 0090 PoC (PoC branch only, never merged into main): the service worker's
// part of the background-runner measurement. It registers its listener
// synchronously when imported by background.ts.
//
// - "pocStart" from the control page: keeps the run's payload in memory,
//   opens poc-runner.html#<runId> as an inactive tab, and writes the run's
//   first record to chrome.storage.session.
// - "pocClaim" from the runner page: hands the payload over once and forgets
//   it. The payload may hold the API key; it lives only in this worker's
//   memory until claimed and is never written to storage or logged.
// - "pocUpdate" from the runner page: merges the runner's record (which never
//   holds the key, the workspace ID, the transcript, or the article) into the
//   run's chrome.storage.session record.
//   On each change of the reported status, it also pings the runner once over
//   the PoC BroadcastChannel and appends the round trip to the record's
//   swPings (the liveness query the design relies on).
// Every other message is left to the start coordinator: this listener returns
// false without responding.
import { pingRunner } from "./pocPing.ts";

interface PocPayload {
  kind: "generate" | "probe";
  request: string;
  workerForm: "classic" | "module";
  deadlineSec: number | null;
  settings: Record<string, unknown>;
}

const payloads = new Map<string, PocPayload>();

/** The last status each run reported, to ping its runner once per change. */
const lastStatus = new Map<string, unknown>();

function recordKey(runId: string): string {
  return `pocRun:${runId}`;
}

/** Serializes every read-modify-write of the run records in this worker. */
let mergeQueue: Promise<void> = Promise.resolve();

function mergeRecord(
  runId: string,
  change: (current: Record<string, unknown>) => Record<string, unknown>,
): Promise<void> {
  const next = mergeQueue.then(async () => {
    const key = recordKey(runId);
    const current = ((await chrome.storage.session.get(key))[key] ??
      {}) as Record<string, unknown>;
    await chrome.storage.session.set({
      [key]: { ...current, ...change(current), updatedAt: Date.now() },
    });
  });
  mergeQueue = next.catch(() => undefined);
  return next;
}

/** Pings the run's runner once and appends the result to swPings. */
async function swPing(runId: string, atStatus: unknown): Promise<void> {
  const result = await pingRunner(runId);
  await mergeRecord(runId, (current) => ({
    swPings: [
      ...((current["swPings"] as unknown[] | undefined) ?? []),
      { atStatus, ...result },
    ],
  }));
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

chrome.runtime.onMessage.addListener((message: unknown, sender, respond) => {
  if (!isObject(message) || sender.id !== chrome.runtime.id) {
    return false;
  }
  switch (message["poc"]) {
    case "pocStart": {
      const payload = message["payload"] as PocPayload;
      const runId = crypto.randomUUID();
      payloads.set(runId, payload);
      void (async () => {
        await mergeRecord(runId, () => ({
          runId,
          kind: payload.kind,
          workerForm: payload.workerForm,
          deadlineSec: payload.deadlineSec,
          settings: payload.settings,
          status: "opening",
          requestedAt: new Date().toISOString(),
        }));
        await chrome.tabs.create({
          url: `poc-runner.html#${runId}`,
          active: false,
        });
        respond({ runId });
      })().catch((error: unknown) => {
        respond({ error: String(error) });
      });
      return true;
    }
    case "pocClaim": {
      const runId = String(message["runId"]);
      const payload = payloads.get(runId);
      payloads.delete(runId);
      respond(payload === undefined ? { error: "no payload" } : { payload });
      return false;
    }
    case "pocUpdate": {
      const runId = String(message["runId"]);
      const update = message["record"];
      if (!isObject(update)) {
        return false;
      }
      // The runner's record never holds swPings, so the merge keeps them.
      void mergeRecord(runId, () => update).then(
        () => respond({ ok: true }),
        (error: unknown) => respond({ error: String(error) }),
      );
      const status = update["status"];
      if (lastStatus.get(runId) !== status) {
        lastStatus.set(runId, status);
        void swPing(runId, status).catch(() => undefined);
      }
      return true;
    }
    default:
      return false;
  }
});
