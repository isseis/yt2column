// The job records and the start contexts the service worker keeps in
// chrome.storage.session, and the check that reads one back. Only the service
// worker writes them; the popup and the viewers read them. A value read back
// is untrusted: an item another version wrote, or one a developer edited, may
// hold anything. A value that is not exactly one of the shapes below is
// treated as absent, never repaired. Neither shape holds the copied
// transcript or the video description.

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
import type { JsonValue } from "./strictJson.ts";
import {
  articleResult,
  inputDigestPattern,
  internalDetails,
  readFinishedSteps,
  readSteps,
  videoIdPattern,
  wasmFailures,
} from "./wasmResult.ts";
import type {
  ArticleResult,
  InternalDetail,
  Steps,
  WasmFailure,
} from "./wasmResult.ts";

/** The longest record or start context accepted, in UTF-8 bytes of its JSON. */
export const maxRecordBytes = 512 * 1024;

/** The one record version; an extension update empties the session area, so none is migrated. */
const recordVersion = 1;

/** Why a run recorded in a job record failed: the module's failures and the two the running viewer decides. */
export type RecordFailure = WasmFailure | "api_key_missing" | "run_timeout";

const recordFailures: Readonly<Record<RecordFailure, true>> = {
  ...wasmFailures,
  api_key_missing: true,
  run_timeout: true,
};

/** Why the service worker marked a run stopped. */
export type StopReason =
  "tab_closed" | "page_closed" | "page_reloaded" | "no_contact";

const stopReasons: Readonly<Record<StopReason, true>> = {
  tab_closed: true,
  page_closed: true,
  page_reloaded: true,
  no_contact: true,
};

/** The viewer page that runs a job: its tab and the random ID of that page load. */
export interface Runner {
  readonly tabId: number;
  /** 32 lowercase hexadecimal characters. */
  readonly instanceId: string;
}

/** The values every job record has. */
interface RecordBase {
  readonly version: 1;
  /** 32 lowercase hexadecimal characters; opaque. */
  readonly jobId: string;
  /** 64 lowercase hexadecimal characters; compared only. */
  readonly inputDigest: string;
  /** Compared only, for the same-video inquiry. */
  readonly videoId: string;
  /** As collected, for the header before completion. */
  readonly heading: {
    readonly videoTitle: string;
    readonly channelName: string;
  };
  /** As collected, for the link to the video. */
  readonly videoUrl: string;
  /** When this run was reserved, in ms since the epoch. */
  readonly acceptedAt: number;
  readonly steps: Steps;
}

/** One job record, in chrome.storage.session under "job:" and its job ID. */
export type JobRecord =
  | (RecordBase & { readonly state: "running"; readonly runner: Runner })
  | (RecordBase & {
      readonly state: "done";
      readonly endedAt: number;
      readonly article: ArticleResult;
    })
  | (RecordBase & {
      readonly state: "failed";
      readonly endedAt: number;
      readonly failure: RecordFailure;
      /** Only with the failure internal, and not always then. */
      readonly internalDetail?: InternalDetail;
    })
  | (RecordBase & {
      readonly state: "stopped";
      readonly endedAt: number;
      readonly runner: Runner;
      readonly stopReason: StopReason;
    });

/** How a generation start was sent: with the same-video inquiry, or without it. */
export type StartMode = "inquiry" | "plain";

/**
 * One start context, in chrome.storage.session under "start:" and its start ID,
 * until the run is reserved or 60 seconds pass. It holds the display values
 * the viewer shows before the reservation, never the input itself.
 */
export interface StartContext {
  readonly version: 1;
  /** 32 lowercase hexadecimal characters. */
  readonly startId: string;
  /** The viewer's tab. */
  readonly tabId: number;
  readonly mode: StartMode;
  /** When the context was made, in ms since the epoch. */
  readonly createdAt: number;
  /** The heading the popup showed: the video title, or the tab title without one. */
  readonly heading: string;
  /** "" when none was collected. */
  readonly channelName: string;
  /** "" when none was collected. */
  readonly videoTitle: string;
  readonly videoUrl: string;
}

/** A job ID, a start ID or an instance ID: 128 random bits as 32 lowercase hexadecimal characters. */
const hex32 = /^[0-9a-f]{32}$/;

function runner(value: JsonValue): Runner {
  const members = exactMembers(value, ["tabId", "instanceId"]);
  return {
    tabId: safeInteger(member(members, "tabId")),
    instanceId: patternMember(members, "instanceId", hex32),
  };
}

