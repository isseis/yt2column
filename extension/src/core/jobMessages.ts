// The messages the extension's pages pass to each other in the extension-only
// path: the requests to the service worker and their replies, the runner's
// Port hello, and the viewers' queries on the BroadcastChannel. A message is
// untrusted until its parse function accepts it: each reads it as the JSON it
// travels as, checks the exact member set and every value's type and range,
// and rejects anything else instead of repairing it. Nothing here carries an
// API key: the service worker never holds one.

import { recordFailures } from "./jobRecord.ts";
import type { RecordFailure } from "./jobRecord.ts";
import {
  ShapeMismatch,
  codeMember,
  discriminant,
  exactMembers,
  member,
  patternMember,
  readShaped,
  safeInteger,
  stringMember,
} from "./jsonShape.ts";
import type { Members } from "./jsonShape.ts";
import { maxBodyBytes } from "./prepare.ts";
import type { JsonValue } from "./strictJson.ts";
import {
  articleResult,
  inputDigestPattern,
  internalDetails,
  readFinishedSteps,
  readSteps,
  videoIdPattern,
} from "./wasmResult.ts";
import type { ArticleResult, InternalDetail, Steps } from "./wasmResult.ts";

/** How a generation start was sent: with the same-video inquiry, or without it. */
export type SendMode = "inquiry" | "plain";

/** The copied input of a start. The service worker holds it in memory only. */
export interface StartPayload {
  readonly videoUrl: string;
  readonly transcript: string;
  readonly title?: string;
  readonly channelName?: string;
  readonly description?: string;
}

/** Why a start is refused. */
export type StartRejection =
  "api_key_missing" | "invalid_body" | "too_many_requests" | "internal";

/** What a running viewer reports at the end of a run. */
export type RunResult =
  | {
      readonly state: "done";
      readonly steps: Steps;
      readonly article: ArticleResult;
    }
  | {
      readonly state: "failed";
      readonly steps: Steps;
      readonly failure: RecordFailure;
      readonly internalDetail?: InternalDetail;
    };

/** The display values a reservation carries. */
export interface ReserveHeading {
  readonly videoTitle: string;
  readonly channelName: string;
}

/** A request to the service worker (chrome.runtime.sendMessage). */
export type SwRequest =
  | {
      readonly type: "start";
      readonly mode: SendMode;
      readonly input: StartPayload;
    }
  | { readonly type: "claimInput"; readonly startId: string }
  | {
      readonly type: "prepareRejected";
      readonly startId: string;
      readonly reason: "invalid_body" | "internal";
      readonly internalDetail?: InternalDetail;
    }
  | {
      readonly type: "reserve";
      readonly startId: string;
      readonly inputDigest: string;
      readonly videoId: string;
      readonly instanceId: string;
      readonly heading: ReserveHeading;
      readonly videoUrl: string;
    }
  | {
      readonly type: "progress";
      readonly jobId: string;
      readonly instanceId: string;
      readonly steps: Steps;
    }
  | {
      readonly type: "result";
      readonly jobId: string;
      readonly instanceId: string;
      readonly result: RunResult;
    }
  | {
      readonly type: "runnerReloaded";
      readonly jobId: string;
      readonly instanceId: string;
    };

export type StartReply =
  | { readonly status: "accepted"; readonly jobId: string }
  | { readonly status: "same_video"; readonly jobId: string }
  | { readonly status: "rejected"; readonly reason: StartRejection }
  | { readonly status: "unknown" };

export type ClaimInputReply =
  | { readonly status: "ok"; readonly input: StartPayload }
  | { readonly status: "gone" };

export type ReserveReply =
  | { readonly status: "new_run"; readonly jobId: string }
  | { readonly status: "existing"; readonly jobId: string }
  | { readonly status: "same_video"; readonly jobId: string }
  | {
      readonly status: "rejected";
      readonly reason: "too_many_requests" | "internal";
    };

