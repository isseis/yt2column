import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  classifyPublish,
  classifyStart,
  classifyStatus,
  parseRequestId,
  publishLogView,
  startLogView,
  statusLogView,
  toPublishOutcome,
  toStartOutcome,
  toStatusOutcome,
} from "../src/core/response.ts";
import type { RequestId, StatusOutcomeLog } from "../src/core/response.ts";
import type { ArticleKey } from "../src/core/articleKey.ts";
import { classifyHostReply } from "../src/core/native.ts";
import type {
  ExchangeFailureLog,
  HostOutcome,
  UnexpectedCheck,
} from "../src/core/exchange.ts";
import { responseReply } from "./helpers/native.ts";

// Tests may assert a branded type to build fixtures.
const requestId = "0123456789abcdef" as RequestId;
const key = "key-0123" as ArticleKey;

/** A relayed response whose body is the UTF-8 encoding of text. */
function response(status: number, text: string): HostOutcome {
  return { kind: "response", status, bytes: new TextEncoder().encode(text) };
}

/** A relayed response whose body is value as JSON. */
function json(status: number, value: unknown): HostOutcome {
  return response(status, JSON.stringify(value));
}

/** A relayed response with raw bytes, for the cases JSON.stringify cannot write. */
function raw(status: number, bytes: readonly number[]): HostOutcome {
  return { kind: "response", status, bytes: new Uint8Array(bytes) };
}

/** The result of an unexpected reply at status. */
function unexpected(
  check: UnexpectedCheck,
  status: number,
): { kind: "unexpected"; check: UnexpectedCheck; status: number } {
  return { kind: "unexpected", check, status };
}

type Json = Record<string, unknown>;

/** A well-formed done status body; each AC case below changes one value of it. */
function doneBody(): Json {
  return {
    status: "ok",
    requestId,
    webhookConfigured: true,
    job: {
      state: "done",
      steps: { draft: "done", shorten: "skipped", review: "done" },
      article: {
        markdown: "# Title\n\nBody",
        file: "---\ntitle: Title\n---\n\nBody\n",
        cost: { state: "known", usd: "0.1234" },
        durationMs: 81234,
      },
      publish: { state: "not_posted" },
    },
  };
}

/** The member at path inside body, which every case below has built. */
function at(body: Json, ...path: string[]): Json {
  let current = body;
  for (const name of path) {
    current = current[name] as Json;
  }
  return current;
}

describe("parseRequestId", () => {
  it("accepts 16 lowercase hex characters", () => {
    assert.equal(parseRequestId(requestId), requestId);
  });

  const rejected = [
    "0123456789ABCDEF",
    "0123456789abcde",
    "0123456789abcdef0",
    "",
  ];
  for (const value of rejected) {
    it(`rejects ${JSON.stringify(value)}`, () => {
      assert.equal(parseRequestId(value), undefined);
    });
  }
});

describe("exchange failures", () => {
  const failures: HostOutcome[] = [
    { kind: "not-sent", cause: "server-unavailable" },
    { kind: "no-response" },
    { kind: "host-failed", lastError: "Native host has exited." },
    { kind: "unexpected", check: "envelope-shape" },
  ];
  for (const failure of failures) {
    it(`passes ${failure.kind} through every operation unchanged`, () => {
      assert.deepEqual(classifyStart(failure), failure);
      assert.deepEqual(classifyStatus(failure), failure);
      assert.deepEqual(classifyPublish(failure), failure);
    });
  }
});

describe("classifyStart", () => {
  it("accepts a well-formed accepted response", () => {
    assert.deepEqual(
      classifyStart(json(202, { status: "accepted", key, requestId })),
      { kind: "accepted", key, requestId },
    );
  });

  // The extension's version of the host relays a start to /articles; the
  // server reads no videoUrl and answers this, the form of a valid rejection.
  it("reads an old host's /articles answer as an invalid_body rejection", () => {
    const answer = `{"status":"rejected","reason":"invalid_body","requestId":"${requestId}"}`;
    assert.deepEqual(classifyStart(response(400, answer)), {
      kind: "rejected",
      reason: "invalid_body",
      requestId,
    });
  });

  const refused: {
    name: string;
    received: HostOutcome;
    check: UnexpectedCheck;
  }[] = [
    {
      name: "an empty key",
      received: json(202, { status: "accepted", key: "", requestId }),
      check: "value-shape",
    },
    {
      name: "a key over the length limit",
      received: json(202, {
        status: "accepted",
        key: "k".repeat(257),
        requestId,
      }),
      check: "value-shape",
    },
    {
      name: "a key that is not a string",
      received: json(202, { status: "accepted", key: 1, requestId }),
      check: "value-shape",
    },
    {
      name: "an accepted response without a key",
      received: json(202, { status: "accepted", requestId }),
      check: "member-set",
    },
    {
      name: "a status query's ok",
      received: json(202, { status: "ok", key, requestId }),
      check: "member-set",
    },
  ];
  for (const { name, received, check } of refused) {
    it(`refuses ${name}`, () => {
      assert.deepEqual(classifyStart(received), unexpected(check, 202));
    });
  }

  it("refuses an accepted response at 200", () => {
    assert.deepEqual(
      classifyStart(json(200, { status: "accepted", key, requestId })),
      unexpected("status-pairing", 200),
    );
  });
});

