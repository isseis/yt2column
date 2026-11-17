import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  parseWorkerCall,
  parseWorkerMessage,
  serveWorkerCalls,
} from "../src/core/workerCalls.ts";
import type { LoadedModule, WorkerMessage } from "../src/core/workerCalls.ts";
import { settle } from "./helpers/clock.ts";

/**
 * Serves calls against a module that loads as load decides. posted holds the
 * answers and progress; readies counts the ready messages.
 */
function serve(load: () => Promise<LoadedModule>): {
  handle: (data: unknown) => void;
  posted: WorkerMessage[];
  readies: () => number;
  reported: unknown[];
} {
  const posted: WorkerMessage[] = [];
  const reported: unknown[] = [];
  let readies = 0;
  const handle = serveWorkerCalls({
    load,
    post: (message) => {
      if (message.kind === "ready") {
        readies += 1;
      } else {
        posted.push(message);
      }
    },
    reportError: (error) => reported.push(error),
  });
  return { handle, posted, readies: () => readies, reported };
}

/** Serves calls against a module that loads at once with namespace; exit ends its Go program. */
function serveLoaded(namespace: Record<string, unknown>): {
  handle: (data: unknown) => void;
  posted: WorkerMessage[];
  exit: () => void;
} {
  let exit = (): void => {};
  const exited = new Promise<void>((resolve) => {
    exit = resolve;
  });
  const { handle, posted } = serve(async () => ({
    namespace: () => namespace,
    exited,
  }));
  return { handle, posted, exit };
}

const call = (callId: number, fn: string, request = "r"): unknown => ({
  callId,
  fn,
  request,
});

describe("serveWorkerCalls", () => {
  it("says ready once, when the module has set its namespace", async () => {
    const { handle, readies } = serve(async () => ({
      namespace: () => ({ buildInfo: () => "info" }),
      exited: new Promise<void>(() => {}),
    }));
    assert.equal(readies(), 0, "ready before the load");
    await settle();
    handle(call(1, "buildInfo", ""));
    await settle();
    assert.equal(readies(), 1);
  });

  it("never says ready for a module that registered nothing, nor for a failed load", async () => {
    const empty = serve(async () => ({
      namespace: () => undefined,
      exited: Promise.resolve(),
    }));
    const failed = serve(() => Promise.reject(new Error("no module")));
    await settle();
    assert.equal(empty.readies(), 0);
    assert.equal(failed.readies(), 0);
  });

  it("replies with the string each function returns", async () => {
    const { handle, posted } = serveLoaded({
      prepare: (request: string) => `prepared ${request}`,
      buildInfo: () => "info",
      checkSettings: async (request: string) => `checked ${request}`,
      generate: async (request: string) => `generated ${request}`,
    });
    handle(call(1, "prepare", "a"));
    handle(call(2, "buildInfo", ""));
    handle(call(3, "checkSettings", "b"));
    handle(call(4, "generate", "c"));
    await settle();
    assert.deepEqual(posted, [
      { callId: 1, kind: "reply", value: "prepared a" },
      { callId: 2, kind: "reply", value: "info" },
      { callId: 3, kind: "reply", value: "checked b" },
      { callId: 4, kind: "reply", value: "generated c" },
    ]);
  });

  it("posts generate's progress while the call waits, and none after the answer", async () => {
    let progress: (event: unknown) => void = () => {};
    let finish: (value: string) => void = () => {};
    const { handle, posted } = serveLoaded({
      generate: (_request: string, onProgress: (event: unknown) => void) => {
        progress = onProgress;
        return new Promise<string>((resolve) => {
          finish = resolve;
        });
      },
    });
    handle(call(1, "generate"));
    await settle();
    progress("p1");
    progress(7);
    finish("done");
    await settle();
    progress("late");
    assert.deepEqual(posted, [
      { callId: 1, kind: "progress", event: "p1" },
      { callId: 1, kind: "reply", value: "done" },
    ]);
  });

  it("answers error when the function's Promise rejects or resolves to a non-string", async () => {
    const { handle, posted } = serveLoaded({
      checkSettings: () => Promise.reject(new Error("panic")),
      generate: async () => 1,
    });
    handle(call(1, "checkSettings"));
    handle(call(2, "generate"));
    await settle();
    assert.deepEqual(posted, [
      { callId: 1, kind: "error" },
      { callId: 2, kind: "error" },
    ]);
  });

  const exits: {
    name: string;
    namespace: Record<string, unknown>;
    fn: string;
  }[] = [
    {
      name: "a function that throws",
      namespace: {
        prepare: () => {
          throw new Error("Go program has already exited");
        },
      },
      fn: "prepare",
    },
    {
      name: "checkSettings returning a non-Promise",
      namespace: { checkSettings: () => undefined },
      fn: "checkSettings",
    },
    {
      name: "generate returning a non-Promise",
      namespace: { generate: () => "text" },
      fn: "generate",
    },
    {
      name: "prepare returning a non-string",
      namespace: { prepare: () => undefined },
      fn: "prepare",
    },
    {
      name: "buildInfo returning a non-string",
      namespace: { buildInfo: () => Promise.resolve("x") },
      fn: "buildInfo",
    },
    {
      name: "a function the module did not register",
      namespace: {},
      fn: "buildInfo",
    },
  ];
  for (const { name, namespace, fn } of exits) {
    it(`answers exit for ${name}`, async () => {
      const { handle, posted } = serveLoaded(namespace);
      handle(call(1, fn));
      await settle();
      assert.deepEqual(posted, [{ callId: 1, kind: "exit" }]);
    });
  }

  it("answers exit when the module set no namespace", async () => {
    const { handle, posted } = serve(async () => ({
      namespace: () => undefined,
      exited: Promise.resolve(),
    }));
    handle(call(1, "buildInfo", ""));
    await settle();
    assert.deepEqual(posted, [{ callId: 1, kind: "exit" }]);
  });

  it("answers exit to every waiting call when the Go program ends, and to every later call", async () => {
    const { handle, posted, exit } = serveLoaded({
      prepare: () => "p",
      generate: () => new Promise<string>(() => {}),
      checkSettings: () => new Promise<string>(() => {}),
    });
    handle(call(1, "prepare"));
    handle(call(2, "generate"));
    handle(call(3, "checkSettings"));
    await settle();
    exit();
    await settle();
    handle(call(4, "prepare"));
    await settle();
    assert.deepEqual(posted, [
      { callId: 1, kind: "reply", value: "p" },
      { callId: 2, kind: "exit" },
      { callId: 3, kind: "exit" },
      { callId: 4, kind: "exit" },
    ]);
  });

  it("answers each call once, even when the Promise settles after the exit", async () => {
    let finish: (value: string) => void = () => {};
    const { handle, posted, exit } = serveLoaded({
      generate: () =>
        new Promise<string>((resolve) => {
          finish = resolve;
        }),
    });
    handle(call(1, "generate"));
    await settle();
    exit();
    await settle();
    finish("late");
    await settle();
    assert.deepEqual(posted, [{ callId: 1, kind: "exit" }]);
  });

  it("waits for the module before calling, and reports a failed load once without answering", async () => {
    let fail: (error: unknown) => void = () => {};
    const { handle, posted, reported } = serve(
      () =>
        new Promise<LoadedModule>((_resolve, reject) => {
          fail = reject;
        }),
    );
    handle(call(1, "prepare"));
    handle(call(2, "buildInfo", ""));
    await settle();
    assert.deepEqual(posted, []);
    const error = new Error("instantiate failed");
    fail(error);
    await settle();
    assert.deepEqual(posted, []);
    assert.deepEqual(reported, [error]);
  });

  it("ignores a malformed call and a call whose ID still waits", async () => {
    const { handle, posted } = serveLoaded({
      prepare: () => "p",
      generate: () => new Promise<string>(() => {}),
    });
    handle({ callId: 1, fn: "prepare" });
    handle(call(2, "generate"));
    handle(call(2, "prepare"));
    await settle();
    assert.deepEqual(posted, []);
  });
});

