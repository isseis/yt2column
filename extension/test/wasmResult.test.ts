import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  maxResultBytes,
  readBuildInfo,
  readGenerateResult,
  readPrepareResult,
  readProgressEvent,
  readSettingsCheckResult,
} from "../src/core/wasmResult.ts";
import type { WasmRead } from "../src/core/wasmResult.ts";

const refused = { ok: false, detail: "result_shape" } as const;

/** Asserts read refuses each of texts as result_shape. */
function expectRefused(
  read: (text: string) => WasmRead<unknown>,
  rows: readonly { name: string; text: string }[],
): void {
  for (const { name, text } of rows) {
    it(`refuses ${name}`, () => {
      assert.deepEqual(read(text), refused);
    });
  }
}

const doneSteps = '{"draft":"done","shorten":"skipped","review":"done"}';
const article = (overrides: Record<string, string> = {}): string => {
  const members: Record<string, string> = {
    markdown: '"# Title\\n\\nBody."',
    mentionsNeutralized: "false",
    videoTitle: '"Title"',
    channelName: '""',
    file: '"---\\ntitle: Title\\n---\\n"',
    cost: '{"state":"known","usd":"0.1234"}',
    durationMs: "1500",
    ...overrides,
  };
  return `{${Object.entries(members)
    .map(([name, value]) => `"${name}":${value}`)
    .join(",")}}`;
};
const done = (articleText = article(), stepsText = doneSteps): string =>
  `{"status":"done","steps":${stepsText},"article":${articleText}}`;

describe("readGenerateResult", () => {
  it("reads a done result", () => {
    assert.deepEqual(readGenerateResult(done()), {
      ok: true,
      value: {
        status: "done",
        steps: { draft: "done", shorten: "skipped", review: "done" },
        article: {
          markdown: "# Title\n\nBody.",
          mentionsNeutralized: false,
          videoTitle: "Title",
          channelName: "",
          file: "---\ntitle: Title\n---\n",
          cost: { state: "known", usd: "0.1234" },
          durationMs: 1500,
        },
      },
    });
  });

  it("reads an unknown cost", () => {
    const read = readGenerateResult(
      done(article({ cost: '{"state":"unknown"}' })),
    );
    assert.ok(read.ok && read.value.status === "done");
    assert.deepEqual(read.value.article.cost, { state: "unknown" });
  });

  it("reads a failed result, with and without an internal detail", () => {
    const steps = '{"draft":"failed","shorten":"pending","review":"pending"}';
    assert.deepEqual(
      readGenerateResult(
        `{"status":"failed","steps":${steps},"failure":"api_key_rejected"}`,
      ),
      {
        ok: true,
        value: {
          status: "failed",
          steps: { draft: "failed", shorten: "pending", review: "pending" },
          failure: "api_key_rejected",
        },
      },
    );
    assert.deepEqual(
      readGenerateResult(
        `{"status":"failed","steps":${steps},"failure":"internal"}`,
      ).ok,
      true,
    );
    for (const detail of ["panic", "input_mismatch", "request_shape"]) {
      assert.deepEqual(
        readGenerateResult(
          `{"status":"failed","steps":${steps},"failure":"internal","internalDetail":"${detail}"}`,
        ),
        {
          ok: true,
          value: {
            status: "failed",
            steps: { draft: "failed", shorten: "pending", review: "pending" },
            failure: "internal",
            internalDetail: detail,
          },
        },
      );
    }
  });

  const pending = '{"draft":"pending","shorten":"pending","review":"pending"}';
  expectRefused(readGenerateResult, [
    {
      name: "an unknown status",
      text: `{"status":"running","steps":${pending}}`,
    },
    {
      name: "a duplicate member",
      text: `{"status":"done","status":"done","steps":${doneSteps},"article":${article()}}`,
    },
    {
      name: "a duplicate member inside the article",
      text: done(article({ videoTitle: '"a","videoTitle":"b"' })),
    },
    {
      name: "an unknown member",
      text: `{"status":"done","steps":${doneSteps},"article":${article()},"extra":1}`,
    },
    {
      name: "an unknown article member",
      text: done(article({ apiKey: '"x"' })),
    },
    {
      name: "a done result with a failure",
      text: `{"status":"done","steps":${doneSteps},"article":${article()},"failure":"internal"}`,
    },
    {
      name: "a failed result with an article",
      text: `{"status":"failed","steps":${pending},"failure":"internal","article":${article()}}`,
    },
    {
      name: "a done result whose step did not finish",
      text: done(
        article(),
        '{"draft":"done","shorten":"running","review":"done"}',
      ),
    },
    {
      name: "an unknown step state",
      text: done(article(), doneSteps.replace("skipped", "paused")),
    },
    { name: "an empty Markdown", text: done(article({ markdown: '""' })) },
    { name: "an empty file", text: done(article({ file: '""' })) },
    {
      name: "a non-boolean neutralized mark",
      text: done(article({ mentionsNeutralized: '"false"' })),
    },
    {
      name: "a cost with more digits",
      text: done(article({ cost: '{"state":"known","usd":"0.12345"}' })),
    },
    {
      name: "a cost given as a number",
      text: done(article({ cost: '{"state":"known","usd":0.1234}' })),
    },
    {
      name: "an unknown cost with an amount",
      text: done(article({ cost: '{"state":"unknown","usd":"0.1234"}' })),
    },
    {
      name: "a fractional duration",
      text: done(article({ durationMs: "1.5" })),
    },
    { name: "a negative duration", text: done(article({ durationMs: "-1" })) },
    {
      name: "a duration beyond a safe integer",
      text: done(article({ durationMs: "9007199254740992" })),
    },
    {
      name: "an unknown failure",
      text: `{"status":"failed","steps":${pending},"failure":"api_key_missing"}`,
    },
    {
      name: "a detail on a failure that is not internal",
      text: `{"status":"failed","steps":${pending},"failure":"llm_failed","internalDetail":"panic"}`,
    },
    {
      name: "a detail only the page reports",
      text: `{"status":"failed","steps":${pending},"failure":"internal","internalDetail":"worker_load"}`,
    },
    { name: "a JSON array", text: "[]" },
    { name: "text after the value", text: `${done()} x` },
  ]);
});