describe("classifyPublish", () => {
  it("accepts a well-formed accepted response", () => {
    assert.deepEqual(
      classifyPublish(json(202, { status: "accepted", requestId })),
      { kind: "accepted", requestId },
    );
  });

  it("refuses an accepted response with a key", () => {
    assert.deepEqual(
      classifyPublish(json(202, { status: "accepted", key, requestId })),
      unexpected("member-set", 202),
    );
  });

  it("refuses an accepted response at 200", () => {
    assert.deepEqual(
      classifyPublish(json(200, { status: "accepted", requestId })),
      unexpected("status-pairing", 200),
    );
  });
});

// The reasons each operation's path returns and their status codes, from the
// server's handlers (internal/server); invalid_body is a start's only.
const reasonTable: {
  readonly name: string;
  readonly classify: (received: HostOutcome) => { readonly kind: string };
  readonly reasons: Readonly<Record<string, number>>;
}[] = [
  {
    name: "classifyStart",
    classify: classifyStart,
    reasons: {
      too_many_requests: 429,
      body_too_large: 413,
      invalid_body: 400,
      invalid_video_url: 400,
      invalid_transcript: 400,
      internal_error: 500,
    },
  },
  {
    name: "classifyStatus",
    classify: classifyStatus,
    reasons: { invalid_key: 400, internal_error: 500 },
  },
  {
    name: "classifyPublish",
    classify: classifyPublish,
    reasons: {
      invalid_key: 400,
      webhook_not_configured: 503,
      article_not_ready: 409,
      article_not_found: 404,
      article_not_postable: 409,
      internal_error: 500,
    },
  },
];

// Every reason the server has, with its status code: the ones outside an
// operation's set are unexpected for it.
const allReasons: Readonly<Record<string, number>> = {
  ...reasonTable[0]?.reasons,
  ...reasonTable[1]?.reasons,
  ...reasonTable[2]?.reasons,
  not_found: 404,
  method_not_allowed: 405,
};

describe("reject reasons", () => {
  for (const { name, classify, reasons } of reasonTable) {
    for (const [reason, status] of Object.entries(allReasons)) {
      const body = { status: "rejected", reason, requestId };
      if (Object.hasOwn(reasons, reason)) {
        it(`${name} accepts ${reason} at ${status}`, () => {
          assert.deepEqual(classify(json(status, body)), {
            kind: "rejected",
            reason,
            requestId,
          });
        });
        it(`${name} refuses ${reason} at another status`, () => {
          assert.deepEqual(
            classify(json(status + 1, body)),
            unexpected("status-pairing", status + 1),
          );
        });
      } else {
        it(`${name} refuses ${reason}, outside its set`, () => {
          assert.deepEqual(
            classify(json(status, body)),
            unexpected("status-pairing", status),
          );
        });
      }
    }
  }

  it("refuses an internal_error with an empty request id", () => {
    const body = {
      status: "rejected",
      reason: "internal_error",
      requestId: "",
    };
    for (const { classify } of reasonTable) {
      assert.deepEqual(
        classify(json(500, body)),
        unexpected("request-id", 500),
      );
    }
  });

  it("refuses a rejection without a reason", () => {
    const body = { status: "rejected", requestId };
    for (const { classify } of reasonTable) {
      assert.deepEqual(
        classify(json(400, body)),
        unexpected("member-set", 400),
      );
    }
  });
});

