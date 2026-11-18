import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, describe, it } from "node:test";

import { extensionDir } from "./helpers/paths.ts";

const script = path.join(extensionDir, "scripts", "has-extension-changes.sh");

const tempDirs: string[] = [];
after(() => {
  for (const dir of tempDirs) {
    rmSync(dir, { recursive: true, force: true });
  }
});

/** Writes the dependency directory list into a new file and returns its path. */
function depDirFile(content: string): string {
  const dir = mkdtempSync(path.join(tmpdir(), "dep-dirs-"));
  tempDirs.push(dir);
  const file = path.join(dir, "dirs");
  writeFileSync(file, content);
  return file;
}

/** The dependency directories the tests classify against, as wasm-dep-dirs.sh prints them. */
const depDirs =
  "internal/llm/\ninternal/llm/devfake/\nprompts/\ncmd/yt2column-wasm/\n";

/** Runs the script on files (NUL-separated) and returns its stdout. */
function classify(files: string[], dirs = depDirs): string {
  const input = files.map((file) => `${file}\0`).join("");
  const result = spawnSync("bash", [script, depDirFile(dirs)], {
    input,
    encoding: "utf8",
  });
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
    ["go.sum"],
    ["README.md"],
    ["README.en.md"],
    ["docs/dev/guides/development.md"],
    // Files under a dependency directory: a Go file, an embedded template,
    // an embedded fake article (a .md file, which the Go jobs'
    // classification skips), and a file in a subdirectory whose only listed
    // ancestor is internal/llm/. None exists on disk, as for a deleted
    // file: the path alone decides.
    ["internal/llm/llm.go"],
    ["prompts/draft.tmpl"],
    ["internal/llm/devfake/articles/standard.md"],
    ["internal/llm/deepseek/client.go"],
    ["cmd/yt2column-wasm/main.go"],
  ];
  for (const files of triggering) {
    it(`runs the extension job for ${files.join(", ")}`, () => {
      assert.equal(classify(files), "true\n");
    });
  }

  // A mixed list must read past the non-triggering first record to the
  // triggering one; a loop that stops at the first non-match misses it.
  it("reads a mixed list of non-triggering and triggering paths", () => {
    assert.equal(
      classify(["internal/transcript/ytdlp.go", "extension/package.json"]),
      "true\n",
    );
  });

  // NUL-delimited input keeps a control character in the path, which git would
  // otherwise quote, from hiding an extension path from the ^ anchor.
  for (const file of ["extension/a\tb.ts", "extension/a\nb.ts"]) {
    it(`runs the extension job for ${JSON.stringify(file)}`, () => {
      assert.equal(classify([file]), "true\n");
    });
  }

  // A newline before "extension/" must not split one non-extension path into a
  // fragment that matches: NUL delimiting keeps each path whole.
  it("skips a non-extension path containing a newline", () => {
    assert.equal(classify(["notes/x\nextension/y.ts"]), "false\n");
  });

  const quiet = [
    ["internal/transcript/ytdlp.go"],
    ["notes/myextension.txt"],
    // Each pattern's text elsewhere in a path: these fail a pattern that
    // lost its ^ or $.
    ["notes/extension/x.txt"],
    ["tools/Makefile"],
    ["Makefile.local"],
    ["tools/go.mod"],
    ["go.modx"],
    ["tools/.github/workflows/ci.yml"],
    // The document paths match whole paths only.
    ["docs/README.md"],
    ["README.md.bak"],
    ["docs/README.en.md"],
    ["README.en.md.bak"],
    ["x/docs/dev/guides/development.md"],
    ["docs/dev/guides/development.md.orig"],
    // Documents under docs/dev that no extension test reads.
    ["docs/dev/security.md"],
    ["docs/dev/project_overview.md"],
    ["docs/dev/guides/cli.md"],
    // A directory name that only starts like a dependency directory, and the
    // directory's name without its separator.
    ["internal/llmx/a.go"],
    ["internal/llm"],
    ["cmd/yt2column-wasm-other/main.go"],
    ["x/internal/llm/llm.go"],
    ["go.sum.orig"],
    [],
  ];
  for (const files of quiet) {
    it(`skips the extension job for [${files.join(", ")}]`, () => {
      assert.equal(classify(files), "false\n");
    });
  }

  it("reads the last directory without a trailing newline", () => {
    assert.equal(
      classify(["prompts/x.tmpl"], "internal/llm/\nprompts/"),
      "true\n",
    );
  });

  it("skips a path under no directory when the list is empty", () => {
    assert.equal(classify(["internal/llm/llm.go"], ""), "false\n");
  });

  const malformed: { name: string; args: string[] }[] = [
    { name: "without the directory file", args: [script] },
    {
      name: "with a directory that does not end in /",
      args: [script, depDirFile("internal/llm\n")],
    },
  ];
  for (const { name, args } of malformed) {
    it(`fails ${name}`, () => {
      const result = spawnSync("bash", args, {
        input: "internal/llm/llm.go\0",
        encoding: "utf8",
      });
      assert.notEqual(result.status, 0);
      assert.equal(result.stdout, "");
    });
  }
});