describe("the size limit", () => {
  // Pads the Markdown so the whole result is bytes long in UTF-8, with
  // three-byte characters so a character count would differ.
  const sized = (bytes: number): string => {
    const base = done(article({ markdown: '"#"' }));
    let pad = "あ".repeat(Math.floor((bytes - base.length) / 3));
    pad += "a".repeat(bytes - base.length - 3 * [...pad].length);
    const text = done(article({ markdown: `"#${pad}"` }));
    assert.equal(new TextEncoder().encode(text).length, bytes);
    return text;
  };

  it("accepts a result of exactly the limit", () => {
    assert.equal(readGenerateResult(sized(maxResultBytes)).ok, true);
  });

  it("refuses a result one byte over the limit", () => {
    assert.deepEqual(readGenerateResult(sized(maxResultBytes + 1)), refused);
  });

  it("refuses a text with an unpaired surrogate instead of replacing it", () => {
    assert.deepEqual(
      readGenerateResult(done(article({ videoTitle: '"a\uD800b"' }))),
      refused,
    );
  });
});

describe("readSettingsCheckResult", () => {
  for (const status of ["ok", "key_rejected", "unverified"]) {
    it(`reads ${status}`, () => {
      assert.deepEqual(readSettingsCheckResult(`{"status":"${status}"}`), {
        ok: true,
        value: { status },
      });
    });
  }

  it("reads the rejected fields", () => {
    assert.deepEqual(
      readSettingsCheckResult(
        '{"status":"rejected","fields":["anthropicApiKey","anthropicWorkspaceId"]}',
      ),
      {
        ok: true,
        value: {
          status: "rejected",
          fields: ["anthropicApiKey", "anthropicWorkspaceId"],
        },
      },
    );
  });

  expectRefused(readSettingsCheckResult, [
    { name: "ok with fields", text: '{"status":"ok","fields":[]}' },
    { name: "rejected without fields", text: '{"status":"rejected"}' },
    {
      name: "rejected with no field",
      text: '{"status":"rejected","fields":[]}',
    },
    {
      name: "an unknown field",
      text: '{"status":"rejected","fields":["model"]}',
    },
    {
      name: "a repeated field",
      text: '{"status":"rejected","fields":["anthropicApiKey","anthropicApiKey"]}',
    },
    {
      name: "a field that is not a string",
      text: '{"status":"rejected","fields":[1]}',
    },
    { name: "an unknown status", text: '{"status":"saved"}' },
    { name: "a status that is not a string", text: '{"status":true}' },
  ]);
});