export type WriteReply =
  | { readonly status: "stored" }
  | { readonly status: "not_stored" }
  | { readonly status: "not_runner" };

/** The first message of the runner's Port, which names the run. */
export interface RunnerHello {
  readonly type: "hello";
  readonly jobId: string;
  readonly instanceId: string;
}

/** The name of the runner's Port. */
export const runnerPortName = "runner";

/** The viewers' query for a job on the BroadcastChannel, and a viewer's answer. */
export type JobChannelMessage =
  | { readonly type: "find"; readonly queryId: string; readonly jobId: string }
  | {
      readonly type: "present";
      readonly queryId: string;
      readonly tabId: number;
      readonly windowId: number;
    };

export type PresentAnswer = Extract<JobChannelMessage, { type: "present" }>;

/** The viewer to use among those that answered present: the smallest tab ID. */
export function chooseViewer(
  answers: readonly PresentAnswer[],
): PresentAnswer | undefined {
  let chosen: PresentAnswer | undefined;
  for (const answer of answers) {
    if (chosen === undefined || answer.tabId < chosen.tabId) {
      chosen = answer;
    }
  }
  return chosen;
}

const hex32 = /^[0-9a-f]{32}$/;

/** The most a message may hold: the start input's limit and the message's own members. */
export const maxMessageBytes = maxBodyBytes + 4096;

const startRejections: Readonly<Record<StartRejection, true>> = {
  api_key_missing: true,
  invalid_body: true,
  too_many_requests: true,
  internal: true,
};

const sendModes: Readonly<Record<SendMode, true>> = {
  inquiry: true,
  plain: true,
};

/** Reads value as the JSON it travels as; undefined when it has no JSON form or read refuses it. */
function readMessage<T>(
  value: unknown,
  read: (json: JsonValue) => T,
): T | undefined {
  let text: string | undefined;
  try {
    text = JSON.stringify(value) as string | undefined;
  } catch {
    return undefined;
  }
  return text === undefined
    ? undefined
    : readShaped(text, maxMessageBytes, read);
}

/** The members of an object with every required member and, of the optional ones, only those present. */
function withOptional(
  value: JsonValue,
  required: readonly string[],
  optional: readonly string[],
): Members {
  if (value.kind !== "object") {
    throw new ShapeMismatch();
  }
  const present = optional.filter((name) => value.members.has(name));
  return exactMembers(value, [...required, ...present]);
}

function optionalText(members: Members, name: string): string | undefined {
  return members.has(name) ? stringMember(members, name) : undefined;
}

function payload(value: JsonValue): StartPayload {
  const members = withOptional(
    value,
    ["videoUrl", "transcript"],
    ["title", "channelName", "description"],
  );
  const title = optionalText(members, "title");
  const channelName = optionalText(members, "channelName");
  const description = optionalText(members, "description");
  return {
    videoUrl: stringMember(members, "videoUrl"),
    transcript: stringMember(members, "transcript"),
    ...(title === undefined ? {} : { title }),
    ...(channelName === undefined ? {} : { channelName }),
    ...(description === undefined ? {} : { description }),
  };
}

const encoder = new TextEncoder();

/** The input's size in UTF-8 bytes of its JSON, which the 4 MiB limit counts. */
export function startPayloadBytes(input: StartPayload): number {
  return encoder.encode(JSON.stringify(input)).length;
}

function heading(value: JsonValue): ReserveHeading {
  const members = exactMembers(value, ["videoTitle", "channelName"]);
  return {
    videoTitle: stringMember(members, "videoTitle"),
    channelName: stringMember(members, "channelName"),
  };
}

