import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, describe, it } from "node:test";

import { JSDOM } from "jsdom";

import { previewCases } from "../scripts/popup-preview.ts";
import { extensionDir } from "./helpers/paths.ts";

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

    for (const previewCase of previewCases) {
      const file = `${previewCase.name}.html`;
      const full = path.join(output, file);
      assert.ok(existsSync(full), file);
      const document = new JSDOM(readFileSync(full, "utf8")).window.document;
      assert.ok(
        document.getElementById("primary") !== null,
        `${file}: no button`,
      );
      assert.equal(
        document.querySelectorAll("script").length,
        0,
        `${file}: holds a script`,
      );
      assert.equal(
        document.documentElement.lang,
        previewCase.locale,
        `${file}: wrong page language`,
      );
    }
  });
});
