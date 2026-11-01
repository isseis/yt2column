import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { ArticleKey } from "../src/core/articleKey.ts";
import { parseViewerUrl, viewerQuery } from "../src/core/viewerUrl.ts";
import type {
  ViewerParams,
  ViewerSeed,
  ViewerUrlError,
} from "../src/core/viewerUrl.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";
const sentAt = 1_700_000_000_000;

/** A watch URL of exactly length code points, padded in an ignored member. */
function watchUrlOfLength(length: number): string {
  const prefix = `${watchUrl}&x=`;
  return prefix + "a".repeat(length - prefix.length);
}

/** The error of a rejected parse, asserting it was rejected. */
function errorOf(search: string, hash = "", loadedAt = sentAt): ViewerUrlError {
  const result = parseViewerUrl(search, hash, loadedAt);
  assert.ok(!result.ok, JSON.stringify(result).slice(0, 200));
  return result.error;
}

/** The params of an accepted parse, asserting it was accepted. */
function paramsOf(search: string, loadedAt = sentAt): ViewerParams {
  const result = parseViewerUrl(search, "", loadedAt);
  assert.ok(result.ok, JSON.stringify(result));
  return result.params;
}

/** The query of a minimal valid URL with one pair replaced or appended. */
function queryWith(pairs: Record<string, string>): string {
  const base: Record<string, string> = {
    h: "Heading",
    u: encodeURIComponent(watchUrl),
    s: String(sentAt),
  };
  const merged = { ...base, ...pairs };
  return (
    "?" +
    Object.entries(merged)
      .map(([name, value]) => `${name}=${value}`)
      .join("&")
  );
}

describe("round trip", () => {
  const key = "key+/=&%#\u{1F600}" as ArticleKey;
  const cases: { name: string; params: ViewerParams }[] = [
    {
      name: "the required values only",
      params: { heading: "Heading", url: watchUrl, sentAt },
    },
    {
      name: "every value, with the characters the query reserves",
      params: {
        heading: "a+b %41 & c=d #e",
        channelName: "+&=%#",
        title: "50% off? a/b:c",
        url: `${watchUrl}&t=1+2#x`,
        sentAt,
        key,
      },
    },
    {
      name: "characters outside the BMP, U+FFFD and line breaks",
      params: {
        heading: "\u{1F600}\ufffd\n\r\t",
        channelName: "\u{20000}",
        title: "\ufffd",
        url: watchUrl,
        sentAt: 0,
      },
    },
    {
      name: "every value at its limit",
      params: {
        // Counted in code points: each U+1F600 is two UTF-16 units.
        heading: "\u{1F600}".repeat(1_001),
        channelName: "\u{1F600}".repeat(201),
        title: "\u{1F600}".repeat(1_000),
        url: watchUrlOfLength(2_097_152),
        sentAt: Number.MAX_SAFE_INTEGER,
        key: "\u{1F600}".repeat(256) as ArticleKey,
      },
    },
  ];
  for (const { name, params } of cases) {
    it(`reads back ${name}`, () => {
      const query = viewerQuery(params);
      assert.equal(
        query.includes("+"),
        false,
        "a + is written as %2B, so it is never read as a space",
      );
      assert.deepEqual(paramsOf(`?${query}`, params.sentAt), params);
    });
  }

  it("writes the names in the table's order", () => {
    const query = viewerQuery({
      heading: "h",
      channelName: "c",
      title: "t",
      url: watchUrl,
      sentAt,
      key: "k" as ArticleKey,
    });
    assert.deepEqual(
      query.split("&").map((pair) => pair.split("=")[0]),
      ["h", "c", "t", "u", "s", "k"],
    );
  });
});

describe("no transcript or description", () => {
  it("writes none of a seed's other members", () => {
    const transcriptMarker = "TRANSCRIPT-MARKER";
    const descriptionMarker = "DESCRIPTION-MARKER";
    // A wider object passed where a ViewerSeed is expected, as a caller that
    // spread its input into the seed would: only the seed's members are written.
    const input = {
      heading: "Heading",
      url: watchUrl,
      sentAt,
      transcript: transcriptMarker,
      description: descriptionMarker,
    };
    const seed: ViewerSeed = input;
    const query = viewerQuery(seed);
    const decoded = decodeURIComponent(query);
    for (const marker of [transcriptMarker, descriptionMarker]) {
      assert.equal(decoded.includes(marker), false, marker);
    }
  });
});

