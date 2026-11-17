import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { BuildKind } from "../src/buildKind.d.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { viewerTitle } from "../src/core/viewerTitle.ts";
import type { TitleSource } from "../src/core/viewerTitle.ts";

const localeList = Object.keys(locales) as Locale[];
const buildKinds: BuildKind[] = ["release", "dev"];

describe("viewerTitle", () => {
  // One row per row of the design's title table (§3.8.2), with plan B for
  // the viewer that has neither a record nor a start context.
  const rows: { name: string; source: TitleSource; en: string; ja: string }[] =
    [
      {
        name: "a record's video title",
        source: { kind: "record", videoTitle: "Video", heading: "Tab" },
        en: "Video - yt2column",
        ja: "Video - yt2column",
      },
      {
        name: "a record's heading when it has no video title",
        source: { kind: "record", videoTitle: "", heading: "Tab" },
        en: "Tab - yt2column",
        ja: "Tab - yt2column",
      },
      {
        name: "a start context's video title",
        source: { kind: "start", videoTitle: "Video", heading: "Tab" },
        en: "Video - yt2column",
        ja: "Video - yt2column",
      },
      {
        name: "a start context's heading when it has no video title",
        source: { kind: "start", videoTitle: "", heading: "Tab" },
        en: "Tab - yt2column",
        ja: "Tab - yt2column",
      },
      {
        name: "the URL's title when neither is left",
        source: { kind: "url", urlTitle: "From URL" },
        en: "From URL - yt2column",
        ja: "From URL - yt2column",
      },
      {
        name: "the fixed title of an invalid link",
        source: { kind: "invalid-link" },
        en: "Invalid link - yt2column",
        ja: "正しくないリンク - yt2column",
      },
    ];
  for (const { name, source, en, ja } of rows) {
    it(`titles ${name}`, () => {
      assert.equal(viewerTitle(source, "release", "en"), en);
      assert.equal(viewerTitle(source, "release", "ja"), ja);
    });
  }

  it("ends with the development build's name in a development build", () => {
    const source: TitleSource = { kind: "url", urlTitle: "Video" };
    assert.equal(
      viewerTitle(source, "dev", "en"),
      "Video - yt2column (development build)",
    );
    assert.equal(
      viewerTitle(source, "dev", "ja"),
      "Video - yt2column（開発用のビルド）",
    );
  });

  it("falls back to a fixed text when no title is known", () => {
    assert.equal(
      viewerTitle(
        { kind: "record", videoTitle: "", heading: "" },
        "release",
        "en",
      ),
      "Untitled video - yt2column",
    );
  });

  it("never returns a title with an empty part, for any input", () => {
    const sources: TitleSource[] = [
      { kind: "record", videoTitle: "", heading: "" },
      { kind: "start", videoTitle: "", heading: "" },
      { kind: "url", urlTitle: "" },
      { kind: "invalid-link" },
    ];
    for (const source of sources) {
      for (const buildKind of buildKinds) {
        for (const locale of localeList) {
          const title = viewerTitle(source, buildKind, locale);
          assert.ok(!title.startsWith(" - "), title);
          assert.notEqual(title.trim(), "");
        }
      }
    }
  });
});
