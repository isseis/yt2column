import assert from "node:assert/strict";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, beforeEach, describe, it } from "node:test";

import {
  BuildError,
  build,
  goModulePath,
  goToolchain,
  parseBuildArgs,
  scenarioLdflags,
} from "../scripts/build.ts";
import type { BuildOptions, CommandResult, Runner } from "../scripts/build.ts";
import { buildKindSource, devManifest } from "../scripts/build-variant.ts";
import type { DistExpectation, ScenarioValues } from "../scripts/check-dist.ts";

describe("goToolchain", () => {
  it("names the toolchain of go.mod's go line", () => {
    assert.equal(
      goToolchain("module example.com/m\n\ngo 1.26.5\n\nignore ./x\n"),
      "go1.26.5",
    );
  });

  const rejected: { name: string; goMod: string }[] = [
    { name: "a go line without a patch version", goMod: "go 1.26\n" },
    { name: "a release candidate", goMod: "go 1.27rc1\n" },
    { name: "no go line", goMod: "module example.com/m\n" },
    { name: "two go lines", goMod: "go 1.26.5\ngo 1.26.6\n" },
  ];
  for (const { name, goMod } of rejected) {
    it(`stops on ${name}`, () => {
      assert.throws(() => goToolchain(goMod), BuildError);
    });
  }

  it("says why a go line without a patch version stops the build", () => {
    assert.throws(() => goToolchain("go 1.26\n"), /no patch version/);
  });

  it("reads the go line, not the toolchain line", () => {
    assert.equal(goToolchain("go 1.26.5\ntoolchain go1.27.1\n"), "go1.26.5");
  });
});

describe("goModulePath", () => {
  it("reads the module line", () => {
    assert.equal(
      goModulePath("module github.com/a/b\n\ngo 1.26.5\n"),
      "github.com/a/b",
    );
  });

  it("rejects go.mod without a module line", () => {
    assert.throws(() => goModulePath("go 1.26.5\n"), BuildError);
  });
});

/** A scenario whose every value differs from make ext-build-dev's defaults. */
const scenario: ScenarioValues = {
  article: "long",
  failure: "api_key_rejected",
  failAt: "shorten",
  delaySeconds: "12",
  settingsCheck: "key_rejected",
};

describe("scenarioLdflags", () => {
  it("sets each linker variable to its own value, once", () => {
    assert.equal(
      scenarioLdflags(scenario),
      "-X main.scenarioArticle=long -X main.scenarioFailure=api_key_rejected -X main.scenarioFailAt=shorten -X main.scenarioDelaySeconds=12 -X main.scenarioSettingsCheck=key_rejected",
    );
  });

  it("passes an empty value through for the module to reject", () => {
    assert.match(
      scenarioLdflags({ ...scenario, article: "" }),
      /-X main\.scenarioArticle= -X/,
    );
  });

  // Each would change what the go command hands the linker.
  for (const value of [
    "long -X main.x=y",
    "'long'",
    '"long"',
    "a\\b",
    "a\tb",
  ]) {
    it(`rejects ${JSON.stringify(value)}`, () => {
      assert.throws(
        () => scenarioLdflags({ ...scenario, failure: value }),
        BuildError,
      );
    });
  }
});

describe("parseBuildArgs", () => {
  const devArgs = [
    "dev",
    "--article",
    "long",
    "--failure",
    "api_key_rejected",
    "--fail-at",
    "shorten",
    "--delay",
    "12",
    "--settings-check",
    "key_rejected",
  ];

  it("reads release with no options", () => {
    assert.deepEqual(parseBuildArgs(["release"]), { kind: "release" });
  });

  it("reads dev with its five options, each to its own value", () => {
    assert.deepEqual(parseBuildArgs(devArgs), { kind: "dev", scenario });
  });

  const rejected: { name: string; argv: string[] }[] = [
    { name: "no kind", argv: [] },
    { name: "an unknown kind", argv: ["prod"] },
    { name: "two kinds", argv: ["release", "dev"] },
    { name: "a scenario for release", argv: ["release", "--delay", "1"] },
    { name: "a missing option", argv: devArgs.slice(0, -2) },
    { name: "a repeated option", argv: [...devArgs, "--delay", "1"] },
    { name: "an unknown option", argv: [...devArgs, "--model", "x"] },
  ];
  for (const { name, argv } of rejected) {
    it(`rejects ${name}`, () => {
      assert.throws(() => parseBuildArgs(argv), BuildError);
    });
  }
});

