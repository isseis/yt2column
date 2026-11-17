// Reads the JSON strings the article generation module returns through the
// Worker: the results of its four functions and generate's progress events.
// The module is this repository's Go code, but its output crosses into the
// page as text, so each string is accepted only when it is one strict JSON
// value of at most maxResultBytes whose members are exactly the documented set
// for its status, with every code in its closed set. Anything else is the
// internal failure result_shape: no part of it is used, and nothing is
// repaired. The types here are the extension-only path's; the server path's
// response.ts goes away with the server.

import {
  ShapeMismatch,
  booleanMember,
  codeMember,
  discriminant,
  exactMembers,
  isIn,
  member,
  patternMember,
  readShaped,
  safeInteger,
  stringMember,
} from "./jsonShape.ts";
import type { Members } from "./jsonShape.ts";
import type { JsonValue } from "./strictJson.ts";

/** The longest result string accepted, in UTF-8 bytes: the page-to-page limit. */
export const maxResultBytes = 512 * 1024;

export type StepName = "draft" | "shorten" | "review";

export type StepState = "pending" | "running" | "done" | "skipped" | "failed";

const stepStates: Readonly<Record<StepState, true>> = {
  pending: true,
  running: true,
  done: true,
  skipped: true,
  failed: true,
};

export type Steps = Readonly<Record<StepName, StepState>>;

/** Why a run failed inside the article generation module. */
export type WasmFailure =
  | "internal"
  | "transcript_too_long"
  | "llm_timeout"
  | "llm_empty"
  | "llm_truncated"
  | "llm_failed"
  | "article_invalid"
  | "article_too_large"
  | "api_key_rejected"
  | "settings_invalid";

export const wasmFailures: Readonly<Record<WasmFailure, true>> = {
  internal: true,
  transcript_too_long: true,
  llm_timeout: true,
  llm_empty: true,
  llm_truncated: true,
  llm_failed: true,
  article_invalid: true,
  article_too_large: true,
  api_key_rejected: true,
  settings_invalid: true,
};

/** The kind of an internal failure; for the record and the developer log only. */
export type InternalDetail =
  | "panic"
  | "worker_error"
  | "worker_load"
  | "result_shape"
  | "store_failed"
  | "input_mismatch"
  | "request_shape"
  | "settings_read"
  | "runtime_exit";

export const internalDetails: Readonly<Record<InternalDetail, true>> = {
  panic: true,
  worker_error: true,
  worker_load: true,
  result_shape: true,
  store_failed: true,
  input_mismatch: true,
  request_shape: true,
  settings_read: true,
  runtime_exit: true,
};

/** The internal details the module itself reports; the others are the page's. */
export type ModuleInternalDetail = Extract<
  InternalDetail,
  "panic" | "input_mismatch" | "request_shape"
>;

const moduleInternalDetails: Readonly<Record<ModuleInternalDetail, true>> = {
  panic: true,
  input_mismatch: true,
  request_shape: true,
};

declare const usdBrand: unique symbol;

/** One to six digits, ".", four digits: the module's cost text. Only wasmResult.ts creates it. */
export type UsdAmount = `${number}` & { readonly [usdBrand]: true };

// Six integer digits cover any cost below one million dollars; a longer run
// of digits is no article's cost and is never handed to the formatter.
const usdPattern = /^[0-9]{1,6}\.[0-9]{4}$/;

export type Cost =
  | { readonly state: "known"; readonly usd: UsdAmount }
  | { readonly state: "unknown" };

/** A done article as the module made it. */
export interface ArticleResult {
  /** The neutralized display Markdown: drawn rendered and copied unchanged. */
  readonly markdown: string;
  /** Whether markdown neutralized at least one mention. */
  readonly mentionsNeutralized: boolean;
  /** The neutralized video title and channel name ("" for none), for the heading. */
  readonly videoTitle: string;
  readonly channelName: string;
  /** The file content, saved unchanged. */
  readonly file: string;
  readonly cost: Cost;
  readonly durationMs: number;
}

export type SettingsField = "anthropicApiKey" | "anthropicWorkspaceId";

const settingsFields: Readonly<Record<SettingsField, true>> = {
  anthropicApiKey: true,
  anthropicWorkspaceId: true,
};

/** checkSettings' result. */
export type SettingsCheckResult =
  | { readonly status: "ok" }
  | { readonly status: "rejected"; readonly fields: readonly SettingsField[] }
  | { readonly status: "key_rejected" }
  | { readonly status: "unverified" };

