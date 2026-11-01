import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { isAcceptedWatchUrl, watchVideoId } from "../src/core/acceptedUrl.ts";

// Every accepted URL has the video ID abc.
const accepted = [
  "https://www.youtube.com/watch?v=abc",
  "https://www.youtube.com/watch?v=abc&t=42s",
  "https://www.youtube.com/watch?v=abc&list=PL123",
  "https://www.youtube.com/watch?v=abc#comments",
  "https://www.youtube.com:443/watch?v=abc",
  // The parser lowercases the host and resolves dot segments; the
  // decision uses the parsed URL.
  "https://WWW.YouTube.com/watch?v=abc",
  "https://www.youtube.com/a/../watch?v=abc",
];

const rejected = [
  "http://www.youtube.com/watch?v=abc",
  "https://youtube.com/watch?v=abc",
  "https://m.youtube.com/watch?v=abc",
  "https://music.youtube.com/watch?v=abc",
  "https://www.youtube.com.example/watch?v=abc",
  "https://www.youtube.com./watch?v=abc",
  "https://www.youtube.com:8443/watch?v=abc",
  "https://user@www.youtube.com/watch?v=abc",
  "https://:secret@www.youtube.com/watch?v=abc",
  "https://www.youtube.com/",
  "https://www.youtube.com/watch",
  "https://www.youtube.com/watch?v=",
  "https://www.youtube.com/watch?v=abc&v=def",
  "https://www.youtube.com/watch/?v=abc",
  "https://www.youtube.com/watchlater?v=abc",
  "https://www.youtube.com/WATCH?v=abc",
  "https://www.youtube.com/shorts/abc",
  "https://youtu.be/abc",
  "chrome://extensions/",
  "not a url",
  "",
];

describe("isAcceptedWatchUrl", () => {
  for (const url of accepted) {
    it(`accepts ${url}`, () => {
      assert.equal(isAcceptedWatchUrl(url), true);
    });
  }
  for (const url of rejected) {
    it(`rejects ${JSON.stringify(url)}`, () => {
      assert.equal(isAcceptedWatchUrl(url), false);
    });
  }
});

describe("watchVideoId", () => {
  for (const url of accepted) {
    it(`returns the v of ${url}`, () => {
      assert.equal(watchVideoId(url), "abc");
    });
  }
  // isAcceptedWatchUrl is this parse, so its table above covers the rest.
  for (const url of [
    "https://www.youtube.com/watch?v=",
    "https://www.youtube.com/watch?v=abc&v=def",
    "https://youtu.be/abc",
  ]) {
    it(`returns nothing for ${JSON.stringify(url)}`, () => {
      assert.equal(watchVideoId(url), undefined);
    });
  }

  it("returns v as the URL parser decodes it", () => {
    assert.equal(
      watchVideoId("https://www.youtube.com/watch?v=a%2Fb&list=PL1"),
      "a/b",
    );
  });
});
