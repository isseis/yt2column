import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, describe, it } from "node:test";

import { JSDOM } from "jsdom";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { metadataDisplayLimit } from "../src/core/popupView.ts";
import { truncate } from "../src/core/summary.ts";
import { publishControl } from "../src/core/jobView.ts";
import type {
  JobDisplay,
  PublishRow,
  ViewerState,
} from "../src/core/jobView.ts";
import { renderViewer } from "../src/ui/renderViewer.ts";
import { previewCases, viewerPreviewCases } from "../scripts/popup-preview.ts";
import {
  popupDocument,
  viewerDocument,
  viewerElements,
} from "./helpers/dom.ts";
import { extensionDir } from "./helpers/paths.ts";

/** What one input set's page must show beyond the common structure. */
interface Expectation {
  /** The warning rows in a collected view; absent for a rejected view. */
  readonly warnings?: number;
  /** The first text of the head preview. */
  readonly headPrefix?: string;
  /** The last text of the tail preview; null when the page has no tail. */
  readonly tail: string | null;
}

/**
 * The required input sets, keyed by file name. The keys are the plan's list:
 * a case missing from the script fails the lookup, and a new case without an
 * expectation fails before its page is read.
 */
const expectations: Readonly<Record<string, Expectation>> = {
  "worst-case-en": { warnings: 2, headPrefix: "a", tail: "a" },
  "worst-case-ja": { warnings: 2, headPrefix: "a", tail: "a" },
  "tail-ltr-en": { warnings: 1, headPrefix: "00:00 ", tail: "END" },
  "tail-rtl-en": { warnings: 1, headPrefix: "00:00 ", tail: "\u05dd" },
  "tail-mixed-en": { warnings: 1, headPrefix: "00:00 ", tail: "\u05e9" },
  "tail-mark-en": { warnings: 1, headPrefix: "00:00 ", tail: "\u200f" },
  "rtl-all-en": { warnings: 1, headPrefix: "\u05e9", tail: "\u05dd" },
  "bidi-control-en": {
    warnings: 0,
    headPrefix: "\u202e",
    tail: "\u202e",
  },
  "read-failure-en": { warnings: 1, headPrefix: "00:00 ", tail: null },
  "rejected-en": { tail: null },
  "rejected-ja": { tail: null },
};

/** The page's element with id, asserting it exists. */
function byId(doc: Document, id: string): HTMLElement {
  const element = doc.getElementById(id);
  assert.ok(element !== null, `#${id}`);
  return element;
}

