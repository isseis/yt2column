import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import type { BuildKind } from "../src/buildKind.d.ts";
import { catalogs } from "../src/core/i18n/catalog.ts";
import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import { viewerTitle } from "../src/core/viewerTitle.ts";
import type { TitleSource } from "../src/core/viewerTitle.ts";
import { extensionDir } from "./helpers/paths.ts";

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
        en: "Video - YomuDoga",
        ja: "Video - ヨムドウガ",
      },
      {
        name: "a record's heading when it has no video title",
        source: { kind: "record", videoTitle: "", heading: "Tab" },
        en: "Tab - YomuDoga",
        ja: "Tab - ヨムドウガ",
      },
      {
        name: "a start context's video title",
        source: { kind: "start", videoTitle: "Video", heading: "Tab" },
        en: "Video - YomuDoga",
        ja: "Video - ヨムドウガ",
      },
      {
        name: "a start context's heading when it has no video title",
        source: { kind: "start", videoTitle: "", heading: "Tab" },
        en: "Tab - YomuDoga",
        ja: "Tab - ヨムドウガ",
      },
      {
        name: "the URL's title when neither is left",
        source: { kind: "url", urlTitle: "From URL" },
        en: "From URL - YomuDoga",
        ja: "From URL - ヨムドウガ",
      },
      {
        name: "the fixed title of an invalid link",
        source: { kind: "invalid-link" },
        en: "Invalid link - YomuDoga",
        ja: "正しくないリンク - ヨムドウガ",
      },
    ];
  for (const { name, source, en, ja } of rows) {
    it(`titles ${name}`, () => {
      assert.equal(viewerTitle(source, "release", "en"), en);
      assert.equal(viewerTitle(source, "release", "ja"), ja);
    });
  }

  it("keeps the manifest names consistent with the viewer title, in each locale", () => {
    // The release title ends with the core product name (viewerTitle.name),
    // and the manifest's release name is that name plus the store's note. The
    // development title is the extNameDev message, which the build writes.
    for (const locale of localeList) {
      const messages = JSON.parse(
        readFileSync(
          path.join(
            extensionDir,
            "static",
            "_locales",
            locale,
            "messages.json",
          ),
          "utf8",
        ),
      ) as Record<string, { message: string }>;
      const coreName = catalogs[locale].viewerTitle.name.release;
      const source: TitleSource = { kind: "url", urlTitle: "V" };
      assert.equal(
        messages["extName"]?.message,
        `${coreName} for YouTube™`,
        locale,
      );
      assert.equal(
        viewerTitle(source, "dev", locale),
        `V - ${messages["extNameDev"]?.message}`,
        locale,
      );
    }
  });

  it("treats a white-space title as no title", () => {
    assert.equal(
      viewerTitle(
        { kind: "start", videoTitle: "  ", heading: "Tab" },
        "release",
        "en",
      ),
      "Tab - YomuDoga",
    );
    assert.equal(
      viewerTitle({ kind: "url", urlTitle: " " }, "release", "en"),
      "Untitled video - YomuDoga",
    );
  });

  it("never returns a title with an empty or white-space part, for any input", () => {
    const sources: TitleSource[] = [
      { kind: "record", videoTitle: "", heading: "" },
      { kind: "start", videoTitle: " ", heading: "\t" },
      { kind: "url", urlTitle: "" },
      { kind: "url", urlTitle: "  " },
      { kind: "invalid-link" },
    ];
    for (const source of sources) {
      for (const buildKind of buildKinds) {
        for (const locale of localeList) {
          const title = viewerTitle(source, buildKind, locale);
          const part = title.slice(0, title.lastIndexOf(" - "));
          assert.notEqual(part.trim(), "", title);
        }
      }
    }
  });
});
