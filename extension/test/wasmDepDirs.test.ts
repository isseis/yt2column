import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  chmodSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, describe, it } from "node:test";

import { extensionDir, repoRoot } from "./helpers/paths.ts";

const script = path.join(extensionDir, "scripts", "wasm-dep-dirs.sh");

const tempDirs: string[] = [];
after(() => {
  for (const dir of tempDirs) {
    rmSync(dir, { recursive: true, force: true });
  }
});

interface Run {
  readonly status: number | null;
  readonly stdout: string;
  /** The arguments and the GOOS and GOARCH the fake go saw, one per line. */
  readonly seen: string;
}

/**
 * Runs the script with a fake go first on PATH that writes what it was given
 * and then prints output and exits with status.
 */
function runWithGo(output: string, status = 0): Run {
  const bin = mkdtempSync(path.join(tmpdir(), "fake-go-"));
  tempDirs.push(bin);
  const seen = path.join(bin, "seen");
  const printed = path.join(bin, "printed");
  writeFileSync(printed, output);
  const go = path.join(bin, "go");
  writeFileSync(
    go,
    [
      "#!/bin/sh",
      `printf 'GOOS=%s GOARCH=%s\\n' "$GOOS" "$GOARCH" > '${seen}'`,
      `printf '%s\\n' "$@" >> '${seen}'`,
      `cat '${printed}'`,
      `exit ${status}`,
      "",
    ].join("\n"),
  );
  chmodSync(go, 0o755);
  const result = spawnSync("bash", [script], {
    encoding: "utf8",
    env: { ...process.env, PATH: `${bin}:${process.env["PATH"] ?? ""}` },
  });
  return {
    status: result.status,
    stdout: result.stdout,
    seen: readFileSync(seen, "utf8"),
  };
}

// The real root as go list prints it: with symbolic links resolved.
const root = spawnSync("pwd", ["-P"], {
  cwd: repoRoot,
  encoding: "utf8",
}).stdout.trim();

describe("wasm-dep-dirs", () => {
  it("prints each main-module directory relative to the root, ending in /", () => {
    const run = runWithGo(
      `${root}/internal/llm\n\n${root}/internal/llm/devfake\n${root}/cmd/yt2column-wasm\n`,
    );
    assert.equal(run.status, 0);
    assert.equal(
      run.stdout,
      "internal/llm/\ninternal/llm/devfake/\ncmd/yt2column-wasm/\n",
    );
  });

  it("lists the js/wasm dependencies of both modules, keeping go list's failures", () => {
    const run = runWithGo(`${root}/internal/llm\n`);
    const [env, ...args] = run.seen.trimEnd().split("\n");
    assert.equal(env, "GOOS=js GOARCH=wasm");
    assert.deepEqual(args, [
      "list",
      "-deps",
      "-f",
      "{{if and .Module .Module.Main}}{{.Dir}}{{end}}",
      "./cmd/yt2column-wasm",
      "./cmd/yt2column-wasm-dev",
    ]);
    // -e would turn a broken package into a success.
    assert.equal(args.includes("-e"), false);
  });

  const failures: { name: string; output: string; status: number }[] = [
    { name: "go list fails", output: `${root}/internal/llm\n`, status: 1 },
    { name: "go list prints nothing", output: "", status: 0 },
    {
      name: "a directory is outside the root",
      output: "/elsewhere/pkg\n",
      status: 0,
    },
    {
      name: "a directory only starts like the root",
      output: `${root}x/pkg\n`,
      status: 0,
    },
  ];
  for (const { name, output, status } of failures) {
    it(`exits non-zero and prints nothing when ${name}`, () => {
      const run = runWithGo(output, status);
      assert.notEqual(run.status, 0);
      assert.equal(run.stdout, "");
    });
  }
});