describe("classifyStatus", () => {
  it("accepts the done response the cases below each change one value of", () => {
    assert.deepEqual(classifyStatus(json(200, doneBody())), {
      kind: "ok",
      requestId,
      webhookConfigured: true,
      job: {
        state: "done",
        steps: { draft: "done", shorten: "skipped", review: "done" },
        article: {
          markdown: "# Title\n\nBody",
          file: "---\ntitle: Title\n---\n\nBody\n",
          cost: { state: "known", usd: "0.1234" },
          durationMs: 81234,
        },
        publish: { state: "not_posted" },
      },
    });
  });

  const steps = { draft: "done", shorten: "running", review: "pending" };
  const jobs: { name: string; job: Json }[] = [
    { name: "an unknown job", job: { state: "unknown" } },
    { name: "a running job", job: { state: "running", steps } },
    {
      name: "a failed job",
      job: {
        state: "failed",
        steps: { draft: "failed", shorten: "pending", review: "pending" },
        failure: "llm_timeout",
      },
    },
  ];
  for (const { name, job } of jobs) {
    it(`accepts ${name}`, () => {
      const body = { ...doneBody(), webhookConfigured: false, job };
      assert.deepEqual(classifyStatus(json(200, body)), {
        kind: "ok",
        requestId,
        webhookConfigured: false,
        job,
      });
    });
  }

  const accepted: { name: string; change: (body: Json) => void }[] = [
    {
      name: "an unknown cost",
      change: (b) => {
        at(b, "job", "article").cost = { state: "unknown" };
      },
    },
    {
      name: "the largest cost",
      change: (b) => {
        at(b, "job", "article", "cost").usd = "999999.9999";
      },
    },
    {
      name: "a zero duration",
      change: (b) => {
        at(b, "job", "article").durationMs = 0;
      },
    },
    {
      name: "a duration of 2^53-1",
      change: (b) => {
        at(b, "job", "article").durationMs = Number.MAX_SAFE_INTEGER;
      },
    },
    ...["posting", "posted"].map((state) => ({
      name: `a ${state} article`,
      change: (b: Json) => {
        at(b, "job").publish = { state };
      },
    })),
    ...[
      ["failed", "webhook"],
      ["failed", "internal"],
      ["unpostable", "mention"],
      ["unpostable", "other"],
    ].map(([state, reason]) => ({
      name: `a ${state} publish for ${reason}`,
      change: (b: Json) => {
        at(b, "job").publish = { state, reason };
      },
    })),
  ];
  for (const { name, change } of accepted) {
    it(`accepts ${name}`, () => {
      const body = doneBody();
      change(body);
      assert.deepEqual(classifyStatus(json(200, body)), {
        kind: "ok",
        requestId,
        webhookConfigured: true,
        job: body.job,
      });
    });
  }
});

