import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  isUrlTitle,
  jobViewerQuery,
  parseJobViewerUrl,
  titleLimit,
  urlTitle,
} from "../src/core/jobViewerUrl.ts";
import type {
  JobViewerParams,
  JobViewerUrlError,
} from "../src/core/jobViewerUrl.ts";
import { countCharacters } from "../src/core/summary.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";
const startId = "0123456789abcdef0123456789abcdef";
const jobId = "fedcba9876543210fedcba9876543210";

/** The query of a valid job URL with the given pairs replaced, or removed when undefined. */
function queryWith(pairs: Record<string, string | undefined>): string {
  const base: Record<string, string | undefined> = {
    k: jobId,
    u: encodeURIComponent(watchUrl),
    t: "Title",
    ...pairs,
  };
  return Object.entries(base)
    .filter(([, value]) => value !== undefined)
    .map(([name, value]) => `${name}=${value}`)
    .join("&");
}

function errorOf(search: string, hash = ""): JobViewerUrlError {
  const result = parseJobViewerUrl(search, hash);
  assert.ok(!result.ok, JSON.stringify(result));
  return result.error;
}

function paramsOf(search: string): JobViewerParams {
  const result = parseJobViewerUrl(search, "");
  assert.ok(result.ok, JSON.stringify(result));
  return result.params;
}

describe("parseJobViewerUrl", () => {
  it("reads a start URL and a job URL, with and without the existing mark", () => {
    assert.deepEqual(paramsOf(queryWith({ k: undefined, r: startId })), {
      target: { kind: "start", startId },
      videoUrl: watchUrl,
      title: "Title",
    });
    assert.deepEqual(paramsOf(`?${queryWith({})}`).target, {
      kind: "job",
      jobId,
    });
    assert.deepEqual(paramsOf(queryWith({ e: "1" })).target, {
      kind: "job",
      jobId,
      existing: true,
    });
  });

  it("rejects, by rule, what is outside the closed set of names and values", () => {
    const cases: [string, JobViewerUrlError][] = [
      [queryWith({ x: "1" }), "unknown-name"],
      [`${queryWith({})}&t=Again`, "duplicate-name"],
      [`${queryWith({})}&e`, "malformed-pair"],
      [`${queryWith({})}&e=1=2`, "malformed-pair"],
      [queryWith({ r: startId }), "both-targets"],
      [queryWith({ k: undefined }), "missing-name"],
      [queryWith({ u: undefined }), "missing-name"],
      [queryWith({ t: undefined }), "missing-name"],
      [queryWith({ k: jobId.toUpperCase() }), "id"],
      [queryWith({ k: jobId.slice(1) }), "id"],
      [queryWith({ u: "%E0%A4%A" }), "encoding"],
      [
        queryWith({ u: encodeURIComponent("https://example.com/") }),
        "url-not-accepted",
      ],
      [queryWith({ e: "2" }), "existing"],
      [queryWith({ e: "" }), "existing"],
      [queryWith({ k: undefined, r: startId, e: "1" }), "existing"],
      [queryWith({ t: "" }), "title"],
      [queryWith({ t: "a%00b" }), "title"],
    ];
    for (const [search, error] of cases) {
      assert.equal(errorOf(search), error, search);
    }
    assert.equal(errorOf(queryWith({}), "#x"), "fragment");
  });

  it("reads a plus as a plus, never as a space", () => {
    assert.equal(paramsOf(queryWith({ t: "a+b" })).title, "a+b");
  });

  it("accepts a title of exactly the limit and rejects one over it", () => {
    assert.equal(
      paramsOf(queryWith({ t: "a".repeat(titleLimit) })).title.length,
      titleLimit,
    );
    assert.equal(
      errorOf(queryWith({ t: "a".repeat(titleLimit + 1) })),
      "title",
    );
  });
});

describe("jobViewerQuery", () => {
  it("writes the names in order and reads back what it wrote", () => {
    const params: JobViewerParams = {
      target: { kind: "job", jobId, existing: true },
      videoUrl: watchUrl,
      title: "A & B = C + 日本語",
    };
    const query = jobViewerQuery(params);
    assert.deepEqual(
      query.split("&").map((pair) => pair.split("=")[0]),
      ["k", "u", "e", "t"],
    );
    assert.deepEqual(paramsOf(query), params);
    const start: JobViewerParams = {
      target: { kind: "start", startId },
      videoUrl: watchUrl,
      title: "T",
    };
    assert.deepEqual(paramsOf(jobViewerQuery(start)), start);
  });
});

describe("urlTitle", () => {
  it("keeps a title of at most the limit and cuts a longer one to the limit with an ellipsis", () => {
    assert.equal(urlTitle("short"), "short");
    const exact = "a".repeat(titleLimit);
    assert.equal(urlTitle(exact), exact);
    const cut = urlTitle("a".repeat(titleLimit + 1));
    assert.equal(cut, `${"a".repeat(titleLimit - 1)}…`);
    assert.equal(countCharacters(cut), titleLimit);
  });

  it("never splits a surrogate pair", () => {
    const cut = urlTitle("😀".repeat(titleLimit + 5));
    assert.equal(cut, `${"😀".repeat(titleLimit - 1)}…`);
    assert.equal(countCharacters(cut), titleLimit);
    assert.ok(isUrlTitle(cut));
  });
});

describe("isUrlTitle", () => {
  it("rejects empty text and every kind of control character", () => {
    assert.ok(isUrlTitle("x"));
    assert.ok(!isUrlTitle(""));
    for (const control of ["\u0000", "\u001f", "\u007f", "\u0080", "\u009f"]) {
      assert.ok(!isUrlTitle(`a${control}b`), JSON.stringify(control));
    }
    assert.ok(isUrlTitle("a\u00a0b"));
    assert.ok(!isUrlTitle("a\ud800b"));
  });
});