function runResult(value: JsonValue): RunResult {
  switch (discriminant(value, "state")) {
    case "done": {
      const members = exactMembers(value, ["state", "steps", "article"]);
      return {
        state: "done",
        steps: readFinishedSteps(member(members, "steps")),
        article: articleResult(member(members, "article")),
      };
    }
    case "failed": {
      const members = withOptional(
        value,
        ["state", "steps", "failure"],
        ["internalDetail"],
      );
      const failure = codeMember(members, "failure", recordFailures);
      const failed = {
        state: "failed",
        steps: readSteps(member(members, "steps")),
        failure,
      } as const;
      if (!members.has("internalDetail")) {
        return failed;
      }
      if (failure !== "internal") {
        throw new ShapeMismatch();
      }
      return {
        ...failed,
        internalDetail: codeMember(members, "internalDetail", internalDetails),
      };
    }
    default:
      throw new ShapeMismatch();
  }
}

/** The jobId and instanceId members every run message after the reservation carries. */
function runIds(members: Members): {
  readonly jobId: string;
  readonly instanceId: string;
} {
  return {
    jobId: patternMember(members, "jobId", hex32),
    instanceId: patternMember(members, "instanceId", hex32),
  };
}

function swRequest(value: JsonValue): SwRequest {
  const type = discriminant(value, "type");
  switch (type) {
    case "start": {
      const members = exactMembers(value, ["type", "mode", "input"]);
      return {
        type,
        mode: codeMember(members, "mode", sendModes),
        input: payload(member(members, "input")),
      };
    }
    case "claimInput": {
      const members = exactMembers(value, ["type", "startId"]);
      return { type, startId: patternMember(members, "startId", hex32) };
    }
    case "prepareRejected": {
      const members = withOptional(
        value,
        ["type", "startId", "reason"],
        ["internalDetail"],
      );
      const reason = codeMember(members, "reason", {
        invalid_body: true,
        internal: true,
      });
      const base = {
        type,
        startId: patternMember(members, "startId", hex32),
        reason,
      } as const;
      if (!members.has("internalDetail")) {
        return base;
      }
      if (reason !== "internal") {
        throw new ShapeMismatch();
      }
      return {
        ...base,
        internalDetail: codeMember(members, "internalDetail", internalDetails),
      };
    }
    case "reserve": {
      const members = exactMembers(value, [
        "type",
        "startId",
        "inputDigest",
        "videoId",
        "instanceId",
        "heading",
        "videoUrl",
      ]);
      return {
        type,
        startId: patternMember(members, "startId", hex32),
        inputDigest: patternMember(members, "inputDigest", inputDigestPattern),
        videoId: patternMember(members, "videoId", videoIdPattern),
        instanceId: patternMember(members, "instanceId", hex32),
        heading: heading(member(members, "heading")),
        videoUrl: stringMember(members, "videoUrl"),
      };
    }
    case "progress": {
      const members = exactMembers(value, [
        "type",
        "jobId",
        "instanceId",
        "steps",
      ]);
      return {
        type,
        ...runIds(members),
        steps: readSteps(member(members, "steps")),
      };
    }
    case "result": {
      const members = exactMembers(value, [
        "type",
        "jobId",
        "instanceId",
        "result",
      ]);
      return {
        type,
        ...runIds(members),
        result: runResult(member(members, "result")),
      };
    }
    case "runnerReloaded": {
      const members = exactMembers(value, ["type", "jobId", "instanceId"]);
      return { type, ...runIds(members) };
    }
    default:
      throw new ShapeMismatch();
  }
}

/** Why a request is not accepted: its shape, or its start input's size. */
export type SwRequestCheck = "shape" | "size";

export type ParsedSwRequest =
  | { readonly ok: true; readonly request: SwRequest }
  | { readonly ok: false; readonly check: SwRequestCheck };

/**
 * Checks a request to the service worker. A start whose input is over the
 * limit is rejected by size, and every request of another shape by shape;
 * nothing is repaired.
 */
export function parseSwRequest(value: unknown): ParsedSwRequest {
  const request = readMessage(value, swRequest);
  if (request === undefined) {
    return { ok: false, check: "shape" };
  }
  if (
    request.type === "start" &&
    startPayloadBytes(request.input) > maxBodyBytes
  ) {
    return { ok: false, check: "size" };
  }
  return { ok: true, request };
}