describe("classifyStatus refuses each malformed response as unexpected", () => {
  const changed: {
    name: string;
    change: (body: Json) => void;
    check: UnexpectedCheck;
  }[] = [
    // A member that is not declared, at each level.
    {
      name: "an undeclared top-level member",
      change: (b) => void (b.extra = 1),
      check: "member-set",
    },
    {
      name: "an undeclared job member",
      change: (b) => void (at(b, "job").extra = 1),
      check: "member-set",
    },
    {
      name: "an undeclared article member",
      change: (b) => void (at(b, "job", "article").title = "x"),
      check: "member-set",
    },
    {
      name: "an undeclared cost member",
      change: (b) => void (at(b, "job", "article", "cost").currency = "usd"),
      check: "member-set",
    },
    {
      name: "a usd member on an unknown cost",
      change: (b) =>
        void (at(b, "job", "article").cost = {
          state: "unknown",
          usd: "0.1234",
        }),
      check: "member-set",
    },
    {
      name: "an undeclared step",
      change: (b) => void (at(b, "job", "steps").publish = "done"),
      check: "member-set",
    },
    {
      name: "a reason on a not_posted publish",
      change: (b) =>
        void (at(b, "job").publish = { state: "not_posted", reason: "other" }),
      check: "member-set",
    },
    {
      name: "an article on a running job",
      change: (b) => void (at(b, "job").state = "running"),
      check: "member-set",
    },
    // A member that is missing, at each level.
    {
      name: "a missing status",
      change: (b) => void delete b.status,
      check: "member-set",
    },
    {
      name: "a missing webhookConfigured",
      change: (b) => void delete b.webhookConfigured,
      check: "member-set",
    },
    {
      name: "a missing publish",
      change: (b) => void delete at(b, "job").publish,
      check: "member-set",
    },
    {
      name: "a missing file",
      change: (b) => void delete at(b, "job", "article").file,
      check: "member-set",
    },
    {
      name: "a missing usd",
      change: (b) => void delete at(b, "job", "article", "cost").usd,
      check: "member-set",
    },
    {
      name: "a missing step",
      change: (b) => void delete at(b, "job", "steps").review,
      check: "member-set",
    },
    {
      name: "a failed publish without a reason",
      change: (b) => void (at(b, "job").publish = { state: "failed" }),
      check: "member-set",
    },
    // A code outside its closed set.
    {
      name: "an unknown job state",
      change: (b) => void (at(b, "job").state = "queued"),
      check: "value-shape",
    },
    {
      name: "an unknown step state",
      change: (b) => void (at(b, "job", "steps").draft = "queued"),
      check: "value-shape",
    },
    {
      name: "a running step in a done job",
      change: (b) => void (at(b, "job", "steps").review = "running"),
      check: "value-shape",
    },
    {
      name: "an unknown publish state",
      change: (b) => void (at(b, "job").publish = { state: "queued" }),
      check: "value-shape",
    },
    {
      name: "an unpostable reason on a failed publish",
      change: (b) =>
        void (at(b, "job").publish = { state: "failed", reason: "mention" }),
      check: "value-shape",
    },
    {
      name: "a failed reason on an unpostable publish",
      change: (b) =>
        void (at(b, "job").publish = {
          state: "unpostable",
          reason: "webhook",
        }),
      check: "value-shape",
    },
    {
      name: "an unknown cost state",
      change: (b) => void (at(b, "job", "article", "cost").state = "estimated"),
      check: "value-shape",
    },
    {
      name: "an unknown failure code",
      change: (b) =>
        void (b.job = {
          state: "failed",
          steps: { draft: "failed", shorten: "pending", review: "pending" },
          failure: "llm_unknown",
        }),
      check: "value-shape",
    },
    // durationMs that is not a decimal integer up to 2^53-1.
    {
      name: "a negative duration",
      change: (b) => void (at(b, "job", "article").durationMs = -1),
      check: "value-shape",
    },
    {
      name: "a fractional duration",
      change: (b) => void (at(b, "job", "article").durationMs = 1.5),
      check: "value-shape",
    },
    {
      name: "a duration over 2^53-1",
      change: (b) =>
        void (at(b, "job", "article").durationMs = Number.MAX_SAFE_INTEGER + 1),
      check: "value-shape",
    },
    {
      name: "a duration as a string",
      change: (b) => void (at(b, "job", "article").durationMs = "81234"),
      check: "value-shape",
    },
    // cost.usd outside the server's form.
    ...[
      "1.234",
      "1.23456",
      "1234567.0000",
      "+1.1234",
      ".1234",
      "1,1234",
      "0x1.1234",
    ].map((usd) => ({
      name: `the cost ${usd}`,
      change: (b: Json) => void (at(b, "job", "article", "cost").usd = usd),
      check: "value-shape" as const,
    })),
    {
      name: "a cost as a number",
      change: (b) => void (at(b, "job", "article", "cost").usd = 0.1234),
      check: "value-shape",
    },
    // Empty article text, and other value shapes.
    {
      name: "an empty markdown",
      change: (b) => void (at(b, "job", "article").markdown = ""),
      check: "value-shape",
    },
    {
      name: "an empty file",
      change: (b) => void (at(b, "job", "article").file = ""),
      check: "value-shape",
    },
    {
      name: "a webhookConfigured string",
      change: (b) => void (b.webhookConfigured = "true"),
      check: "value-shape",
    },
    {
      name: "a job that is not an object",
      change: (b) => void (b.job = "done"),
      check: "value-shape",
    },
    {
      name: "an article that is null",
      change: (b) => void (at(b, "job").article = null),
      check: "value-shape",
    },
    {
      name: "an uppercase request id",
      change: (b) => void (b.requestId = "0123456789ABCDEF"),
      check: "request-id",
    },
  ];
  for (const { name, change, check } of changed) {
    it(`refuses ${name} as ${check}`, () => {
      const body = doneBody();
      change(body);
      assert.deepEqual(classifyStatus(json(200, body)), unexpected(check, 200));
    });
  }

  it("refuses an ok response at 202 as status-pairing", () => {
    assert.deepEqual(
      classifyStatus(json(202, doneBody())),
      unexpected("status-pairing", 202),
    );
  });

  const text = JSON.stringify(doneBody());
  const encoded = new TextEncoder().encode(text);
  const markdownAt = text.indexOf("# Title");
  /** text with replacement put in place of the first occurrence of target. */
  const replaced = (target: string, replacement: string): HostOutcome => {
    assert.ok(text.includes(target), target);
    return response(200, text.replace(target, replacement));
  };

  const written: {
    name: string;
    received: HostOutcome;
    check: UnexpectedCheck;
  }[] = [
    {
      name: "a duplicate top-level member",
      received: replaced('"status":"ok"', '"status":"ok","status":"ok"'),
      check: "duplicate-member",
    },
    {
      name: "a duplicate nested member",
      received: replaced(
        '"durationMs":81234',
        '"durationMs":81234,"durationMs":1',
      ),
      check: "duplicate-member",
    },
    {
      name: "an unpaired high surrogate",
      received: replaced("# Title", "\\ud800# Title"),
      check: "unpaired-surrogate",
    },
    {
      name: "an unpaired low surrogate",
      received: replaced("# Title", "\\udc00# Title"),
      check: "unpaired-surrogate",
    },
    {
      name: "invalid UTF-8",
      received: raw(200, [
        ...encoded.subarray(0, markdownAt),
        0xff,
        ...encoded.subarray(markdownAt),
      ]),
      check: "encoding",
    },
    {
      name: "a leading BOM",
      received: raw(200, [0xef, 0xbb, 0xbf, ...encoded]),
      check: "bom",
    },
    {
      name: "a trailing object",
      received: response(200, `${text} {}`),
      check: "trailing-data",
    },
    {
      name: "a trailing character",
      received: response(200, `${text}x`),
      check: "trailing-data",
    },
    {
      name: "a duration written 1.0",
      received: replaced('"durationMs":81234', '"durationMs":1.0'),
      check: "value-shape",
    },
    {
      name: "a duration written 1e3",
      received: replaced('"durationMs":81234', '"durationMs":1e3'),
      check: "value-shape",
    },
    {
      name: "a duration written -0",
      received: replaced('"durationMs":81234', '"durationMs":-0'),
      check: "value-shape",
    },
    {
      name: "a duration written 081234",
      received: replaced('"durationMs":81234', '"durationMs":081234'),
      check: "syntax",
    },
    // The top-level rows of internal/strictjson's rejection table: strict JSON
    // that is not an object.
    {
      name: "a top-level null",
      received: response(200, "null"),
      check: "value-shape",
    },
    {
      name: "a top-level array",
      received: response(200, "[]"),
      check: "value-shape",
    },
    {
      name: "a top-level string",
      received: response(200, '"x"'),
      check: "value-shape",
    },
    {
      name: "a top-level number",
      received: response(200, "42"),
      check: "value-shape",
    },
  ];
  for (const { name, received, check } of written) {
    it(`refuses ${name} as ${check}`, () => {
      assert.deepEqual(classifyStatus(received), unexpected(check, 200));
    });
  }
});