interface Call {
  readonly command: string;
  readonly args: readonly string[];
  readonly cwd: string;
  readonly env: NodeJS.ProcessEnv;
}

describe("build", () => {
  const dirs: string[] = [];
  let extensionDir = "";
  let repoRoot = "";
  let goroot = "";
  let calls: Call[] = [];
  let checked: DistExpectation[] = [];

  const manifest = JSON.stringify({ name: "__MSG_extName__", key: "k" });
  const wasmExec = "// toolchain wasm_exec.js\n";

  beforeEach(() => {
    const root = mkdtempSync(path.join(tmpdir(), "build-"));
    dirs.push(root);
    repoRoot = root;
    extensionDir = path.join(root, "extension");
    goroot = path.join(root, "goroot");
    mkdirSync(path.join(extensionDir, "static"), { recursive: true });
    mkdirSync(path.join(goroot, "lib", "wasm"), { recursive: true });
    writeFileSync(
      path.join(repoRoot, "go.mod"),
      "module example.com/m\n\ngo 1.26.5\n",
    );
    writeFileSync(path.join(extensionDir, "static", "manifest.json"), manifest);
    writeFileSync(
      path.join(extensionDir, "static", "buildKind.js"),
      buildKindSource("release"),
    );
    writeFileSync(path.join(goroot, "lib", "wasm", "wasm_exec.js"), wasmExec);
    calls = [];
    checked = [];
  });
  after(() => {
    for (const dir of dirs) {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  /**
   * A runner that records each call and answers like the real commands: go
   * run exits with runStatus, go env GOROOT prints the fake GOROOT, and the
   * other commands succeed.
   */
  function fakeRunner(runStatus = 0): Runner {
    return (command, args, options): CommandResult => {
      calls.push({ command, args, cwd: options.cwd, env: options.env });
      if (command === "go" && args[0] === "run") {
        return { status: runStatus, stdout: "", stderr: "not accepted" };
      }
      if (command === "go" && args[0] === "env") {
        return { status: 0, stdout: `${goroot}\n`, stderr: "" };
      }
      return { status: 0, stdout: "", stderr: "" };
    };
  }

  function options(
    target: BuildOptions["target"],
    overrides: Partial<BuildOptions> = {},
  ): BuildOptions {
    return {
      extensionDir,
      repoRoot,
      target,
      // Values the build must override, so a test sees that it does.
      env: { GOOS: "plan9", GOARCH: "386", GOFLAGS: "-ldflags=-X=x=y" },
      run: fakeRunner(),
      check: (_root, expectation) => {
        checked.push(expectation);
        return Promise.resolve([]);
      },
      readBuildInfo: () => Promise.resolve("{}"),
      ...overrides,
    };
  }

  const goCalls = (): Call[] => calls.filter((call) => call.command === "go");

  it("builds the release module with go.mod's toolchain and no -ldflags", async () => {
    await build(options({ kind: "release" }));
    const goBuild = goCalls().find((call) => call.args[0] === "build");
    assert.deepEqual(goBuild?.args, [
      "build",
      "-trimpath",
      "-o",
      path.join(extensionDir, "dist", "yt2column.wasm"),
      "./cmd/yt2column-wasm",
    ]);
    assert.equal(goBuild?.cwd, repoRoot);
    assert.equal(goBuild?.env["GOOS"], "js");
    assert.equal(goBuild?.env["GOARCH"], "wasm");
    assert.equal(goBuild?.env["GOFLAGS"], "");
    // No scenario check for release.
    assert.equal(
      goCalls().some((call) => call.args[0] === "run"),
      false,
    );
  });

  it("gives every go command go.mod's toolchain", async () => {
    await build(options({ kind: "dev", scenario }));
    assert.equal(goCalls().length, 3);
    for (const call of goCalls()) {
      assert.equal(call.env["GOTOOLCHAIN"], "go1.26.5", call.args.join(" "));
    }
  });

  it("compiles the sources into the kind's output directory", async () => {
    await build(options({ kind: "dev", scenario }));
    const tsc = calls.find((call) => call.command === process.execPath);
    assert.deepEqual(tsc?.args.slice(1), [
      "-p",
      "tsconfig.build.json",
      "--outDir",
      "dist-dev",
    ]);
    assert.equal(tsc?.cwd, extensionDir);
  });

  it("checks the scenario natively before building, and passes -ldflags once with five -X", async () => {
    await build(options({ kind: "dev", scenario }));
    const [first, ...rest] = calls;
    assert.equal(first?.command, "go");
    assert.deepEqual(first?.args, [
      "run",
      "-ldflags",
      scenarioLdflags(scenario),
      "./cmd/yt2column-wasm-dev",
    ]);
    assert.equal(first?.env["GOOS"], "");
    assert.equal(first?.env["GOARCH"], "");
    const goBuild = rest.find(
      (call) => call.command === "go" && call.args[0] === "build",
    );
    assert.ok(goBuild !== undefined);
    assert.deepEqual(
      goBuild.args.filter((arg) => arg === "-ldflags"),
      ["-ldflags"],
    );
    const flags = goBuild.args[goBuild.args.indexOf("-ldflags") + 1] ?? "";
    assert.equal(flags, scenarioLdflags(scenario));
    assert.equal(flags.split(" -X ").length, 5);
    assert.equal(goBuild.args.at(-1), "./cmd/yt2column-wasm-dev");
    assert.equal(goBuild.env["GOOS"], "js");
  });

  it("stops on a rejected scenario before touching dist-dev/", async () => {
    const marker = path.join(extensionDir, "dist-dev", "previous.js");
    mkdirSync(path.dirname(marker), { recursive: true });
    writeFileSync(marker, "");
    await assert.rejects(
      build(options({ kind: "dev", scenario }, { run: fakeRunner(1) })),
      (error) =>
        error instanceof BuildError && /not accepted/.test(error.message),
    );
    assert.ok(existsSync(marker), "dist-dev/ was touched");
    assert.equal(calls.length, 1, "ran more than the scenario check");
    assert.equal(checked.length, 0);
  });

  it("writes the development manifest and build kind module over the copies", async () => {
    await build(options({ kind: "dev", scenario }));
    const out = path.join(extensionDir, "dist-dev");
    assert.equal(
      readFileSync(path.join(out, "manifest.json"), "utf8"),
      devManifest(manifest),
    );
    assert.equal(
      readFileSync(path.join(out, "buildKind.js"), "utf8"),
      buildKindSource("dev"),
    );
  });

  it("copies static/ unchanged for release", async () => {
    await build(options({ kind: "release" }));
    const out = path.join(extensionDir, "dist");
    assert.equal(
      readFileSync(path.join(out, "manifest.json"), "utf8"),
      manifest,
    );
    assert.equal(
      readFileSync(path.join(out, "buildKind.js"), "utf8"),
      buildKindSource("release"),
    );
  });

  it("copies wasm_exec.js from the toolchain's GOROOT and checks it against those bytes", async () => {
    await build(options({ kind: "release" }));
    assert.equal(
      readFileSync(path.join(extensionDir, "dist", "wasm_exec.js"), "utf8"),
      wasmExec,
    );
    assert.equal(
      Buffer.from(checked[0]?.toolchainWasmExec ?? []).toString(),
      wasmExec,
    );
  });

  it("empties the output directory before building", async () => {
    const stale = path.join(extensionDir, "dist", "stale.js");
    mkdirSync(path.dirname(stale), { recursive: true });
    writeFileSync(stale, "");
    await build(options({ kind: "release" }));
    assert.equal(existsSync(stale), false);
  });

  it("checks the output with the kind's expectation", async () => {
    await build(options({ kind: "dev", scenario }));
    assert.equal(checked.length, 1);
    const [expectation] = checked;
    assert.deepEqual(expectation?.target, { kind: "dev", scenario });
    assert.equal(expectation?.outDir, "dist-dev");
    assert.equal(expectation?.modulePath, "example.com/m");
    assert.equal(expectation?.goVersion, "go1.26.5");
  });

  it("fails when the check reports a violation", async () => {
    await assert.rejects(
      build(
        options(
          { kind: "release" },
          { check: () => Promise.resolve(["dist/x.js: missing"]) },
        ),
      ),
      (error) =>
        error instanceof BuildError &&
        error.message.includes("dist/x.js: missing"),
    );
  });

  it("fails when a command fails", async () => {
    const run: Runner = (command, args, opts) =>
      command === "go" && args[0] === "build"
        ? { status: 1, stdout: "", stderr: "compile error" }
        : fakeRunner()(command, args, opts);
    await assert.rejects(
      build(options({ kind: "release" }, { run })),
      (error) =>
        error instanceof BuildError && /compile error/.test(error.message),
    );
    assert.equal(checked.length, 0);
  });
});
