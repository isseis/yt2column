import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  mkdtempSync,
  readFileSync,
  rmSync,
  statSync,
  symlinkSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, it } from "node:test";

import { repoRoot } from "./helpers/paths.ts";

function run(command: string, args: string[], env?: NodeJS.ProcessEnv) {
  const result = spawnSync(command, args, {
    cwd: repoRoot,
    encoding: "utf8",
    env,
  });
  assert.equal(result.error, undefined, `${command}: ${String(result.error)}`);
  return result;
}

function readRepoFile(file: string): string {
  return readFileSync(path.join(repoRoot, file), "utf8");
}

const ciPath = ".github/workflows/ci.yml";

/**
 * The lines of one job under `jobs:` in ci.yml, from its `  <name>:` line up
 * to the next job. ci.yml indents job names by two spaces and steps by six;
 * that is all this relies on, it is not a YAML parser.
 */
function jobLines(name: string): string[] {
  const lines = readRepoFile(ciPath).split("\n");
  const start = lines.indexOf(`  ${name}:`);
  assert.notEqual(start, -1, `${ciPath} has no job ${name}`);
  const rest = lines.slice(start + 1);
  const end = rest.findIndex((line) => /^ {0,2}\S/.test(line));
  return [lines[start] ?? "", ...(end < 0 ? rest : rest.slice(0, end))];
}

/** The steps of a job, each as the text of its lines. */
function jobSteps(job: string[]): string[] {
  const steps: string[] = [];
  for (const line of job) {
    if (line.startsWith("      - ")) {
      steps.push(line);
    } else if (steps.length > 0 && line.startsWith("        ")) {
      steps[steps.length - 1] += `\n${line}`;
    }
  }
  return steps;
}

describe("build outputs are ignored", () => {
  const paths = [
    "extension/node_modules/pkg/index.js",
    "extension/dist/background.js",
    // A key must never be committed; *.pem keeps a stray one out of git add.
    "key.pem",
    "extension/key.pem",
  ];
  for (const file of paths) {
    it(`ignores ${file}`, () => {
      const result = run("git", ["check-ignore", "--quiet", file]);
      assert.equal(
        result.status,
        0,
        `${file} is not ignored: ${result.stderr}`,
      );
    });
  }
});

describe("no private key is tracked", () => {
  it("finds no PEM private key header in a tracked file", () => {
    // The class keeps this source from matching its own pattern.
    const header = /-----BEGIN [A-Z0-9 ]*PRIVATE KEY/;
    const files = run("git", ["ls-files", "-z"])
      .stdout.split("\0")
      .filter(Boolean);
    assert.ok(files.length > 0, "git ls-files listed nothing");
    const offenders = files.filter((file) => {
      const full = path.join(repoRoot, file);
      // A tracked path deleted from the working tree has nothing to scan.
      const stat = statSync(full, { throwIfNoEntry: false });
      return (
        stat?.isFile() === true && header.test(readFileSync(full, "latin1"))
      );
    });
    assert.deepEqual(offenders, []);
  });
});

