import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, describe, it } from "node:test";

import { JSDOM } from "jsdom";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { previewCases } from "../scripts/popup-preview.ts";
import { extensionDir } from "./helpers/paths.ts";

/** The input sets the plan's step 5-3 requires the script to write. */
const requiredCases = [
  "worst-case-en",
  "worst-case-ja",
  "tail-ltr-en",
  "tail-rtl-en",
  "tail-mixed-en",
  "tail-mark-en",
  "rtl-all-en",
  "bidi-control-en",
  "read-failure-en",
  "rejected-en",
  "rejected-ja",
] as const;

/** The page's element with id, asserting it exists. */
function byId(doc: Document, id: string): HTMLElement {
  const element = doc.getElementById(id);
  assert.ok(element !== null, `#${id}`);
  return element;
}

/** Asserts the page shows what a collected view shows. */
function expectCollectedSections(doc: Document, file: string): void {
  for (const id of ["video", "checks", "action", "details"]) {
    assert.equal(byId(doc, id).hidden, false, `${file}: #${id} is hidden`);
  }
  assert.equal(
    byId(doc, "rejection").hidden,
    true,
    `${file}: #rejection shown`,
  );
}

/** Asserts the page shows only the rejection card and the action section. */
function expectRejectedSections(doc: Document, file: string): void {
  for (const id of ["video", "checks", "details"]) {
    assert.equal(byId(doc, id).hidden, true, `${file}: #${id} shown`);
  }
  assert.equal(
    byId(doc, "rejection").hidden,
    false,
    `${file}: #rejection hidden`,
  );
  assert.equal(byId(doc, "action").hidden, false, `${file}: #action hidden`);
  assert.ok(
    doc.getElementById("rejection-text") !== null,
    `${file}: no rejection text`,
  );
}

describe("popup preview script", () => {
  const dirs: string[] = [];

  after(() => {
    for (const dir of dirs) {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  it("writes one static page per input set and the stylesheet", () => {
    const output = mkdtempSync(path.join(tmpdir(), "popup-preview-"));
    dirs.push(output);
    const result = spawnSync(
      process.execPath,
      ["scripts/popup-preview.ts", output],
      { cwd: extensionDir, encoding: "utf8" },
    );
    assert.equal(result.status, 0, result.stderr);
    assert.ok(existsSync(path.join(output, "style.css")), "style.css");

    for (const name of requiredCases) {
      assert.ok(
        previewCases.some((previewCase) => previewCase.name === name),
        `missing case ${name}`,
      );
    }

    for (const previewCase of previewCases) {
      const file = `${previewCase.name}.html`;
      const full = path.join(output, file);
      assert.ok(existsSync(full), file);
      const doc = new JSDOM(readFileSync(full, "utf8")).window.document;
      const primary = byId(doc, "primary");
      assert.equal(
        primary.textContent,
        catalogs[previewCase.locale].primaryButton,
        `${file}: button name`,
      );
      assert.equal(
        doc.querySelectorAll("script").length,
        0,
        `${file}: holds a script`,
      );
      assert.equal(
        doc.documentElement.lang,
        previewCase.locale,
        `${file}: wrong page language`,
      );
      // The preview loads only the stylesheet copied beside it.
      const links = [...doc.querySelectorAll("link")];
      assert.equal(links.length, 1, `${file}: link count`);
      assert.equal(
        links[0]?.getAttribute("href"),
        "style.css",
        `${file}: stylesheet`,
      );
      assert.equal(
        doc.querySelectorAll("img, iframe, object, embed").length,
        0,
        `${file}: embeds a resource`,
      );

      if (previewCase.kind === "rejected") {
        expectRejectedSections(doc, file);
        continue;
      }
      expectCollectedSections(doc, file);
      assert.equal(
        doc.querySelectorAll(".check-row").length,
        2,
        `${file}: check rows`,
      );
      const warnings = doc.querySelectorAll(".check-row-warning").length;

      switch (previewCase.name) {
        case "worst-case-en":
        case "worst-case-ja":
          assert.equal(warnings, 2, `${file}: warning rows`);
          assert.ok(
            doc.querySelector(".channel-name") !== null,
            `${file}: no channel name`,
          );
          assert.ok(
            doc.querySelector(".preview-tail") !== null,
            `${file}: no tail preview`,
          );
          break;
        case "read-failure-en":
          assert.equal(
            doc.querySelector("h1")?.textContent,
            "A tab title",
            `${file}: heading is not the tab title`,
          );
          assert.equal(
            doc.querySelector(".channel-name"),
            null,
            `${file}: channel name shown`,
          );
          assert.equal(warnings, 1, `${file}: warning rows`);
          break;
        case "rtl-all-en":
          // The head preview starts with the selection's first character, so
          // its text starts with a Hebrew letter.
          assert.match(
            doc.querySelector(".preview-head")?.textContent ?? "",
            /^\p{Script=Hebrew}/u,
            `${file}: head preview`,
          );
          assert.ok(
            doc.querySelector(".preview-tail") !== null,
            `${file}: no tail preview`,
          );
          assert.equal(warnings, 1, `${file}: warning rows`);
          break;
        case "bidi-control-en":
          assert.ok(
            (doc.querySelector("h1")?.textContent ?? "").includes("\u202e"),
            `${file}: heading lost the control character`,
          );
          assert.equal(warnings, 0, `${file}: warning rows`);
          assert.ok(
            doc.querySelector(".preview-tail") !== null,
            `${file}: no tail preview`,
          );
          break;
        default:
          // The tail cases: a sendable selection and the not-sent description.
          assert.ok(
            doc.querySelector(".preview-tail") !== null,
            `${file}: no tail preview`,
          );
          assert.equal(warnings, 1, `${file}: warning rows`);
      }
    }
  });

  it("refuses to run without an output directory", () => {
    const result = spawnSync(process.execPath, ["scripts/popup-preview.ts"], {
      cwd: extensionDir,
      encoding: "utf8",
    });
    assert.equal(result.status, 2);
    assert.match(
      result.stderr,
      /usage: node scripts\/popup-preview\.ts <output-directory>/,
    );
  });
});