describe("parseWorkerCall", () => {
  it("accepts a call of each function", () => {
    for (const fn of ["checkSettings", "prepare", "generate", "buildInfo"]) {
      assert.deepEqual(parseWorkerCall({ callId: 1, fn, request: "" }), {
        callId: 1,
        fn,
        request: "",
      });
    }
  });

  const refused: { name: string; data: unknown }[] = [
    { name: "a missing member", data: { callId: 1, fn: "prepare" } },
    {
      name: "an unknown member",
      data: { callId: 1, fn: "prepare", request: "", extra: 1 },
    },
    {
      name: "an unknown function",
      data: { callId: 1, fn: "run", request: "" },
    },
    {
      name: "an inherited function name",
      data: { callId: 1, fn: "toString", request: "" },
    },
    { name: "a zero callId", data: { callId: 0, fn: "prepare", request: "" } },
    {
      name: "a fractional callId",
      data: { callId: 1.5, fn: "prepare", request: "" },
    },
    {
      name: "a non-string request",
      data: { callId: 1, fn: "prepare", request: 1 },
    },
    { name: "an array", data: [1, "prepare", ""] },
    { name: "null", data: null },
  ];
  for (const { name, data } of refused) {
    it(`refuses ${name}`, () => {
      assert.equal(parseWorkerCall(data), undefined);
    });
  }
});

describe("parseWorkerMessage", () => {
  it("accepts each kind with exactly its members", () => {
    const messages: WorkerMessage[] = [
      { kind: "ready" },
      { callId: 1, kind: "reply", value: "v" },
      { callId: 2, kind: "progress", event: "e" },
      { callId: 3, kind: "error" },
      { callId: 4, kind: "exit" },
    ];
    for (const message of messages) {
      assert.deepEqual(parseWorkerMessage({ ...message }), message);
    }
  });

  const refused: { name: string; data: unknown }[] = [
    { name: "a reply without a value", data: { callId: 1, kind: "reply" } },
    {
      name: "a reply with a non-string value",
      data: { callId: 1, kind: "reply", value: 1 },
    },
    {
      name: "a progress event with a value",
      data: { callId: 1, kind: "progress", value: "v" },
    },
    {
      name: "an exit with an extra member",
      data: { callId: 1, kind: "exit", value: "v" },
    },
    { name: "an error without a callId", data: { kind: "error" } },
    { name: "a ready with a callId", data: { callId: 1, kind: "ready" } },
    { name: "an unknown kind", data: { callId: 1, kind: "loaded" } },
    { name: "a string", data: "reply" },
  ];
  for (const { name, data } of refused) {
    it(`refuses ${name}`, () => {
      assert.equal(parseWorkerMessage(data), undefined);
    });
  }
});