/** A reply that is only a status and a job ID. */
function jobIdReply<S extends string>(
  json: JsonValue,
  status: S,
): { readonly status: S; readonly jobId: string } {
  const members = exactMembers(json, ["status", "jobId"]);
  return { status, jobId: patternMember(members, "jobId", hex32) };
}

/** Checks a reply to a start request. */
export function parseStartReply(value: unknown): StartReply | undefined {
  return readMessage(value, (json): StartReply => {
    switch (discriminant(json, "status")) {
      case "accepted":
        return jobIdReply(json, "accepted");
      case "same_video":
        return jobIdReply(json, "same_video");
      case "rejected": {
        const members = exactMembers(json, ["status", "reason"]);
        return {
          status: "rejected",
          reason: codeMember(members, "reason", startRejections),
        };
      }
      case "unknown":
        exactMembers(json, ["status"]);
        return { status: "unknown" };
      default:
        throw new ShapeMismatch();
    }
  });
}

/** Checks a reply to a claimInput request. */
export function parseClaimInputReply(
  value: unknown,
): ClaimInputReply | undefined {
  return readMessage(value, (json): ClaimInputReply => {
    switch (discriminant(json, "status")) {
      case "ok": {
        const members = exactMembers(json, ["status", "input"]);
        return { status: "ok", input: payload(member(members, "input")) };
      }
      case "gone":
        exactMembers(json, ["status"]);
        return { status: "gone" };
      default:
        throw new ShapeMismatch();
    }
  });
}

/** Checks a reply to a reserve request. */
export function parseReserveReply(value: unknown): ReserveReply | undefined {
  return readMessage(value, (json): ReserveReply => {
    switch (discriminant(json, "status")) {
      case "new_run":
        return jobIdReply(json, "new_run");
      case "existing":
        return jobIdReply(json, "existing");
      case "same_video":
        return jobIdReply(json, "same_video");
      case "rejected": {
        const members = exactMembers(json, ["status", "reason"]);
        return {
          status: "rejected",
          reason: codeMember(members, "reason", {
            too_many_requests: true,
            internal: true,
          }),
        };
      }
      default:
        throw new ShapeMismatch();
    }
  });
}

/** Checks a reply to a progress or result request. */
export function parseWriteReply(value: unknown): WriteReply | undefined {
  return readMessage(value, (json): WriteReply => {
    const status = codeMember(exactMembers(json, ["status"]), "status", {
      stored: true,
      not_stored: true,
      not_runner: true,
    });
    return { status };
  });
}

/** Checks the first message of a runner's Port. */
export function parseRunnerHello(value: unknown): RunnerHello | undefined {
  return readMessage(value, (json): RunnerHello => {
    const members = exactMembers(json, ["type", "jobId", "instanceId"]);
    if (stringMember(members, "type") !== "hello") {
      throw new ShapeMismatch();
    }
    return { type: "hello", ...runIds(members) };
  });
}

/** Checks a message on the viewers' channel; the channel carries no sender. */
export function parseJobChannelMessage(
  value: unknown,
): JobChannelMessage | undefined {
  return readMessage(value, (json): JobChannelMessage => {
    switch (discriminant(json, "type")) {
      case "find": {
        const members = exactMembers(json, ["type", "queryId", "jobId"]);
        return {
          type: "find",
          queryId: patternMember(members, "queryId", hex32),
          jobId: patternMember(members, "jobId", hex32),
        };
      }
      case "present": {
        const members = exactMembers(json, [
          "type",
          "queryId",
          "tabId",
          "windowId",
        ]);
        return {
          type: "present",
          queryId: patternMember(members, "queryId", hex32),
          tabId: safeInteger(member(members, "tabId")),
          windowId: safeInteger(member(members, "windowId")),
        };
      }
      default:
        throw new ShapeMismatch();
    }
  });
}
