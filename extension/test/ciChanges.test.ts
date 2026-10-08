import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import path from "node:path";
import { describe, it } from "node:test";

import { extensionDir } from "./helpers/paths.ts";

const script = path.join(extensionDir, "scripts", "has-extension-changes.sh");

function classify(files: string[]): string {
  const input = files.map((file) => `${file}\n`).join("");
  const result = spawnSync("bash", [script], { input, encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
  return result.stdout;
}

describe("has-extension-changes", () => {
  // One list per pattern, matching only that pattern. The two workflow lists
  // are separate so a pattern narrowed to ci.yml fails on release.yml.
  const triggering = [
    ["extension/src/core/acceptedUrl.ts"],
    ["extension/scripts/has-extension-changes.sh"],
    ["Makefile"],
    [".github/workflows/ci.yml"],
    [".github/workflows/release.yml"],
    ["go.mod"],
    ["README.md", "extension/package.json"],
  ];
  for (const files of triggering) {
    it(`runs the extension job for ${files.join(", ")}`, () => {
      assert.equal(classify(files), "true\n");
    });
  }

  const quiet = [
    ["internal/transcript/ytdlp.go"],
    ["docs/dev/security.md"],
    ["README.md"],
    ["notes/myextension.txt"],
    // Each pattern's text elsewhere in a path: these fail a pattern that
    // lost its ^ or $.
    ["notes/extension/x.txt"],
    ["tools/Makefile"],
    ["Makefile.local"],
    ["tools/go.mod"],
    ["go.modx"],
    ["tools/.github/workflows/ci.yml"],
    [],
  ];
  for (const files of quiet) {
    it(`skips the extension job for [${files.join(", ")}]`, () => {
      assert.equal(classify(files), "false\n");
    });
  }
});
