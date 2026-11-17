// Builds the extension into its output directory and checks the result:
//
//   node scripts/build.ts release
//   node scripts/build.ts dev --article <a> --failure <f> --fail-at <s> \
//       --delay <seconds> --settings-check <c>
//
// The release build writes dist/ with the release article generation module
// (cmd/yt2column-wasm). The development build writes dist-dev/ with the
// development module (cmd/yt2column-wasm-dev) and the scenario given by the
// five arguments (make ext-build-dev passes all five); its manifest has no key,
// so it gets an extension ID of its own, and its build kind module declares
// "dev".
//
// Contract: every Go command runs with GOTOOLCHAIN set to go.mod's version,
// so the module and wasm_exec.js come from the toolchain go.mod names, never
// from whatever go is on PATH; a go line without a patch version names no
// toolchain and stops the build, and so does a go that reports another
// version or any GOFLAGS (from the environment or go env -w). The development build parses its scenario
// with a native build of its main package before it touches dist-dev/, so a
// scenario the module would reject leaves the previous output as it was. The
// scenario reaches the linker as one -ldflags string of five -X flags, given
// once, so no second -ldflags can override it. The build ends by checking its
// output (scripts/check-dist.ts) and fails on any violation.
import { spawnSync } from "node:child_process";
import { cpSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parseArgs } from "node:util";

import {
  buildKindFileName,
  buildKindSource,
  devManifest,
  outputDir,
  parseBuildKind,
  wasmExecFileName,
  wasmFileName,
  wasmMainPackage,
} from "./build-variant.ts";
import { checkDist } from "./check-dist.ts";
import type {
  BuildTarget,
  DistExpectation,
  ScenarioValues,
} from "./check-dist.ts";

/** What a finished command left behind. */
export interface CommandResult {
  /** The exit status; null when the command could not start or was killed. */
  readonly status: number | null;
  readonly stdout: string;
  readonly stderr: string;
}

/** Runs one command to completion. Injected, so the tests run no go or tsc. */
export type Runner = (
  command: string,
  args: readonly string[],
  options: { readonly cwd: string; readonly env: NodeJS.ProcessEnv },
) => CommandResult;

/** Everything a build needs from outside: directories, commands, and the check. */
export interface BuildOptions {
  /** The extension directory: src/, static/, and the output directories. */
  readonly extensionDir: string;
  /** The Go module root, which holds go.mod and the main packages. */
  readonly repoRoot: string;
  readonly target: BuildTarget;
  /** The environment the commands start from. */
  readonly env: NodeJS.ProcessEnv;
  readonly run: Runner;
  readonly check: (
    root: string,
    expectation: DistExpectation,
  ) => Promise<string[]>;
  readonly readBuildInfo: DistExpectation["readBuildInfo"];
}

/** A build that stopped, with the reason to print. */
export class BuildError extends Error {
  override name = "BuildError";
}

/**
 * The toolchain go.mod's go line names, such as "go1.26.5". The line must
 * carry a patch version: "go 1.26" is a language version, not a toolchain, so
 * it is rejected rather than completed with a guessed patch.
 */
export function goToolchain(goMod: string): string {
  const lines = goMod.split("\n").filter((line) => /^go\s/.test(line));
  if (lines.length !== 1) {
    throw new BuildError(`go.mod has ${lines.length} go lines, want 1`);
  }
  const match = /^go (\d+\.\d+\.\d+)\s*$/.exec(lines[0] ?? "");
  if (match === null) {
    throw new BuildError(
      `go.mod's "${lines[0]?.trim()}" has no patch version, so it names no Go toolchain; write the go line as go <major>.<minor>.<patch>`,
    );
  }
  return `go${match[1]}`;
}

/** The module path of go.mod's module line. */
export function goModulePath(goMod: string): string {
  const paths = goMod
    .split("\n")
    .map((line) => /^module\s+(\S+)\s*$/.exec(line)?.[1])
    .filter((value) => value !== undefined);
  if (paths.length !== 1) {
    throw new BuildError(`go.mod has ${paths.length} module lines, want 1`);
  }
  return paths[0] ?? "";
}

// The linker variables of cmd/yt2column-wasm-dev, one per scenario value.
const scenarioVariables: readonly [keyof ScenarioValues, string][] = [
  ["article", "main.scenarioArticle"],
  ["failure", "main.scenarioFailure"],
  ["failAt", "main.scenarioFailAt"],
  ["delaySeconds", "main.scenarioDelaySeconds"],
  ["settingsCheck", "main.scenarioSettingsCheck"],
];

/**
 * The one -ldflags value that embeds the scenario: five -X flags. The go
 * command splits the value at spaces and reads quotes, so a value with a
 * character other than a letter, a digit, "_" or "-" would change the flags
 * the linker gets; such a value is rejected. Every accepted spelling is made
 * of those characters, and an empty value passes through for the module to
 * reject.
 */