/** prepare's result. */
export type PrepareResult =
  | {
      readonly status: "ok";
      /** 64 lowercase hexadecimal characters; compared only. */
      readonly inputDigest: string;
      readonly videoId: string;
    }
  | { readonly status: "rejected"; readonly reason: "invalid_body" };

/** generate's result. internalDetail is present only for some internal failures. */
export type GenerateResult =
  | {
      readonly status: "done";
      readonly steps: Steps;
      readonly article: ArticleResult;
    }
  | {
      readonly status: "failed";
      readonly steps: Steps;
      readonly failure: WasmFailure;
      readonly internalDetail?: ModuleInternalDetail;
    };

/** One of generate's progress events. */
export interface ProgressEvent {
  readonly steps: Steps;
}

/** The scenario a development build embedded. */
export interface FakeScenario {
  readonly article: string;
  readonly failure: string;
  readonly failAt: string;
  readonly delaySeconds: number;
  readonly settingsCheck: string;
}

/** buildInfo's result. llm is absent when the module declared no implementation. */
export interface BuildInfo {
  readonly goVersion: string;
  readonly vcsRevision: string;
  readonly vcsModified: boolean;
  readonly mainPath: string;
  readonly llm?:
    | { readonly kind: "claude" }
    | { readonly kind: "fake"; readonly scenario: FakeScenario };
}

/** A read result string: the value, or the internal failure result_shape. */
export type WasmRead<T> =
  | { readonly ok: true; readonly value: T }
  | { readonly ok: false; readonly detail: "result_shape" };

/** Reads text with read, as a WasmRead. */
function readResult<T>(
  text: string,
  read: (value: JsonValue) => T,
): WasmRead<T> {
  const value = readShaped(text, maxResultBytes, read);
  return value === undefined
    ? { ok: false, detail: "result_shape" }
    : { ok: true, value };
}

/** Reads a run's steps: exactly the three, each in its closed set. */
export function readSteps(value: JsonValue): Steps {
  const members = exactMembers(value, ["draft", "shorten", "review"]);
  return {
    draft: codeMember(members, "draft", stepStates),
    shorten: codeMember(members, "shorten", stepStates),
    review: codeMember(members, "review", stepStates),
  };
}

/** Reads a done run's steps: every step finished or was not needed. */
export function readFinishedSteps(value: JsonValue): Steps {
  const result = readSteps(value);
  for (const state of Object.values(result)) {
    if (state !== "done" && state !== "skipped") {
      throw new ShapeMismatch();
    }
  }
  return result;
}

/** A non-empty string member: the article's text is never empty. */
function textMember(members: Members, name: string): string {
  const value = stringMember(members, name);
  if (value === "") {
    throw new ShapeMismatch();
  }
  return value;
}

function cost(value: JsonValue): Cost {
  switch (discriminant(value, "state")) {
    case "known":
      return {
        state: "known",
        usd: patternMember(
          exactMembers(value, ["state", "usd"]),
          "usd",
          usdPattern,
        ) as UsdAmount,
      };
    case "unknown":
      exactMembers(value, ["state"]);
      return { state: "unknown" };
    default:
      throw new ShapeMismatch();
  }
}

/** Reads a done article: exactly its members, with non-empty text and file. */
export function articleResult(value: JsonValue): ArticleResult {
  const members = exactMembers(value, [
    "markdown",
    "mentionsNeutralized",
    "videoTitle",
    "channelName",
    "file",
    "cost",
    "durationMs",
  ]);
  return {
    markdown: textMember(members, "markdown"),
    mentionsNeutralized: booleanMember(members, "mentionsNeutralized"),
    videoTitle: stringMember(members, "videoTitle"),
    channelName: stringMember(members, "channelName"),
    file: textMember(members, "file"),
    cost: cost(member(members, "cost")),
    durationMs: safeInteger(member(members, "durationMs")),
  };
}

/** Reads checkSettings' result. */
export function readSettingsCheckResult(
  text: string,
): WasmRead<SettingsCheckResult> {
  return readResult(text, (value): SettingsCheckResult => {
    const status = discriminant(value, "status");
    switch (status) {
      case "ok":
      case "key_rejected":
      case "unverified":
        exactMembers(value, ["status"]);
        return { status };
      case "rejected": {
        const fieldsValue = member(
          exactMembers(value, ["status", "fields"]),
          "fields",
        );
        if (fieldsValue.kind !== "array" || fieldsValue.items.length === 0) {
          throw new ShapeMismatch();
        }
        const fields = fieldsValue.items.map((item) => {
          if (item.kind !== "string" || !isIn(settingsFields, item.value)) {
            throw new ShapeMismatch();
          }
          return item.value;
        });
        if (new Set(fields).size !== fields.length) {
          throw new ShapeMismatch();
        }
        return { status, fields };
      }
      default:
        throw new ShapeMismatch();
    }
  });
}