describe("install uses npm ci", () => {
  it("ext-install runs npm ci without scripts, from the public registry only", () => {
    const result = run("make", ["-n", "ext-install"]);
    assert.equal(result.status, 0, result.stderr);
    const npmCi = result.stdout
      .split("\n")
      .filter((line) => /\bnpm ci\b/.test(line));
    assert.equal(npmCi.length, 1, result.stdout);
    for (const flag of [
      "--ignore-scripts",
      "--registry=https://registry.npmjs.org/",
      "--replace-registry-host=never",
    ]) {
      assert.ok(npmCi[0]?.split(" ").includes(flag), `${flag}: ${npmCi[0]}`);
    }
  });

  // npm install, update and the like may rewrite the lockfile; only ci (the
  // install), run (the scripts), pkg (reading packageManager) and --version
  // are allowed, and npx not at all.
  for (const file of ["Makefile", ciPath]) {
    it(`${file} runs npm only as ci, run, pkg or --version`, () => {
      // Comments are prose, not commands.
      const text = readRepoFile(file)
        .split("\n")
        .filter((line) => !/^\s*#/.test(line))
        .join("\n");
      assert.doesNotMatch(
        text,
        /\bnpm[ \t]+(?!(ci|run|pkg)\b|--version\b)[\w-]+/,
      );
      assert.doesNotMatch(text, /\bnpx\b/);
    });
  }

  it("tracks package-lock.json", () => {
    const result = run("git", [
      "ls-files",
      "--error-unmatch",
      "extension/package-lock.json",
    ]);
    assert.equal(result.status, 0, result.stderr);
  });
});

describe("ci runs every extension step", () => {
  it("check-changes declares has-extension-changes", () => {
    assert.ok(
      jobLines("check-changes").includes(
        "      has-extension-changes: ${{ steps.check.outputs.has-extension-changes }}",
      ),
    );
  });

  it("the extension job runs on has-extension-changes", () => {
    const job = jobLines("extension");
    assert.ok(job.includes("    needs: check-changes"));
    assert.ok(
      job.includes(
        "    if: needs.check-changes.outputs.has-extension-changes == 'true'",
      ),
    );
  });

  const targets = [
    "ext-install",
    "ext-typecheck",
    "ext-lint",
    "ext-fmt-check",
    "ext-test",
    "ext-build",
  ];
  it("runs each make target as a step of its own, in order", () => {
    const runs = jobSteps(jobLines("extension"))
      .map((step) => /^ +run: make (\S+)$/m.exec(step)?.[1])
      .filter((target) => target !== undefined);
    assert.deepEqual(runs, targets);
  });

  it("lists renamed and non-ASCII paths for has-extension-changes", () => {
    assert.ok(
      jobLines("check-changes").some((line) =>
        line.includes(
          "git -c core.quotePath=false diff --no-renames --name-only origin/main...HEAD",
        ),
      ),
    );
  });

  // A step that cannot fail the job does not count as running the check.
  for (const name of ["extension", "secret-scan"]) {
    it(`no step of ${name} is conditional or allowed to fail`, () => {
      for (const step of jobSteps(jobLines(name))) {
        assert.doesNotMatch(step, /^ {6}[- ] (if|continue-on-error):/m);
      }
    });
  }

  it("checks go list ./... after setting up Go", () => {
    const steps = jobSteps(jobLines("extension"));
    const setupGo = steps.findIndex((step) =>
      step.includes("uses: actions/setup-go@"),
    );
    const goList = steps.findIndex(
      (step) =>
        step.includes("go list ./...") &&
        step.includes("'^github.com/isseis/yt2column/extension/'"),
    );
    assert.notEqual(setupGo, -1);
    assert.ok(goList > setupGo, "no go list check after setup-go");
  });
});

describe("secret scan always runs", () => {
  it("secret-scan has no condition and greps tracked files for PEM private keys", () => {
    const job = jobLines("secret-scan");
    assert.equal(
      job.some((line) => /^ {4}(if|needs):/.test(line)),
      false,
      "secret-scan must not depend on another job",
    );
    assert.ok(
      job.some((line) =>
        line.includes("git grep -n -E -e '-----BEGIN [A-Z0-9 ]*PRIVATE KEY'"),
      ),
    );
    // A match (git grep status 0) and an error (above 1) both fail the job.
    assert.ok(job.some((line) => /^ +0\) .*; exit 1 ;;$/.test(line)));
    assert.ok(job.some((line) => /^ +\*\) .*; exit 1 ;;$/.test(line)));
  });
});

describe("go targets do not need node", () => {
  it("make -n build test lint deadcode succeeds without node or npm on PATH", () => {
    const bin = mkdtempSync(path.join(tmpdir(), "no-node-bin-"));
    try {
      for (const tool of ["make", "sh"]) {
        const found = run("sh", ["-c", `command -v ${tool}`]).stdout.trim();
        assert.notEqual(found, "", `${tool} not found`);
        symlinkSync(found, path.join(bin, tool));
      }
      const env = { PATH: bin, HOME: process.env.HOME ?? "" };
      // The premise: neither node nor npm can be found on this PATH.
      for (const tool of ["node", "npm"]) {
        assert.notEqual(
          run(path.join(bin, "sh"), ["-c", `command -v ${tool}`], env).status,
          0,
        );
      }
      const result = run(
        path.join(bin, "make"),
        ["-n", "build", "test", "lint", "deadcode"],
        env,
      );
      assert.equal(result.status, 0, result.stderr);
      assert.equal(result.stderr, "");
      // Nor would the recipes call them when run.
      assert.doesNotMatch(result.stdout, /\b(node|npm|npx)\b/);
    } finally {
      rmSync(bin, { recursive: true, force: true });
    }
  });
});