export function scenarioLdflags(scenario: ScenarioValues): string {
  return scenarioVariables
    .map(([field, variable]) => {
      const value = scenario[field];
      if (!/^[A-Za-z0-9_-]*$/.test(value)) {
        throw new BuildError(
          `the scenario value ${JSON.stringify(value)} has a character other than a letter, a digit, "_" or "-"; the accepted values are in docs/dev/guides/development.md`,
        );
      }
      return `-X ${variable}=${value}`;
    })
    .join(" ");
}

/** Throws a BuildError naming what failed when result is not a success. */
function mustSucceed(what: string, result: CommandResult): CommandResult {
  if (result.status !== 0) {
    const detail = result.stderr.trim();
    throw new BuildError(
      `${what} failed (exit status ${String(result.status)})${detail === "" ? "" : `:\n${detail}`}`,
    );
  }
  return result;
}

/** The go command's settings the build depends on, as go.mod's toolchain sees them. */
interface GoSettings {
  readonly flags: string;
  readonly hostOS: string;
  readonly hostArch: string;
  readonly root: string;
  readonly version: string;
}

/** Reads the settings with one go env call, run with env. */
function readGoSettings(
  run: Runner,
  cwd: string,
  env: NodeJS.ProcessEnv,
): GoSettings {
  const names = ["GOFLAGS", "GOHOSTOS", "GOHOSTARCH", "GOROOT", "GOVERSION"];
  const result = mustSucceed(
    "go env",
    run("go", ["env", "-json", ...names], { cwd, env }),
  );
  let parsed: unknown;
  try {
    parsed = JSON.parse(result.stdout);
  } catch (error) {
    throw new BuildError(`go env printed no JSON: ${String(error)}`);
  }
  const values = (parsed ?? {}) as Record<string, unknown>;
  const value = (name: string): string => {
    const v = values[name];
    if (typeof v !== "string") {
      throw new BuildError(`go env did not report ${name}`);
    }
    return v;
  };
  return {
    flags: value("GOFLAGS"),
    hostOS: value("GOHOSTOS"),
    hostArch: value("GOHOSTARCH"),
    root: value("GOROOT"),
    version: value("GOVERSION"),
  };
}

/** Builds options.target into its output directory and checks it. */
export async function build(options: BuildOptions): Promise<void> {
  const { extensionDir, repoRoot, target, run } = options;
  const goMod = readFileSync(path.join(repoRoot, "go.mod"), "utf8");
  const toolchain = goToolchain(goMod);
  const modulePath = goModulePath(goMod);
  const goEnv = (extra: NodeJS.ProcessEnv): NodeJS.ProcessEnv => ({
    ...options.env,
    GOTOOLCHAIN: toolchain,
    ...extra,
  });
  const goSettings = readGoSettings(run, repoRoot, goEnv({}));
  // GOFLAGS, from the environment or from go env -w, would add flags (another
  // -ldflags, -tags) to every command; an empty variable does not clear a go
  // env -w value, so the build refuses any GOFLAGS instead.
  if (goSettings.flags !== "") {
    throw new BuildError(
      `GOFLAGS is "${goSettings.flags}"; the extension build runs only without it (unset it, or go env -u GOFLAGS)`,
    );
  }
  if (goSettings.version !== toolchain) {
    throw new BuildError(
      `go runs ${goSettings.version}, not go.mod's ${toolchain}; check that GOTOOLCHAIN can select it`,
    );
  }
  const mainPackage = `./${wasmMainPackage(target.kind)}`;

  const ldflags: string[] = [];
  if (target.kind === "dev") {
    ldflags.push("-ldflags", scenarioLdflags(target.scenario));
    // The host's GOOS and GOARCH, set explicitly: an empty variable would fall
    // back to a go env -w value.
    mustSucceed(
      "checking the scenario",
      run("go", ["run", ...ldflags, mainPackage], {
        cwd: repoRoot,
        env: goEnv({ GOOS: goSettings.hostOS, GOARCH: goSettings.hostArch }),
      }),
    );
  }

  const outName = outputDir(target.kind);
  const out = path.join(extensionDir, outName);
  rmSync(out, { recursive: true, force: true });
  mustSucceed(
    "tsc",
    run(
      process.execPath,
      [
        path.join(extensionDir, "node_modules", "typescript", "bin", "tsc"),
        "-p",
        "tsconfig.build.json",
        "--outDir",
        outName,
      ],
      { cwd: extensionDir, env: options.env },
    ),
  );
  // A static file whose path collides with a compiled file fails the build
  // instead of overwriting it.
  cpSync(path.join(extensionDir, "static"), out, {
    recursive: true,
    force: false,
    errorOnExist: true,
  });
  if (target.kind === "dev") {
    writeFileSync(
      path.join(out, "manifest.json"),
      devManifest(
        readFileSync(
          path.join(extensionDir, "static", "manifest.json"),
          "utf8",
        ),
      ),
    );
    writeFileSync(path.join(out, buildKindFileName), buildKindSource("dev"));
  }
  mustSucceed(
    "building the article generation module",
    run(
      "go",
      [
        "build",
        "-trimpath",
        ...ldflags,
        "-o",
        path.join(out, wasmFileName),
        mainPackage,
      ],
      { cwd: repoRoot, env: goEnv({ GOOS: "js", GOARCH: "wasm" }) },
    ),
  );
  const toolchainWasmExec = path.join(
    goSettings.root,
    "lib",
    "wasm",
    wasmExecFileName,
  );
  cpSync(toolchainWasmExec, path.join(out, wasmExecFileName));

  const violations = await options.check(extensionDir, {
    target,
    outDir: outName,
    modulePath,
    goVersion: toolchain,
    toolchainWasmExec: readFileSync(toolchainWasmExec),
    readBuildInfo: options.readBuildInfo,
  });
  if (violations.length > 0) {
    throw new BuildError(
      `${outName}/ does not match the build:\n${violations.map((v) => `  ${v}`).join("\n")}`,
    );
  }
}

