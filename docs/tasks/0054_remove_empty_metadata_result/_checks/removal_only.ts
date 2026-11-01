// Checks that the only change to extension/ between two commits is the removal
// of emptyMetadataResult and its test. Run from anywhere inside the repository,
// after `git fetch origin main`:
//
//   node docs/tasks/0054_remove_empty_metadata_result/_checks/removal_only.ts \
//     "$(git merge-base origin/main HEAD)" HEAD
//
// Both commits are read from git (never from the working tree), and three
// checks are reported, each on its own PASS or FAIL line:
//
//   1. changed files: the files under extension/ that differ between the two
//      commits are exactly extension/src/core/metadata.ts and
//      extension/test/metadata.test.ts.
//   2. removal only: each of those two files at the base commit, with the
//      removed lines taken out, equals the file at the head commit byte for
//      byte, with the same file mode. The removed lines are found by their
//      content, not by their line numbers: in metadata.ts the blank line, the
//      doc comment and the definition of emptyMetadataResult; in
//      metadata.test.ts the import line and the blank line followed by
//      describe("emptyMetadataResult").
//   3. name absent: no file tracked under extension/src or extension/test at
//      the head commit contains the name emptyMetadataResult.
//
// Exits 0 when all three checks pass, 1 when any fails (printing what
// differs), and 2 when the check cannot run: a missing argument, a value that
// is not a commit, no repository, or a removed run of lines found at the base
// commit other than exactly once.
//
// make test and CI do not run it. It reads no environment variable.

import { execFileSync, spawnSync } from "node:child_process";

const name = "emptyMetadataResult";
const metadataPath = "extension/src/core/metadata.ts";
const testPath = "extension/test/metadata.test.ts";

/** The runs of lines removed from each file, matched by exact content. */
const removals: Record<string, string[][]> = {
  [metadataPath]: [
    [
      "",
      "/** The empty result used when the collection was rejected: no read is attempted. */",
      "export function emptyMetadataResult(): MetadataResult {",
      "  return { values: {}, showExpandPrompt: false, omissions: [] };",
      "}",
    ],
  ],
  [testPath]: [
    ["  emptyMetadataResult,"],
    [
      "",
      'describe("emptyMetadataResult", () => {',
      '  it("has no values, no prompt, no omissions, and no check", () => {',
      "    assert.deepEqual(emptyMetadataResult(), {",
      "      values: {},",
      "      showExpandPrompt: false,",
      "      omissions: [],",
      "    });",
      "  });",
      "});",
    ],
  ],
};

function fail(message: string): never {
  process.stderr.write(`removal_only: ${message}\n`);
  process.exit(2);
}

function git(...args: string[]): string {
  return execFileSync("git", args, { encoding: "utf8" });
}

/**
 * The bytes of path at rev, one character per byte (latin1 decodes every byte
 * losslessly, so string equality is byte equality), or undefined when rev has
 * no such file.
 */
function show(rev: string, path: string): string | undefined {
  const result = spawnSync("git", ["cat-file", "blob", `${rev}:${path}`], {
    encoding: "latin1",
  });
  return result.status === 0 ? result.stdout : undefined;
}

/** The file mode of path at rev (such as 100644), or "" when rev has no such file. */
function mode(rev: string, path: string): string {
  return git("ls-tree", rev, "--", path).split(" ")[0] ?? "";
}

/** The start indexes of every occurrence of run in lines. */
function occurrences(lines: string[], run: string[]): number[] {
  const found: number[] = [];
  for (let i = 0; i + run.length <= lines.length; i++) {
    if (run.every((line, j) => lines[i + j] === line)) {
      found.push(i);
    }
  }
  return found;
}

/** text with each run removed; stops the check unless each run occurs exactly once. */
function removeRuns(path: string, text: string, runs: string[][]): string {
  let lines = text.split("\n");
  for (const run of runs) {
    const found = occurrences(lines, run);
    const start = found[0];
    if (found.length !== 1 || start === undefined) {
      fail(
        `${path} at the base commit holds the run starting "${run.find((l) => l !== "")}" ${found.length} times, not once`,
      );
    }
    lines = [...lines.slice(0, start), ...lines.slice(start + run.length)];
  }
  return lines.join("\n");
}