/** The value the renderer shows for a metadata value: cut, with an ellipsis. */
function shown(value: string): string {
  const cut = truncate(value, metadataDisplayLimit);
  return cut.truncated ? `${cut.value}\u2026` : cut.value;
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

/** Asserts the page shows what a rejected view shows. */
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

  it("writes one styled static page per required input set", () => {
    const output = mkdtempSync(path.join(tmpdir(), "popup-preview-"));
    dirs.push(output);
    const result = spawnSync(
      process.execPath,
      ["scripts/popup-preview.ts", output],
      { cwd: extensionDir, encoding: "utf8" },
    );
    assert.equal(result.status, 0, result.stderr);
    // The pages load the stylesheets the shipped popup.html loads, in its
    // order, and each copy beside them is the shipped file.
    const stylesheets = [
      ...popupDocument().querySelectorAll("link[rel=stylesheet]"),
    ].map((link) => link.getAttribute("href") ?? "");
    assert.deepEqual(stylesheets, ["theme.css", "style.css"]);
    for (const stylesheet of stylesheets) {
      assert.equal(
        readFileSync(path.join(output, stylesheet)).toString("utf8"),
        readFileSync(path.join(extensionDir, "static", stylesheet), "utf8"),
        `${stylesheet} is not the shipped stylesheet`,
      );
    }

    for (const name of Object.keys(expectations)) {
      assert.ok(
        previewCases.some((previewCase) => previewCase.name === name),
        `missing case ${name}`,
      );
    }

    for (const previewCase of previewCases) {
      const expected = expectations[previewCase.name];
      assert.ok(
        expected !== undefined,
        `no expectation for ${previewCase.name}`,
      );
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
      // The preview loads only the stylesheets copied beside it.
      assert.deepEqual(
        [...doc.querySelectorAll("link")].map((link) =>
          link.getAttribute("href"),
        ),
        stylesheets,
        `${file}: stylesheets`,
      );
      assert.equal(
        doc.querySelectorAll("img, iframe, object, embed").length,
        0,
        `${file}: embeds a resource`,
      );

      const head = doc.querySelector(".preview-head");
      if (expected.headPrefix !== undefined) {
        assert.ok(
          (head?.textContent ?? "").startsWith(expected.headPrefix),
          `${file}: head preview does not start with ${JSON.stringify(expected.headPrefix)}`,
        );
      }
      const tail = doc.querySelector(".preview-tail-value");
      if (expected.tail === null) {
        assert.equal(
          doc.querySelector(".preview-tail"),
          null,
          `${file}: unexpected tail preview`,
        );
      } else {
        assert.ok(tail !== null, `${file}: no tail preview`);
        assert.ok(
          (tail.textContent ?? "").endsWith(expected.tail),
          `${file}: tail preview does not end with ${JSON.stringify(expected.tail)}`,
        );
      }

      if (previewCase.kind === "rejected") {
        expectRejectedSections(doc, file);
        const message =
          catalogs[previewCase.locale].rejection[previewCase.reason];
        assert.equal(
          byId(doc, "rejection-text").textContent,
          message.text,
          `${file}: rejection reason`,
        );
        assert.equal(
          doc.querySelectorAll("#rejection li").length,
          message.steps.length,
          `${file}: rejection steps`,
        );
        continue;
      }

      expectCollectedSections(doc, file);
      assert.equal(
        doc.querySelectorAll(".check-row").length,
        2,
        `${file}: check rows`,
      );
      assert.equal(
        doc.querySelectorAll(".check-row-warning").length,
        expected.warnings,
        `${file}: warning rows`,
      );

      // The heading is the video title when it is sent, the tab title
      // otherwise; the channel name is shown exactly when it is sent.
      const title = previewCase.metadata.values.title;
      assert.equal(
        doc.querySelector("h1")?.textContent,
        title === undefined ? previewCase.tabTitle : shown(title),
        `${file}: heading`,
      );
      const channelName = previewCase.metadata.values.channelName;
      const channel = doc.querySelector(".channel-name");
      if (channelName === undefined) {
        assert.equal(channel, null, `${file}: channel name shown`);
      } else {
        assert.equal(
          channel?.textContent,
          shown(channelName),
          `${file}: channel name`,
        );
      }
    }
  });

  it("writes one page per viewer state, job kind and publish row, styled as the viewer is", () => {
    const output = mkdtempSync(path.join(tmpdir(), "popup-preview-"));
    dirs.push(output);
    const result = spawnSync(
      process.execPath,
      ["scripts/popup-preview.ts", output],
      { cwd: extensionDir, encoding: "utf8" },
    );
    assert.equal(result.status, 0, result.stderr);

    // Every kind is covered: the keys are total over each union, so a kind
    // added later fails to compile here until it has a page.
    const states: Record<ViewerState["kind"], number> = {
      "invalid-link": 0,
      sending: 0,
      "start-failed": 0,
      loading: 0,
      job: 0,
    };
    const jobs: Record<JobDisplay["kind"], number> = {
      running: 0,
      done: 0,
      failed: 0,
      lost: 0,
    };
    const rows: Record<PublishRow["row"], number> = {
      lost: 0,
      posted: 0,
      posting: 0,
      unpostable: 0,
      "webhook-not-configured": 0,
      failed: 0,
      "not-posted": 0,
    };
    const stylesheets = [
      ...viewerDocument().querySelectorAll("link[rel=stylesheet]"),
    ].map((link) => link.getAttribute("href") ?? "");
    assert.deepEqual(stylesheets, ["theme.css", "viewer.css"]);
    for (const stylesheet of stylesheets) {
      assert.equal(
        readFileSync(path.join(output, stylesheet)).toString("utf8"),
        readFileSync(path.join(extensionDir, "static", stylesheet), "utf8"),
        `${stylesheet} is not the shipped stylesheet`,
      );
    }

    for (const previewCase of viewerPreviewCases) {
      const { state } = previewCase.view;
      states[state.kind] += 1;
      if (state.kind === "job") {
        jobs[state.job.kind] += 1;
        if (state.job.kind === "done" || state.job.kind === "lost") {
          rows[publishControl(state.job, false).row.row] += 1;
        }
      }
      const file = `${previewCase.name}.html`;
      const full = path.join(output, file);
      assert.ok(existsSync(full), file);
      const doc = new JSDOM(readFileSync(full, "utf8")).window.document;
      assert.deepEqual(
        [...doc.querySelectorAll("link")].map((link) =>
          link.getAttribute("href"),
        ),
        stylesheets,
        `${file}: stylesheets`,
      );
      assert.equal(
        doc.querySelectorAll("script").length,
        0,
        `${file}: holds a script`,
      );
      assert.equal(doc.documentElement.lang, previewCase.locale, file);
      // The page shows what the viewer draws for the same view: the same
      // text in the same elements.
      const expected = viewerElements();
      renderViewer(expected, previewCase.view, previewCase.locale);
      const shape = (root: Element | null): string[] =>
        root === null
          ? []
          : [root, ...root.querySelectorAll("*")].map(
              (element) => `${element.tagName}.${element.className}`,
            );
      const shown = doc.getElementById("state");
      assert.equal(
        shown?.textContent,
        expected.state.textContent,
        `${file}: state text`,
      );
      assert.deepEqual(shape(shown), shape(expected.state), `${file}: state`);
    }
    for (const [name, counts] of Object.entries({ states, jobs, rows })) {
      for (const [kind, count] of Object.entries(counts)) {
        assert.ok(count > 0, `no page for the ${kind} ${name}`);
      }
    }
  });

  it("refuses to run without an output directory", () => {
    const result = spawnSync(process.execPath, ["scripts/popup-preview.ts"], {
      cwd: extensionDir,
      encoding: "utf8",
    });
    // The usage line is the script's CLI contract: the output directory is
    // its one argument, and the line tells the developer what to pass.
    assert.equal(result.status, 2);
    assert.match(result.stderr, /^usage: .*<output-directory>/mu);
  });
});