describe("a status response at the server's 256 KiB limit", () => {
  // Go's json.Marshal escapes <, >, & and U+2028, U+2029 as \u sequences;
  // JSON.stringify writes them as they are, so the escapes are put in after.
  const goEscapes: readonly [string, string][] = [
    ["<", "\\u003c"],
    [">", "\\u003e"],
    ["&", "\\u0026"],
    ["\u2028", "\\u2028"],
    ["\u2029", "\\u2029"],
  ];
  const marshal = (value: unknown): string => {
    let text = JSON.stringify(value);
    for (const [char, escape] of goEscapes) {
      text = text.replaceAll(char, escape);
    }
    return text;
  };
  const limit = 256 * 1024;
  // Multi-byte text (U+65E5 U+672C U+8A9E) is counted in bytes, not characters.
  const sample = "<b>&</b>\u2028\u2029 \u65e5\u672c\u8a9e ";

  /** A done body whose markdown and file are sample followed by fill. */
  const bodyWith = (fill: string): Json => {
    const body = doneBody();
    at(body, "job", "article").markdown = `# ${sample}${fill}`;
    at(body, "job", "article").file = `---\n${sample}${fill}`;
    return body;
  };
  const size = (text: string): number =>
    new TextEncoder().encode(text).byteLength;
  // Each ASCII fill character adds two bytes, one to each string; an odd
  // remainder is made up by one more character in the file's text.
  const base = size(marshal(bodyWith("")));
  const fill = "a".repeat(Math.floor((limit - base) / 2));
  const body = bodyWith(fill);
  if ((limit - base) % 2 === 1) {
    at(body, "job", "article").file += "a";
  }
  const text = marshal(body);

  it("fills the limit exactly and carries every Go escape", () => {
    assert.equal(size(text), limit);
    for (const [, escape] of goEscapes) {
      assert.ok(text.includes(escape), escape);
    }
  });

  it("is accepted through the host reply with the article unchanged", () => {
    const result = classifyStatus(classifyHostReply(responseReply(200, text)));
    assert.equal(result.kind, "ok");
    assert.deepEqual(result.job, body.job);
  });
});

