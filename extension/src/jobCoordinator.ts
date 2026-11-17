// The service worker's coordination of generation jobs in the extension-only
// path. It is the only writer of the job records in chrome.storage.session.
// The popup asks for a start; the worker opens the viewer, hands it the input
// once, reserves the run, and from then on only records what the running
// viewer reports. It never reads the API key: it learns only from the names of
// the stored items whether one is set.
//
// Every request is handled one at a time (serial), so the decisions on the
// records and their writes cannot interleave: two starts that arrive together
// cannot both add a record. Waiting for a runner to reconnect and waiting for
// the reservation of a start both happen outside that order, so they hold up
// no other request.
//
// The copied input stays in this worker's memory until the viewer claims it
// (once) or sixty seconds pass; the session area never holds it. A stop of the
// worker loses the input, never a record: the viewer then reports that the
// input could not be taken, and no run is started twice.

import type {
  ExtensionIdentity,
  MessageListener,
  MessageSender,
  ViewerTabs,
  localKeys,
  readSessionItems,
  removeSessionItems,
  writeSessionItems,
} from "./browser/chromeDeps.ts";
import { isAcceptedWatchUrl } from "./core/acceptedUrl.ts";
import {
  jobItemName,
  jobItemPrefix,
  maxRecordBytes,
  readJobRecord,
  readStartContext,
  startItemName,
} from "./core/jobRecord.ts";
import type { JobRecord, StartContext, StopReason } from "./core/jobRecord.ts";
import {
  parseRunnerHello,
  parseSwRequest,
  runnerPortName,
} from "./core/jobMessages.ts";
import type {
  ClaimInputReply,
  ReserveReply,
  RunResult,
  RunnerHello,
  StartPayload,
  StartReply,
  SwRequest,
  WriteReply,
} from "./core/jobMessages.ts";
import { isUrlTitle, jobViewerQuery, urlTitle } from "./core/jobViewerUrl.ts";
import type { Logger } from "./core/types.ts";
import type { Steps } from "./core/wasmResult.ts";

/** The most job records kept. */
export const maxRecords = 50;
/** The most bytes the job records hold together: their JSON in UTF-8. */
export const maxTotalBytes = 8 * 1024 * 1024;
/** The most runs that may run at the same time. */
export const maxRunning = 2;
/** How long an input and a start context are kept without a reservation. */
export const startExpiryMs = 60_000;
/** How long a runner whose Port is not connected has to reconnect. */
export const contactWaitMs = 10_000;
/** The most passes of a reservation's contact checks, so it always ends. */
const maxContactPasses = 3;

/** The Port of a running viewer, as the worker uses it. */
export interface RunnerPort {
  readonly name: string;
  readonly sender: MessageSender;
  onMessage(listener: (message: unknown) => void): void;
  onDisconnect(listener: () => void): void;
  disconnect(): void;
}

export interface JobCoordinatorDeps {
  readonly identity: ExtensionIdentity;
  readonly tabs: Pick<ViewerTabs, "create" | "remove" | "get">;
  /** The names of the stored items: the only thing the worker reads of chrome.storage.local. */
  readonly keys: typeof localKeys;
  readonly readSession: typeof readSessionItems;
  readonly writeSession: typeof writeSessionItems;
  readonly removeSession: typeof removeSessionItems;
  /** A new ID: 128 random bits as 32 lowercase hexadecimal characters. */
  readonly newId: () => string;
  readonly schedule: (ms: number, callback: () => void) => () => void;
  readonly now: () => number;
  readonly log: Logger;
}

export interface JobCoordinator {
  /** The runtime message listener. Returns true only for a request it accepted and will answer. */
  readonly onMessage: MessageListener;
  /** The listener of runtime Port connections. */
  readonly onConnect: (port: RunnerPort) => void;
  /** The listener of closed tabs. */
  readonly onTabRemoved: (tabId: number) => void;
}

const encoder = new TextEncoder();

/** The bytes a value takes in storage: its JSON in UTF-8. */
function storedBytes(value: unknown): number {
  return encoder.encode(JSON.stringify(value)).length;
}

