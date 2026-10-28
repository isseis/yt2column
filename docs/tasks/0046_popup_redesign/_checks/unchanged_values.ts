// Checks that the values this task leaves alone are the same at the merge-base
// with origin/main and at HEAD. Run from the repository root, after
// `git fetch origin main`:
//
//   node docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts
//
// The merge-base's and HEAD's extension/src are taken from their commits (never
// from the working tree) into temporary directories, and the same driver prints
// three things from each:
//
//   1. The catalog items this task does not change -- rejection, preflight,
//      serverReject, notSent, accepted, noResponse, hostFailed, unexpected and
//      sending -- in both locales (AC-11, AC-20, AC-23).
//   2. The body buildArticleBody makes for a fixed set of inputs, with and
//      without metadata and with empty values, and the value of maxBodyBytes
//      (AC-19). The inputs hold no page text.
//   3. The bytes of extension/static/manifest.json (AC-35).
//
// Exits 0 when all three are identical, 1 when any differs (printing the
// difference), and 2 when the check cannot run.

import { execFileSync, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const driver = `
import { catalogs } from "./src/core/i18n/catalog.ts";
import { buildArticleBody, maxBodyBytes } from "./src/core/prepare.ts";

// The catalog items this task does not change; read by name so the items it
// adds or removes cannot enter the comparison.
const unchanged = [
  "rejection",
  "preflight",
  "serverReject",
  "notSent",
  "accepted",
  "noResponse",
  "hostFailed",
  "unexpected",
  "sending",
];
const catalog = {};
for (const locale of ["en", "ja"]) {
  catalog[locale] = {};
  for (const key of unchanged) {
    catalog[locale][key] = catalogs[locale][key];
  }
}

const watchUrl = "https://www.youtube.com/watch?v=abc";
const inputs = [
  { name: "no metadata", metadata: {} },
  {
    name: "every metadata value",
    metadata: {
      title: "A video title",
      channelName: "A channel name",
      description: "A description",
    },
  },
  {
    name: "empty metadata values",
    metadata: { title: "", channelName: "", description: "" },
  },
  { name: "description only", metadata: { description: "A description" } },
];
const bodies = {};
for (const { name, metadata } of inputs) {
  bodies[name] = buildArticleBody(
    { selection: "00:00 a caption\\n00:05 another caption", url: watchUrl, title: "A tab title" },
    metadata,
  );
}

process.stdout.write(
  JSON.stringify({ catalog, bodies, maxBodyBytes }, null, 2) + "\\n",
);
`;

function git(...args: string[]): string {
  return execFileSync("git", args, { encoding: "utf8" }).trim();
}

let workDir = "";

function fail(message: string): never {
  if (workDir !== "") {
    rmSync(workDir, { recursive: true, force: true });
  }
  process.stderr.write(`unchanged_values: ${message}\n`);
  process.exit(2);
}

/** Extracts rev's extension sources into a temporary directory and runs the driver there. */
function dumpValues(rev: string): string {
  const dir = mkdtempSync(path.join(tmpdir(), "unchanged-values-"));
  workDir = dir;
  try {
    const archive = execFileSync("git", [
      "archive",
      rev,
      "extension/src",
      "extension/package.json",
    ]);
    execFileSync("tar", ["-x", "-C", dir], { input: archive });
    const extension = path.join(dir, "extension");
    const script = path.join(extension, "unchanged-values-driver.ts");
    writeFileSync(script, driver);
    const result = spawnSync(process.execPath, [script], {
      cwd: extension,
      encoding: "utf8",
    });
    if (result.status !== 0) {
      fail(`the driver failed at ${rev}:\n${result.stderr}`);
    }
    return result.stdout;
  } finally {
    rmSync(dir, { recursive: true, force: true });
    workDir = "";
  }
}

/** The sha256 of the manifest committed at rev. */
function manifestSha(rev: string): string {
  const bytes = execFileSync("git", [
    "show",
    `${rev}:extension/static/manifest.json`,
  ]);
  return createHash("sha256").update(bytes).digest("hex");
}

function check(): void {
  const root = git("rev-parse", "--show-toplevel");
  process.chdir(root);
  const head = git("rev-parse", "HEAD");
  const base = git("merge-base", "origin/main", "HEAD");

  const before = dumpValues(base).split("\n");
  const after = dumpValues(head).split("\n");
  process.stdout.write(`merge-base ${base}\nhead       ${head}\n`);

  const differing: string[] = [];
  for (let i = 0; i < Math.max(before.length, after.length); i++) {
    if (before[i] !== after[i]) {
      differing.push(
        `line ${i + 1}\n  merge-base: ${before[i] ?? "(none)"}\n  head:       ${after[i] ?? "(none)"}`,
      );
    }
  }

  const baseManifest = manifestSha(base);
  const headManifest = manifestSha(head);

  let failed = false;
  if (differing.length > 0) {
    failed = true;
    process.stdout.write(
      `FAIL: the unchanged catalog items or the built bodies differ\n${differing.join("\n")}\n`,
    );
  } else {
    process.stdout.write(
      "PASS: the unchanged catalog items and the built bodies are identical\n",
    );
  }
  if (baseManifest !== headManifest) {
    failed = true;
    process.stdout.write(
      `FAIL: extension/static/manifest.json differs\n  merge-base: ${baseManifest}\n  head:       ${headManifest}\n`,
    );
  } else {
    process.stdout.write(
      `PASS: extension/static/manifest.json is identical (sha256 ${headManifest})\n`,
    );
  }
  process.exit(failed ? 1 : 0);
}

try {
  check();
} catch (error) {
  fail(error instanceof Error ? error.message : String(error));
}