/** An input digest: 64 lowercase hexadecimal characters. */
export const inputDigestPattern = /^[0-9a-f]{64}$/;
/** A YouTube video ID: 11 characters of [A-Za-z0-9_-]. */
export const videoIdPattern = /^[A-Za-z0-9_-]{11}$/;

/** Reads prepare's result. */
export function readPrepareResult(text: string): WasmRead<PrepareResult> {
  return readResult(text, (value): PrepareResult => {
    const status = discriminant(value, "status");
    switch (status) {
      case "ok": {
        const members = exactMembers(value, [
          "status",
          "inputDigest",
          "videoId",
        ]);
        return {
          status,
          inputDigest: patternMember(
            members,
            "inputDigest",
            inputDigestPattern,
          ),
          videoId: patternMember(members, "videoId", videoIdPattern),
        };
      }
      case "rejected":
        return {
          status,
          reason: codeMember(
            exactMembers(value, ["status", "reason"]),
            "reason",
            { invalid_body: true },
          ),
        };
      default:
        throw new ShapeMismatch();
    }
  });
}

/** Reads generate's result. */
export function readGenerateResult(text: string): WasmRead<GenerateResult> {
  return readResult(text, (value): GenerateResult => {
    const status = discriminant(value, "status");
    switch (status) {
      case "done": {
        const members = exactMembers(value, ["status", "steps", "article"]);
        return {
          status,
          steps: readFinishedSteps(member(members, "steps")),
          article: articleResult(member(members, "article")),
        };
      }
      case "failed": {
        // Only an internal failure may carry a detail, and need not.
        const withDetail =
          value.kind === "object" && value.members.has("internalDetail");
        const members = exactMembers(
          value,
          withDetail
            ? ["status", "steps", "failure", "internalDetail"]
            : ["status", "steps", "failure"],
        );
        const failure = codeMember(members, "failure", wasmFailures);
        const runSteps = readSteps(member(members, "steps"));
        if (!withDetail) {
          return { status, steps: runSteps, failure };
        }
        if (failure !== "internal") {
          throw new ShapeMismatch();
        }
        return {
          status,
          steps: runSteps,
          failure,
          internalDetail: codeMember(
            members,
            "internalDetail",
            moduleInternalDetails,
          ),
        };
      }
      default:
        throw new ShapeMismatch();
    }
  });
}

/** Reads one of generate's progress events. */
export function readProgressEvent(text: string): WasmRead<ProgressEvent> {
  return readResult(text, (value) => ({
    steps: readSteps(member(exactMembers(value, ["steps"]), "steps")),
  }));
}

function fakeScenario(value: JsonValue): FakeScenario {
  const members = exactMembers(value, [
    "article",
    "failure",
    "failAt",
    "delaySeconds",
    "settingsCheck",
  ]);
  return {
    article: stringMember(members, "article"),
    failure: stringMember(members, "failure"),
    failAt: stringMember(members, "failAt"),
    delaySeconds: safeInteger(member(members, "delaySeconds")),
    settingsCheck: stringMember(members, "settingsCheck"),
  };
}

function llmInfo(value: JsonValue): NonNullable<BuildInfo["llm"]> {
  switch (discriminant(value, "kind")) {
    case "claude":
      exactMembers(value, ["kind"]);
      return { kind: "claude" };
    case "fake":
      return {
        kind: "fake",
        scenario: fakeScenario(
          member(exactMembers(value, ["kind", "scenario"]), "scenario"),
        ),
      };
    default:
      throw new ShapeMismatch();
  }
}

/** Reads buildInfo's result. */
export function readBuildInfo(text: string): WasmRead<BuildInfo> {
  return readResult(text, (value): BuildInfo => {
    const withLlm = value.kind === "object" && value.members.has("llm");
    const base = ["goVersion", "vcsRevision", "vcsModified", "mainPath"];
    const members = exactMembers(value, withLlm ? [...base, "llm"] : base);
    const info = {
      goVersion: stringMember(members, "goVersion"),
      vcsRevision: stringMember(members, "vcsRevision"),
      vcsModified: booleanMember(members, "vcsModified"),
      mainPath: stringMember(members, "mainPath"),
    };
    return withLlm ? { ...info, llm: llmInfo(member(members, "llm")) } : info;
  });
}