const pendingSteps: Steps = {
  draft: "pending",
  shorten: "pending",
  review: "pending",
};

/** A start waiting for its reservation. */
interface PendingStart {
  /** Undefined once the viewer has claimed it. */
  input: StartPayload | undefined;
  readonly respond: (reply: StartReply) => void;
  readonly cancelExpiry: () => void;
}

/** A record the worker wrote or read, with the bytes it takes. */
type Loaded = ReadonlyMap<string, JobRecord>;

/** The records of the area, keyed by job ID; undefined when the area could not be read. */
async function loadRecords(
  deps: JobCoordinatorDeps,
): Promise<Loaded | undefined> {
  const read = await deps.readSession("every");
  if (!read.ok) {
    return undefined;
  }
  const records = new Map<string, JobRecord>();
  for (const [name, value] of Object.entries(read.value)) {
    if (!name.startsWith(jobItemPrefix)) {
      continue;
    }
    const jobId = name.slice(jobItemPrefix.length);
    const record = readJobRecord(jobId, value);
    if (record !== undefined) {
      records.set(jobId, record);
    }
  }
  return records;
}

/** The record that is dropped first to make room, or undefined when none may be dropped. */
function victim(records: Iterable<JobRecord>): JobRecord | undefined {
  // Failed and stopped records go first, the oldest end first; a done record
  // goes only when none of those is left, so a paid-for article outlives a
  // failure. A running record is never dropped.
  let chosen: Exclude<JobRecord, { state: "running" }> | undefined;
  for (const record of records) {
    if (record.state === "running") {
      continue;
    }
    const rank = (r: typeof record): number => (r.state === "done" ? 1 : 0);
    if (
      chosen === undefined ||
      rank(record) < rank(chosen) ||
      (rank(record) === rank(chosen) && record.endedAt < chosen.endedAt)
    ) {
      chosen = record;
    }
  }
  return chosen;
}

function isRunnerOf(
  record: JobRecord,
  tabId: number,
  instanceId: string,
): boolean {
  return (
    (record.state === "running" || record.state === "stopped") &&
    record.runner.tabId === tabId &&
    record.runner.instanceId === instanceId
  );
}

/** Whether the result a runner delivers again is the one the record already holds. */
function holdsResult(record: JobRecord, result: RunResult): boolean {
  if (record.state === "done" && result.state === "done") {
    return JSON.stringify(record.article) === JSON.stringify(result.article);
  }
  if (record.state === "failed" && result.state === "failed") {
    return (
      record.failure === result.failure &&
      record.internalDetail === result.internalDetail
    );
  }
  return false;
}