/**
 * The build target the command line names: "release" with no options, or
 * "dev" with each of the five scenario options exactly once. Nothing has a
 * default; an unknown, repeated, or missing option is rejected.
 */
export function parseBuildArgs(argv: readonly string[]): BuildTarget {
  const names = {
    article: "article",
    failure: "failure",
    failAt: "fail-at",
    delaySeconds: "delay",
    settingsCheck: "settings-check",
  } as const;
  let parsed;
  try {
    parsed = parseArgs({
      args: [...argv],
      allowPositionals: true,
      strict: true,
      options: Object.fromEntries(
        Object.values(names).map((name) => [
          name,
          { type: "string", multiple: true },
        ]),
      ),
    });
  } catch (error) {
    throw new BuildError(String(error));
  }
  if (parsed.positionals.length !== 1) {
    throw new BuildError("give exactly one build kind: release or dev");
  }
  let kind;
  try {
    kind = parseBuildKind(parsed.positionals[0] ?? "");
  } catch (error) {
    throw new BuildError(String(error));
  }
  const given = parsed.values as Record<string, string[] | undefined>;
  if (kind === "release") {
    const extra = Object.keys(given);
    if (extra.length > 0) {
      throw new BuildError(
        `the release build takes no scenario; got --${extra.join(", --")}`,
      );
    }
    return { kind };
  }
  const value = (name: string): string => {
    const values = given[name] ?? [];
    if (values.length !== 1) {
      throw new BuildError(
        `give --${name} exactly once (given ${values.length} times)`,
      );
    }
    return values[0] ?? "";
  };
  return {
    kind,
    scenario: {
      article: value(names.article),
      failure: value(names.failure),
      failAt: value(names.failAt),
      delaySeconds: value(names.delaySeconds),
      settingsCheck: value(names.settingsCheck),
    },
  };
}

/** Runs a command and collects its output; never throws. */
function runCommand(
  command: string,
  args: readonly string[],
  options: { readonly cwd: string; readonly env: NodeJS.ProcessEnv },
): CommandResult {
  const result = spawnSync(command, args, {
    cwd: options.cwd,
    env: options.env,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
  const startFailure =
    result.error === undefined ? "" : `${String(result.error)}\n`;
  return {
    status: result.status,
    stdout: result.stdout ?? "",
    stderr: `${startFailure}${result.stderr ?? ""}`,
  };
}

/** The longest the module may take to load and answer buildInfo. */
const buildInfoTimeoutMs = 30_000;

/**
 * Loads the module in a separate Node.js process (scripts/read-build-info.ts),
 * so wasm_exec.js's globals and the Go program stay out of this one, and
 * returns what buildInfo returned.
 */
async function readBuildInfoInNode(
  wasmPath: string,
  wasmExecPath: string,
): Promise<string> {
  const reader = fileURLToPath(
    new URL("./read-build-info.ts", import.meta.url),
  );
  const result = spawnSync(process.execPath, [reader, wasmPath, wasmExecPath], {
    encoding: "utf8",
    timeout: buildInfoTimeoutMs,
  });
  if (result.error !== undefined) {
    throw new Error(
      `read-build-info did not finish within ${buildInfoTimeoutMs} ms or could not start: ${String(result.error)}`,
    );
  }
  if (result.status !== 0) {
    throw new Error(
      `read-build-info exited with ${String(result.status)}: ${result.stderr.trim()}`,
    );
  }
  return result.stdout;
}

if (import.meta.main) {
  const extensionDir = path.dirname(
    path.dirname(fileURLToPath(import.meta.url)),
  );
  try {
    await build({
      extensionDir,
      repoRoot: path.dirname(extensionDir),
      target: parseBuildArgs(process.argv.slice(2)),
      env: process.env,
      run: runCommand,
      check: checkDist,
      readBuildInfo: readBuildInfoInNode,
    });
  } catch (error) {
    if (!(error instanceof BuildError)) {
      throw error;
    }
    console.error(`build: ${error.message}`);
    process.exit(1);
  }
}