/** The first differing line of two texts, for the FAIL message. */
function firstDifference(expected: string, actual: string): string {
  const a = expected.split("\n");
  const b = actual.split("\n");
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    if (a[i] !== b[i]) {
      return `line ${i + 1}\n    expected: ${JSON.stringify(a[i] ?? null)}\n    head:     ${JSON.stringify(b[i] ?? null)}`;
    }
  }
  return "(no line differs)";
}

function check(baseArg: string, headArg: string): void {
  process.chdir(git("rev-parse", "--show-toplevel").trim());
  const base = git(
    "rev-parse",
    "--verify",
    "--end-of-options",
    `${baseArg}^{commit}`,
  ).trim();
  const head = git(
    "rev-parse",
    "--verify",
    "--end-of-options",
    `${headArg}^{commit}`,
  ).trim();
  process.stdout.write(`base ${base}\nhead ${head}\n`);

  // Removal runs are located first, so an unusable base stops with exit 2
  // before any check is reported.
  const expected: Record<string, string> = {};
  for (const [path, runs] of Object.entries(removals)) {
    const text = show(base, path);
    if (text === undefined) {
      fail(`${path} does not exist at the base commit`);
    }
    expected[path] = removeRuns(path, text, runs);
  }

  let failed = false;
  const report = (ok: boolean, label: string, detail: string): void => {
    if (!ok) {
      failed = true;
    }
    process.stdout.write(`${ok ? "PASS" : "FAIL"}: ${label}${detail}\n`);
  };

  // 1. changed files
  // diff-tree is plumbing: user configuration such as diff.renames does not
  // change its output.
  const changed = git(
    "diff-tree",
    "-r",
    "--no-renames",
    "--name-only",
    base,
    head,
    "--",
    "extension",
  )
    .split("\n")
    .filter((line) => line !== "")
    .sort();
  const wanted = Object.keys(removals).sort();
  report(
    changed.length === wanted.length &&
      changed.every((p, i) => p === wanted[i]),
    "changed files under extension/ are exactly the two files",
    `\n  changed: ${changed.length === 0 ? "(none)" : changed.join(", ")}`,
  );

  // 2. removal only
  const differing: string[] = [];
  for (const path of wanted) {
    const want = expected[path];
    if (want === undefined) {
      throw new Error(`no expected content for ${path}`);
    }
    const actual = show(head, path);
    if (actual === undefined) {
      differing.push(`${path}: missing at the head commit`);
    } else if (actual !== want) {
      differing.push(`${path}: ${firstDifference(want, actual)}`);
    }
    if (actual !== undefined && mode(base, path) !== mode(head, path)) {
      differing.push(
        `${path}: mode ${mode(base, path)} at base, ${mode(head, path)} at head`,
      );
    }
  }
  report(
    differing.length === 0,
    "each file at head equals the base file with only the removed lines taken out",
    differing.map((d) => `\n  ${d}`).join(""),
  );

  // 3. name absent; git grep exits 1 when nothing matches.
  const grep = spawnSync(
    "git",
    [
      "grep",
      "-n",
      "-F",
      "-e",
      name,
      head,
      "--",
      "extension/src",
      "extension/test",
    ],
    { encoding: "utf8" },
  );
  if (grep.status !== 0 && grep.status !== 1) {
    fail(`git grep failed:\n${grep.stderr}`);
  }
  report(
    grep.status === 1,
    `no tracked file under extension/src or extension/test names ${name} at head`,
    grep.stdout
      .split("\n")
      .filter((line) => line !== "")
      .map((line) => `\n  ${line}`)
      .join(""),
  );

  process.exit(failed ? 1 : 0);
}

const [baseArg, headArg, ...rest] = process.argv.slice(2);
if (baseArg === undefined || headArg === undefined || rest.length > 0) {
  fail("usage: removal_only.ts <base-commit> <head-commit>");
}
try {
  check(baseArg, headArg);
} catch (error) {
  fail(error instanceof Error ? error.message : String(error));
}