/** Creates the coordinator of generation jobs. */
export function createJobCoordinator(deps: JobCoordinatorDeps): JobCoordinator {
  const viewerPage = deps.identity.getURL("viewer.html");
  const popupPage = deps.identity.getURL("popup.html");
  const pending = new Map<string, PendingStart>();
  /** The connected runner Ports, by job ID. */
  const ports = new Map<string, { instanceId: string; port: RunnerPort }>();
  const contactWaiters = new Set<{
    readonly jobId: string;
    readonly instanceId: string;
    resolve(reconnected: boolean): void;
  }>();

  // The serial order of every request: one lock held while a task runs.
  //
  // A function that runs inside a task is named with the suffix Locked. It
  // must never call serial itself, directly or through another function: the
  // inner task would wait for the outer one, which waits for it, and the
  // request would never be answered. Functions without the suffix either call
  // serial (the entry points) or only read.
  let tail: Promise<unknown> = Promise.resolve();
  function serial<T>(task: () => Promise<T>): Promise<T> {
    const run = tail.then(task, task);
    tail = run.catch(() => undefined);
    return run;
  }

  /** The tab of a viewer page sender, or undefined. */
  function viewerTab(sender: MessageSender): number | undefined {
    const tabId = sender.tab?.id;
    if (
      sender.id !== deps.identity.id ||
      typeof tabId !== "number" ||
      sender.url === undefined
    ) {
      return undefined;
    }
    const query = sender.url.indexOf("?");
    const page = query === -1 ? sender.url : sender.url.slice(0, query);
    return page === viewerPage ? tabId : undefined;
  }

  function isPopup(sender: MessageSender): boolean {
    return (
      sender.id === deps.identity.id &&
      sender.tab === undefined &&
      sender.url === popupPage
    );
  }

  /** The start context of startId when it belongs to the viewer in tabId. */
  async function contextOf(
    startId: string,
    tabId: number,
  ): Promise<StartContext | undefined> {
    const name = startItemName(startId);
    const read = await deps.readSession([name]);
    if (!read.ok) {
      return undefined;
    }
    const context = readStartContext(startId, read.value[name]);
    return context?.tabId === tabId ? context : undefined;
  }

  function connected(record: JobRecord): boolean {
    return (
      record.state === "running" &&
      ports.get(record.jobId)?.instanceId === record.runner.instanceId
    );
  }

  // ---- Writing records ----

  /** Writes record; on a failure, once, drops a record by the rule and writes again. */
  async function storeLocked(record: JobRecord): Promise<boolean> {
    const name = jobItemName(record.jobId);
    if ((await deps.writeSession({ [name]: record })).ok) {
      return true;
    }
    const records = await loadRecords(deps);
    const dropped =
      records === undefined
        ? undefined
        : victim([...records.values()].filter((r) => r.jobId !== record.jobId));
    if (dropped === undefined) {
      return false;
    }
    if (!(await deps.removeSession([jobItemName(dropped.jobId)])).ok) {
      return false;
    }
    return (await deps.writeSession({ [name]: record })).ok;
  }

  /** The record as a failure that could not be stored; one attempt to leave that mark. */
  async function markStoreFailedLocked(record: JobRecord): Promise<void> {
    const base = {
      version: record.version,
      jobId: record.jobId,
      inputDigest: record.inputDigest,
      videoId: record.videoId,
      heading: record.heading,
      videoUrl: record.videoUrl,
      acceptedAt: record.acceptedAt,
      steps: record.steps,
    } as const;
    await deps.writeSession({
      [jobItemName(record.jobId)]: {
        ...base,
        state: "failed",
        endedAt: deps.now(),
        failure: "internal",
        internalDetail: "store_failed",
      } satisfies JobRecord,
    });
  }

  /** Writes record for a progress or a result; the reply says whether it is held. */
  async function storeForRunnerLocked(record: JobRecord): Promise<WriteReply> {
    // A record over the read-back limit would be read as absent: it is not held.
    if (storedBytes(record) > maxRecordBytes) {
      return { status: "not_stored" };
    }
    // Room is made first, by the same rule as for a reservation, so a
    // finished record cannot take the total over its limit.
    const loaded = await loadRecords(deps);
    if (loaded !== undefined) {
      const others = new Map(loaded);
      others.delete(record.jobId);
      const drops = roomFor(others, storedBytes(record));
      if (drops !== undefined && drops.length > 0) {
        await deps.removeSession(drops.map(jobItemName));
      }
    }
    if (await storeLocked(record)) {
      return { status: "stored" };
    }
    await markStoreFailedLocked(record);
    return { status: "not_stored" };
  }

  /** Marks the running record of jobId stopped, when it is still running and still matches. */
  async function stopLocked(
    jobId: string,
    reason: StopReason,
    stillMatches: (record: JobRecord) => boolean,
  ): Promise<void> {
    const records = await loadRecords(deps);
    const record = records?.get(jobId);
    if (record?.state !== "running" || !stillMatches(record)) {
      return;
    }
    const written = await deps.writeSession({
      [jobItemName(jobId)]: {
        version: record.version,
        jobId,
        inputDigest: record.inputDigest,
        videoId: record.videoId,
        heading: record.heading,
        videoUrl: record.videoUrl,
        acceptedAt: record.acceptedAt,
        steps: record.steps,
        state: "stopped",
        endedAt: deps.now(),
        runner: record.runner,
        stopReason: reason,
      } satisfies JobRecord,
    });
    if (!written.ok) {
      deps.log.error("stop-not-written", { reason });
    }
  }

  // ---- Start ----

  /** Finishes a start: answers the popup, forgets the input and the context. */
  async function finishStartLocked(
    startId: string,
    reply: StartReply,
  ): Promise<void> {
    const entry = pending.get(startId);
    pending.delete(startId);
    entry?.cancelExpiry();
    await deps.removeSession([startItemName(startId)]);
    entry?.respond(reply);
  }

  async function beginLocked(
    request: Extract<SwRequest, { type: "start" }>,
    respond: (reply: StartReply) => void,
  ): Promise<void> {
    const reject = (
      reason: Extract<StartReply, { status: "rejected" }>["reason"],
    ): void => respond({ status: "rejected", reason });
    const input = request.input;
    const keys = await deps.keys();
    if (!keys.ok) {
      reject("internal");
      return;
    }
    if (!keys.value.apiKeySet) {
      reject("api_key_missing");
      return;
    }
    if (!isAcceptedWatchUrl(input.videoUrl)) {
      reject("invalid_body");
      return;
    }
    const heading =
      input.title !== undefined && input.title.trim() !== ""
        ? input.title
        : input.videoUrl;
    // The page title the viewer's URL keeps: the video title, else the heading.
    const title = [heading, input.videoUrl].map(urlTitle).find(isUrlTitle);
    const startId = deps.newId();
    let query: string;
    try {
      if (title === undefined) {
        throw new Error("no usable page title");
      }
      query = jobViewerQuery({
        target: { kind: "start", startId },
        videoUrl: input.videoUrl,
        title,
      });
    } catch {
      reject("internal");
      return;
    }
    const created = await deps.tabs.create({
      url: `${viewerPage}?${query}`,
      active: false,
      windowId: request.source.windowId,
      index: request.source.index + 1,
      openerTabId: request.source.tabId,
    });
    if (!created.ok) {
      reject("internal");
      return;
    }
    const context: StartContext = {
      version: 1,
      startId,
      tabId: created.value.tabId,
      mode: request.mode,
      createdAt: deps.now(),
      heading,
      channelName: input.channelName ?? "",
      videoTitle: input.title ?? "",
      videoUrl: input.videoUrl,
    };
    // A context over the read-back limit would be read as absent by the
    // viewer's first request: refuse it before anything is opened.
    if (storedBytes(context) > maxRecordBytes) {
      await deps.tabs.remove(created.value.tabId);
      reject("invalid_body");
      return;
    }
    if (!(await deps.writeSession({ [startItemName(startId)]: context })).ok) {
      await deps.tabs.remove(created.value.tabId);
      reject("internal");
      return;
    }
    pending.set(startId, {
      input,
      respond,
      cancelExpiry: deps.schedule(startExpiryMs, () => {
        void serial(() =>
          finishStartLocked(startId, { status: "unknown" }),
        ).catch(() => undefined);
      }),
    });
  }

  async function claimInputLocked(
    startId: string,
    tabId: number,
  ): Promise<ClaimInputReply | undefined> {
    if ((await contextOf(startId, tabId)) === undefined) {
      return undefined;
    }
    const entry = pending.get(startId);
    const input = entry?.input;
    if (entry === undefined || input === undefined) {
      return { status: "gone" };
    }
    entry.input = undefined;
    return { status: "ok", input };
  }

  // ---- Reservation ----

  type Reservation = Extract<SwRequest, { type: "reserve" }>;

  /** One pass of the decision. Either its reply, or the running records whose runner must be checked first. */
  async function decideLocked(
    request: Reservation,
    context: StartContext,
    tabId: number,
    checkContact: boolean,
  ): Promise<
    { readonly reply: ReserveReply } | { readonly check: readonly JobRecord[] }
  > {
    const loaded = await loadRecords(deps);
    if (loaded === undefined) {
      return { reply: { status: "rejected", reason: "internal" } };
    }
    const records = [...loaded.values()];
    const running = records.filter((record) => record.state === "running");
    if (checkContact) {
      const unconfirmed = running.filter((record) => !connected(record));
      if (unconfirmed.length > 0) {
        return { check: unconfirmed };
      }
    }
    const newest = (matching: JobRecord[]): JobRecord | undefined =>
      matching.reduce<JobRecord | undefined>(
        (best, record) =>
          best === undefined || record.acceptedAt > best.acceptedAt
            ? record
            : best,
        undefined,
      );
    const live = (record: JobRecord): boolean =>
      record.state === "running" || record.state === "done";
    const sameInput = newest(
      records.filter((r) => r.inputDigest === request.inputDigest && live(r)),
    );
    if (sameInput !== undefined) {
      return { reply: { status: "existing", jobId: sameInput.jobId } };
    }
    if (context.mode === "inquiry") {
      const sameVideo = newest(
        records.filter((r) => r.videoId === request.videoId && live(r)),
      );
      if (sameVideo !== undefined) {
        return { reply: { status: "same_video", jobId: sameVideo.jobId } };
      }
    }
    if (running.length >= maxRunning) {
      return { reply: { status: "rejected", reason: "too_many_requests" } };
    }
    // A failed or stopped record of the same input is run again under its
    // own job ID; otherwise the run gets a new one.
    const again = newest(
      records.filter((r) => r.inputDigest === request.inputDigest),
    );
    const jobId = again?.jobId ?? deps.newId();
    const record: JobRecord = {
      version: 1,
      jobId,
      inputDigest: request.inputDigest,
      videoId: request.videoId,
      // From the start context the worker validated, not from the viewer.
      heading: {
        videoTitle: context.videoTitle,
        channelName: context.channelName,
      },
      videoUrl: context.videoUrl,
      acceptedAt: deps.now(),
      steps: pendingSteps,
      state: "running",
      runner: { tabId, instanceId: request.instanceId },
    };
    // The record's values come from the start context, which was read back
    // within the same size limit, so the record fits the read-back limit too.
    const others = new Map(loaded);
    others.delete(jobId);
    const drops = roomFor(others, storedBytes(record));
    // A fail-safe: with at most one running record, a record that can be
    // dropped normally exists, so this is reached only by a full area of
    // running records.
    if (drops === undefined) {
      return { reply: { status: "rejected", reason: "too_many_requests" } };
    }
    if (
      drops.length > 0 &&
      !(await deps.removeSession(drops.map(jobItemName))).ok
    ) {
      return { reply: { status: "rejected", reason: "internal" } };
    }
    if (!(await deps.writeSession({ [jobItemName(jobId)]: record })).ok) {
      return { reply: { status: "rejected", reason: "internal" } };
    }
    return { reply: { status: "new_run", jobId } };
  }

  /** The job IDs to drop so that a record of bytes fits, or undefined when it cannot fit. */
  function roomFor(
    others: Loaded,
    bytes: number,
  ): readonly string[] | undefined {
    const remaining = new Map(others);
    let total = bytes;
    for (const record of remaining.values()) {
      total += storedBytes(record);
    }
    const drops: string[] = [];
    while (remaining.size + 1 > maxRecords || total > maxTotalBytes) {
      const dropped = victim(remaining.values());
      if (dropped === undefined) {
        return undefined;
      }
      remaining.delete(dropped.jobId);
      total -= storedBytes(dropped);
      drops.push(dropped.jobId);
    }
    return drops;
  }

  /** Waits for the runner of record to reconnect; false when its tab is gone or it does not. */
  async function reconnects(record: JobRecord): Promise<boolean> {
    if (record.state !== "running") {
      return true;
    }
    if (!(await deps.tabs.get(record.runner.tabId)).ok) {
      return false;
    }
    const { jobId } = record;
    const { instanceId } = record.runner;
    return new Promise<boolean>((resolve) => {
      const waiter = {
        jobId,
        instanceId,
        resolve: (reconnected: boolean): void => {
          cancel();
          contactWaiters.delete(waiter);
          resolve(reconnected);
        },
      };
      contactWaiters.add(waiter);
      const cancel = deps.schedule(contactWaitMs, () => waiter.resolve(false));
      // A hello may have come in while the tab was looked up.
      if (connected(record)) {
        waiter.resolve(true);
        return;
      }
    });
  }

  async function reserve(
    request: Reservation,
    tabId: number,
  ): Promise<ReserveReply | undefined> {
    const context = await contextOf(request.startId, tabId);
    if (context === undefined) {
      return undefined;
    }
    let reply: ReserveReply | undefined;
    for (let pass = 1; reply === undefined; pass += 1) {
      const outcome = await serial(() =>
        decideLocked(request, context, tabId, pass <= maxContactPasses),
      );
      if ("reply" in outcome) {
        reply = outcome.reply;
        break;
      }
      // The waiting is outside the serial order. The stop happens inside it,
      // after the record is read again: a result, a reconnection or a new
      // reservation may have come in the meantime.
      const alive = await Promise.all(outcome.check.map(reconnects));
      await serial(async () => {
        for (const [index, record] of outcome.check.entries()) {
          if (alive[index] !== true && record.state === "running") {
            const { instanceId } = record.runner;
            await stopLocked(
              record.jobId,
              "no_contact",
              (current) =>
                current.state === "running" &&
                current.runner.instanceId === instanceId &&
                !connected(current),
            );
          }
        }
      });
    }
    const status = reply.status;
    await serial(() =>
      finishStartLocked(
        request.startId,
        status === "rejected"
          ? { status: "rejected", reason: reply.reason }
          : status === "same_video"
            ? { status: "same_video", jobId: reply.jobId }
            : { status: "accepted", jobId: reply.jobId },
      ),
    );
    return reply;
  }

  // ---- Reports of the running viewer ----

  async function reportLocked(
    request: Extract<SwRequest, { type: "progress" | "result" }>,
    tabId: number,
  ): Promise<WriteReply | undefined> {
    const records = await loadRecords(deps);
    if (records === undefined) {
      // The area could not be read: that is not "this sender is not the
      // runner". No reply is given, so the runner ignores a progress and
      // sends a result again.
      return undefined;
    }
    const record = records.get(request.jobId);
    if (record === undefined) {
      return { status: "not_runner" };
    }
    if (
      request.type === "result" &&
      !isRunnerOf(record, tabId, request.instanceId)
    ) {
      // A result delivered again after it was stored: the record no longer
      // names a runner, but it holds the same result.
      return holdsResult(record, request.result)
        ? { status: "stored" }
        : { status: "not_runner" };
    }
    if (!isRunnerOf(record, tabId, request.instanceId)) {
      return { status: "not_runner" };
    }
    const base = {
      version: record.version,
      jobId: record.jobId,
      inputDigest: record.inputDigest,
      videoId: record.videoId,
      heading: record.heading,
      videoUrl: record.videoUrl,
      acceptedAt: record.acceptedAt,
    } as const;
    if (request.type === "progress") {
      return storeForRunnerLocked({ ...record, steps: request.steps });
    }
    const result: RunResult = request.result;
    const next: JobRecord =
      result.state === "done"
        ? {
            ...base,
            steps: result.steps,
            state: "done",
            endedAt: deps.now(),
            article: result.article,
          }
        : {
            ...base,
            steps: result.steps,
            state: "failed",
            endedAt: deps.now(),
            failure: result.failure,
            ...(result.internalDetail === undefined
              ? {}
              : { internalDetail: result.internalDetail }),
          };
    return storeForRunnerLocked(next);
  }

  async function reloadedLocked(
    request: Extract<SwRequest, { type: "runnerReloaded" }>,
    tabId: number,
  ): Promise<WriteReply> {
    const records = await loadRecords(deps);
    const record = records?.get(request.jobId);
    if (
      record?.state !== "running" ||
      record.runner.tabId !== tabId ||
      record.runner.instanceId === request.instanceId
    ) {
      return { status: "not_runner" };
    }
    const { instanceId } = record.runner;
    await stopLocked(
      request.jobId,
      "page_reloaded",
      (current) =>
        current.state === "running" && current.runner.instanceId === instanceId,
    );
    return { status: "stored" };
  }

  // ---- Entry points ----

  /** Runs handler, logging a failure; the sender then gets no reply. */
  function guarded(label: string, handler: () => Promise<void>): void {
    handler().catch((error: unknown) => {
      deps.log.error(label, { error: String(error) });
    });
  }

  const onMessage: MessageListener = (message, sender, respond) => {
    const parsed = parseSwRequest(message);
    if (!parsed.ok) {
      deps.log.info("request-rejected", { check: parsed.check });
      return false;
    }
    const request = parsed.request;
    if (request.type === "start") {
      if (!isPopup(sender)) {
        deps.log.info("request-denied", { type: request.type });
        return false;
      }
      guarded("start-failed", () =>
        serial(() => beginLocked(request, respond)).then(() => undefined),
      );
      return true;
    }
    const tabId = viewerTab(sender);
    if (tabId === undefined) {
      deps.log.info("request-denied", { type: request.type });
      return false;
    }
    switch (request.type) {
      case "claimInput":
        guarded("claim-failed", async () => {
          respond(await serial(() => claimInputLocked(request.startId, tabId)));
        });
        return true;
      case "prepareRejected":
        guarded("prepare-rejected-failed", async () => {
          const accepted = await serial(async () => {
            if ((await contextOf(request.startId, tabId)) === undefined) {
              return false;
            }
            await finishStartLocked(request.startId, {
              status: "rejected",
              reason: request.reason,
            });
            return true;
          });
          respond(accepted ? { status: "stored" } : undefined);
        });
        return true;
      case "reserve":
        guarded("reserve-failed", async () => {
          respond(await reserve(request, tabId));
        });
        return true;
      case "progress":
      case "result":
        guarded("report-failed", async () => {
          respond(await serial(() => reportLocked(request, tabId)));
        });
        return true;
      case "runnerReloaded":
        guarded("reloaded-failed", async () => {
          respond(await serial(() => reloadedLocked(request, tabId)));
        });
        return true;
    }
  };

  const onConnect = (port: RunnerPort): void => {
    const tabId = viewerTab(port.sender);
    if (port.name !== runnerPortName || tabId === undefined) {
      port.disconnect();
      return;
    }
    let hello: RunnerHello | undefined;
    let registered = false;
    let closed = false;
    port.onMessage((message) => {
      if (hello !== undefined) {
        return;
      }
      hello = parseRunnerHello(message);
      if (hello === undefined) {
        port.disconnect();
        return;
      }
      const accepted = hello;
      guarded("hello-failed", async () => {
        const matches = await serial(async () => {
          const records = await loadRecords(deps);
          const record = records?.get(accepted.jobId);
          return (
            record !== undefined &&
            isRunnerOf(record, tabId, accepted.instanceId)
          );
        });
        if (!matches) {
          port.disconnect();
          return;
        }
        if (closed) {
          // Cut while the record was being checked: nothing is connected.
          await serial(() =>
            stopLocked(
              accepted.jobId,
              "page_closed",
              (record) =>
                record.state === "running" &&
                record.runner.instanceId === accepted.instanceId &&
                !connected(record),
            ),
          );
          return;
        }
        registered = true;
        ports.set(accepted.jobId, {
          instanceId: accepted.instanceId,
          port,
        });
        for (const waiter of [...contactWaiters]) {
          if (
            waiter.jobId === accepted.jobId &&
            waiter.instanceId === accepted.instanceId
          ) {
            waiter.resolve(true);
          }
        }
      });
    });
    port.onDisconnect(() => {
      closed = true;
      if (hello === undefined || !registered) {
        return;
      }
      const gone = hello;
      if (ports.get(gone.jobId)?.port !== port) {
        return;
      }
      ports.delete(gone.jobId);
      guarded("disconnect-failed", () =>
        serial(() =>
          stopLocked(
            gone.jobId,
            "page_closed",
            (record) =>
              record.state === "running" &&
              record.runner.instanceId === gone.instanceId &&
              !connected(record),
          ),
        ),
      );
    });
  };

  const onTabRemoved = (tabId: number): void => {
    guarded("tab-removed-failed", () =>
      serial(async () => {
        const records = await loadRecords(deps);
        for (const record of records?.values() ?? []) {
          if (record.state === "running" && record.runner.tabId === tabId) {
            const { instanceId } = record.runner;
            await stopLocked(
              record.jobId,
              "tab_closed",
              (current) =>
                current.state === "running" &&
                current.runner.instanceId === instanceId,
            );
          }
        }
      }),
    );
  };

  return { onMessage, onConnect, onTabRemoved };
}