describe("to...Outcome", () => {
  const unexpectedLog = {
    kind: "unexpected",
    check: "syntax",
    status: 200,
  } as const;
  const hostFailedLog = {
    kind: "host-failed",
    lastError: "Native host has exited.",
  } as const;

  it("drops the status of an unexpected reply and the browser's error text", () => {
    for (const toOutcome of [
      toStartOutcome,
      toStatusOutcome,
      toPublishOutcome,
    ]) {
      assert.deepEqual(toOutcome(unexpectedLog), {
        kind: "unexpected",
        check: "syntax",
      });
      assert.deepEqual(toOutcome(hostFailedLog), { kind: "host-failed" });
    }
  });

  it("keeps a server's answer whole", () => {
    const accepted = classifyStart(
      json(202, { status: "accepted", key, requestId }),
    );
    assert.deepEqual(toStartOutcome(accepted), {
      kind: "accepted",
      key,
      requestId,
    });
    const ok = classifyStatus(json(200, doneBody()));
    assert.deepEqual(toStatusOutcome(ok), ok);
  });
});

describe("log views", () => {
  const marker = "MARKER-7f3a";
  const markedKey = `${marker}-key` as ArticleKey;

  it("leaves the key out of a start's view", () => {
    const view = startLogView(
      classifyStart(
        json(202, { status: "accepted", key: markedKey, requestId }),
      ),
    );
    assert.deepEqual(view, { kind: "accepted", requestId });
  });

  it("leaves the article out of a status query's view", () => {
    const body = doneBody();
    at(body, "job", "article").markdown = `${marker} markdown`;
    at(body, "job", "article").file = `${marker} file`;
    const log = classifyStatus(json(200, body));
    assert.ok(
      JSON.stringify(log).includes(marker),
      "the marker reached the result",
    );
    const view = statusLogView(log);
    assert.equal(
      JSON.stringify(view).includes(marker),
      false,
      JSON.stringify(view),
    );
    assert.deepEqual(view, {
      kind: "ok",
      requestId,
      job: {
        state: "done",
        steps: { draft: "done", shorten: "skipped", review: "done" },
        publish: { state: "not_posted" },
        costKnown: true,
      },
    });
  });

  it("keeps a failed job's codes in a status query's view", () => {
    const job = {
      state: "failed",
      steps: { draft: "failed", shorten: "pending", review: "pending" },
      failure: "llm_failed",
    };
    const view = statusLogView(
      classifyStatus(json(200, { ...doneBody(), job })),
    );
    assert.deepEqual(view, { kind: "ok", requestId, job });
  });

  it("keeps the reason, the status and the browser's error text", () => {
    const rejected = {
      status: "rejected",
      reason: "internal_error",
      requestId,
    };
    const expected = { kind: "rejected", reason: "internal_error", requestId };
    assert.deepEqual(
      startLogView(classifyStart(json(500, rejected))),
      expected,
    );
    assert.deepEqual(
      statusLogView(classifyStatus(json(500, rejected))),
      expected,
    );
    assert.deepEqual(
      publishLogView(classifyPublish(json(500, rejected))),
      expected,
    );
    const failures: ExchangeFailureLog[] = [
      { kind: "unexpected", check: "syntax", status: 200 },
      { kind: "host-failed", lastError: "Native host has exited." },
      { kind: "not-sent", cause: "message-too-large" },
      { kind: "no-response" },
    ];
    for (const failure of failures) {
      assert.deepEqual(startLogView(failure), failure);
      assert.deepEqual(statusLogView(failure), failure);
      assert.deepEqual(publishLogView(failure), failure);
    }
  });

  it("copies only the declared members of a failure", () => {
    const extra = {
      kind: "no-response",
      key: markedKey,
    } as unknown as StatusOutcomeLog;
    assert.deepEqual(statusLogView(extra), { kind: "no-response" });
  });
});