/** The version member, which must be the one version. */
function version(members: Members): 1 {
  if (safeInteger(member(members, "version")) !== recordVersion) {
    throw new ShapeMismatch();
  }
  return recordVersion;
}

/** The members every record has, before the state's own. */
const baseNames = [
  "version",
  "jobId",
  "inputDigest",
  "videoId",
  "heading",
  "videoUrl",
  "acceptedAt",
  "state",
  "steps",
];

/** The base values of a record, with its steps read by readStepsOf. */
function recordBase(
  members: Members,
  readStepsOf: (value: JsonValue) => Steps = readSteps,
): RecordBase {
  const heading = exactMembers(member(members, "heading"), [
    "videoTitle",
    "channelName",
  ]);
  return {
    version: version(members),
    jobId: patternMember(members, "jobId", hex32),
    inputDigest: patternMember(members, "inputDigest", inputDigestPattern),
    videoId: patternMember(members, "videoId", videoIdPattern),
    heading: {
      videoTitle: stringMember(heading, "videoTitle"),
      channelName: stringMember(heading, "channelName"),
    },
    videoUrl: stringMember(members, "videoUrl"),
    acceptedAt: safeInteger(member(members, "acceptedAt")),
    steps: readStepsOf(member(members, "steps")),
  };
}

function jobRecord(value: JsonValue): JobRecord {
  const state = discriminant(value, "state");
  switch (state) {
    case "running": {
      const members = exactMembers(value, [...baseNames, "runner"]);
      return {
        ...recordBase(members),
        state,
        runner: runner(member(members, "runner")),
      };
    }
    case "done": {
      const members = exactMembers(value, [...baseNames, "endedAt", "article"]);
      return {
        ...recordBase(members, readFinishedSteps),
        state,
        endedAt: safeInteger(member(members, "endedAt")),
        article: articleResult(member(members, "article")),
      };
    }
    case "failed": {
      const withDetail =
        value.kind === "object" && value.members.has("internalDetail");
      const names = [...baseNames, "endedAt", "failure"];
      const members = exactMembers(
        value,
        withDetail ? [...names, "internalDetail"] : names,
      );
      const failure = codeMember(members, "failure", recordFailures);
      const failed = {
        ...recordBase(members),
        state,
        endedAt: safeInteger(member(members, "endedAt")),
        failure,
      };
      if (!withDetail) {
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
    case "stopped": {
      const members = exactMembers(value, [
        ...baseNames,
        "endedAt",
        "runner",
        "stopReason",
      ]);
      return {
        ...recordBase(members),
        state,
        endedAt: safeInteger(member(members, "endedAt")),
        runner: runner(member(members, "runner")),
        stopReason: codeMember(members, "stopReason", stopReasons),
      };
    }
    default:
      throw new ShapeMismatch();
  }
}

function startContext(value: JsonValue): StartContext {
  const members = exactMembers(value, [
    "version",
    "startId",
    "tabId",
    "mode",
    "createdAt",
    "heading",
    "channelName",
    "videoTitle",
    "videoUrl",
  ]);
  return {
    version: version(members),
    startId: patternMember(members, "startId", hex32),
    tabId: safeInteger(member(members, "tabId")),
    mode: codeMember(members, "mode", { inquiry: true, plain: true }),
    createdAt: safeInteger(member(members, "createdAt")),
    heading: stringMember(members, "heading"),
    channelName: stringMember(members, "channelName"),
    videoTitle: stringMember(members, "videoTitle"),
    videoUrl: stringMember(members, "videoUrl"),
  };
}

/**
 * Reads value, as read back from chrome.storage.session, with read. The
 * value is measured and checked as the JSON it is stored as; a value that has
 * no JSON form (undefined, a BigInt, a cycle) is absent too.
 */
function readStored<T>(
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
    : readShaped(text, maxRecordBytes, read);
}

/**
 * The job record of jobId in value, the item read back under its name;
 * undefined when value is not exactly a record of that job.
 */
export function readJobRecord(
  jobId: string,
  value: unknown,
): JobRecord | undefined {
  const record = readStored(value, jobRecord);
  return record?.jobId === jobId ? record : undefined;
}

/**
 * The start context of startId in value, the item read back under its name;
 * undefined when value is not exactly a context of that start.
 */
export function readStartContext(
  startId: string,
  value: unknown,
): StartContext | undefined {
  const context = readStored(value, startContext);
  return context?.startId === startId ? context : undefined;
}