describe("readPrepareResult", () => {
  const digest = "0123456789abcdef".repeat(4);

  it("reads an accepted input", () => {
    assert.deepEqual(
      readPrepareResult(
        `{"status":"ok","inputDigest":"${digest}","videoId":"dQw4w9WgXcQ"}`,
      ),
      {
        ok: true,
        value: { status: "ok", inputDigest: digest, videoId: "dQw4w9WgXcQ" },
      },
    );
  });

  it("reads a rejected input", () => {
    assert.deepEqual(
      readPrepareResult('{"status":"rejected","reason":"invalid_body"}'),
      { ok: true, value: { status: "rejected", reason: "invalid_body" } },
    );
  });

  expectRefused(readPrepareResult, [
    {
      name: "an uppercase digest",
      text: `{"status":"ok","inputDigest":"${digest.toUpperCase()}","videoId":"dQw4w9WgXcQ"}`,
    },
    {
      name: "a short digest",
      text: `{"status":"ok","inputDigest":"${digest.slice(1)}","videoId":"dQw4w9WgXcQ"}`,
    },
    {
      name: "a video ID of another length",
      text: `{"status":"ok","inputDigest":"${digest}","videoId":"dQw4w9WgXc"}`,
    },
    {
      name: "an accepted input with a reason",
      text: `{"status":"ok","inputDigest":"${digest}","videoId":"dQw4w9WgXcQ","reason":"invalid_body"}`,
    },
    {
      name: "an unknown reason",
      text: '{"status":"rejected","reason":"body_too_large"}',
    },
  ]);
});

describe("readProgressEvent", () => {
  it("reads the steps", () => {
    assert.deepEqual(
      readProgressEvent(
        '{"steps":{"draft":"running","shorten":"pending","review":"pending"}}',
      ),
      {
        ok: true,
        value: {
          steps: { draft: "running", shorten: "pending", review: "pending" },
        },
      },
    );
  });

  expectRefused(readProgressEvent, [
    {
      name: "a missing step",
      text: '{"steps":{"draft":"running","shorten":"pending"}}',
    },
    {
      name: "an extra member",
      text: `{"steps":${doneSteps},"status":"done"}`,
    },
  ]);
});

describe("readBuildInfo", () => {
  const base =
    '"goVersion":"go1.26.5","vcsRevision":"","vcsModified":false,"mainPath":"github.com/isseis/yt2column/cmd/yt2column-wasm"';
  const info = {
    goVersion: "go1.26.5",
    vcsRevision: "",
    vcsModified: false,
    mainPath: "github.com/isseis/yt2column/cmd/yt2column-wasm",
  };

  it("reads the release module's information", () => {
    assert.deepEqual(readBuildInfo(`{${base},"llm":{"kind":"claude"}}`), {
      ok: true,
      value: { ...info, llm: { kind: "claude" } },
    });
  });

  it("reads a development module's scenario", () => {
    const scenario =
      '{"article":"standard","failure":"none","failAt":"none","delaySeconds":3,"settingsCheck":"ok"}';
    assert.deepEqual(
      readBuildInfo(`{${base},"llm":{"kind":"fake","scenario":${scenario}}}`),
      {
        ok: true,
        value: {
          ...info,
          llm: {
            kind: "fake",
            scenario: {
              article: "standard",
              failure: "none",
              failAt: "none",
              delaySeconds: 3,
              settingsCheck: "ok",
            },
          },
        },
      },
    );
  });

  it("reads a module that declared no implementation", () => {
    assert.deepEqual(readBuildInfo(`{${base}}`), { ok: true, value: info });
  });

  expectRefused(readBuildInfo, [
    {
      name: "a claude implementation with a scenario",
      text: `{${base},"llm":{"kind":"claude","scenario":{}}}`,
    },
    {
      name: "a fake implementation without a scenario",
      text: `{${base},"llm":{"kind":"fake"}}`,
    },
    {
      name: "an unknown implementation",
      text: `{${base},"llm":{"kind":"openai"}}`,
    },
    {
      name: "a missing Go version",
      text: '{"vcsRevision":"","vcsModified":false,"mainPath":"m"}',
    },
    {
      name: "a modified mark given as text",
      text: `{${base.replace("false", '"false"')}}`,
    },
  ]);
});