describe("parseViewerUrl", () => {
  it("accepts the query with or without its leading ?", () => {
    const query = queryWith({}).slice(1);
    assert.deepEqual(paramsOf(query), paramsOf(`?${query}`));
  });

  it("reads a raw + as +, never as a space", () => {
    assert.equal(paramsOf(queryWith({ h: "a+b" })).heading, "a+b");
  });

  it("accepts a send time equal to the load time", () => {
    assert.equal(paramsOf(queryWith({}), sentAt).sentAt, sentAt);
  });

  const rejected: { name: string; search: string; error: ViewerUrlError }[] = [
    {
      name: "a name outside the table",
      search: queryWith({ x: "1" }),
      error: "unknown-name",
    },
    {
      name: "an encoded name",
      search: queryWith({ "%68": "a" }),
      error: "unknown-name",
    },
    {
      name: "a pair without a name",
      search: `${queryWith({})}&=a`,
      error: "unknown-name",
    },
    {
      name: "a repeated name",
      search: `${queryWith({})}&h=Other`,
      error: "duplicate-name",
    },
    {
      name: "a repeated optional name",
      search: `${queryWith({ c: "a" })}&c=a`,
      error: "duplicate-name",
    },
    {
      name: "a pair without =",
      search: `${queryWith({})}&c`,
      error: "malformed-pair",
    },
    {
      name: "a pair with two =",
      search: queryWith({ h: "a=b" }),
      error: "malformed-pair",
    },
    {
      name: "an empty pair",
      search: `${queryWith({})}&`,
      error: "malformed-pair",
    },
    {
      name: "an empty pair in the middle",
      search: queryWith({}).replace("&", "&&"),
      error: "malformed-pair",
    },
    { name: "an empty query", search: "", error: "missing-name" },
    {
      name: "a missing h",
      search: `?u=${encodeURIComponent(watchUrl)}&s=1`,
      error: "missing-name",
    },
    { name: "a missing u", search: "?h=a&s=1", error: "missing-name" },
    {
      name: "a missing s",
      search: `?h=a&u=${encodeURIComponent(watchUrl)}`,
      error: "missing-name",
    },
    {
      name: "a truncated percent-encoding",
      search: queryWith({ h: "%E3%81" }),
      error: "encoding",
    },
    {
      name: "a percent sign without digits",
      search: queryWith({ h: "100%" }),
      error: "encoding",
    },
    {
      name: "a byte that is not UTF-8",
      search: queryWith({ h: "%FF" }),
      error: "encoding",
    },
    {
      name: "an encoded surrogate",
      search: queryWith({ h: "%ED%A0%80" }),
      error: "encoding",
    },
    {
      name: "an overlong encoding",
      search: queryWith({ h: "%C0%AF" }),
      error: "encoding",
    },
    { name: "an empty h", search: queryWith({ h: "" }), error: "empty-value" },
    { name: "an empty c", search: queryWith({ c: "" }), error: "empty-value" },
    { name: "an empty t", search: queryWith({ t: "" }), error: "empty-value" },
    { name: "an empty u", search: queryWith({ u: "" }), error: "empty-value" },
    { name: "an empty s", search: queryWith({ s: "" }), error: "empty-value" },
    {
      name: "an h one over its limit",
      search: queryWith({ h: "a".repeat(1_002) }),
      error: "too-long",
    },
    {
      name: "a c one over its limit",
      search: queryWith({ c: "a".repeat(202) }),
      error: "too-long",
    },
    {
      name: "a t one over its limit",
      search: queryWith({ t: "a".repeat(1_001) }),
      error: "too-long",
    },
    {
      name: "a u one over its limit",
      search: queryWith({ u: encodeURIComponent(watchUrlOfLength(2_097_153)) }),
      error: "too-long",
    },
    {
      name: "an s of seventeen digits",
      search: queryWith({ s: "1".repeat(17) }),
      error: "too-long",
    },
    {
      name: "a javascript: URL",
      search: queryWith({ u: "javascript%3Aalert(1)" }),
      error: "url-not-accepted",
    },
    {
      name: "a watch URL on another host",
      search: queryWith({
        u: encodeURIComponent("https://example.com/watch?v=abc"),
      }),
      error: "url-not-accepted",
    },
    {
      name: "a fractional send time",
      search: queryWith({ s: "1.5" }),
      error: "sent-at",
    },
    {
      name: "a negative send time",
      search: queryWith({ s: "-1" }),
      error: "sent-at",
    },
    {
      name: "a send time with a leading zero",
      search: queryWith({ s: "01" }),
      error: "sent-at",
    },
    {
      name: "a send time with an exponent",
      search: queryWith({ s: "1e3" }),
      error: "sent-at",
    },
    {
      name: "a send time with a plus sign",
      search: queryWith({ s: "+1" }),
      error: "sent-at",
    },
    {
      name: "a send time after the load time",
      search: queryWith({ s: String(sentAt + 1) }),
      error: "sent-at",
    },
    { name: "an empty key", search: queryWith({ k: "" }), error: "key" },
    {
      name: "a key one over its limit",
      search: queryWith({ k: "k".repeat(257) }),
      error: "key",
    },
  ];
  for (const { name, search, error } of rejected) {
    it(`rejects ${name} as ${error}`, () => {
      assert.equal(errorOf(search), error);
    });
  }

  it("rejects a sixteen-digit send time above 2^53 - 1", () => {
    const loadedAt = Number.MAX_VALUE;
    assert.equal(
      errorOf(queryWith({ s: "9007199254740992" }), "", loadedAt),
      "sent-at",
    );
  });

  it("rejects a non-empty fragment", () => {
    assert.equal(errorOf(queryWith({}), "#x"), "fragment");
  });
});
