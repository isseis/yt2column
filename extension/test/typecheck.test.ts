import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, before, describe, it } from "node:test";

import { extensionDir } from "./helpers/paths.ts";

const tsc = path.join(extensionDir, "node_modules", "typescript", "bin", "tsc");

const sources = {
  // Well typed under every setting: the control that shows a failure below
  // comes from the source, not from the temporary configuration.
  good: "export const n: number = 1;\n",
  // A type mismatch, an error with or without strict.
  mismatch: 'export const n: number = "one";\n',
  // Errors only under strict (strictNullChecks, noImplicitAny).
  nullToString: "export const s: string = null;\n",
  implicitAny: "export function f(x) {\n  return x;\n}\n",
};

type Source = keyof typeof sources;

describe("type errors fail typecheck and build", () => {
  let dir = "";
  let outDir = "";

  before(() => {
    dir = mkdtempSync(path.join(tmpdir(), "typecheck-"));
    outDir = path.join(dir, "out");
    // As in extension/package.json: every .ts file is an ES module.
    writeFileSync(path.join(dir, "package.json"), '{ "type": "module" }\n');
    for (const [name, text] of Object.entries(sources)) {
      writeFileSync(path.join(dir, `${name}.ts`), text);
    }
  });
  after(() => {
    rmSync(dir, { recursive: true, force: true });
  });

  // Runs tsc with a configuration that extends base and checks only file.
  // typeRoots points at the extension's node_modules because the temporary
  // directory has none; rootDir and outDir keep the file inside the project.
  function runTsc(
    base: string,
    file: Source,
  ): { status: number | null; output: string } {
    const config = path.join(dir, `${base}.${file}.json`);
    writeFileSync(
      config,
      JSON.stringify({
        extends: path.join(extensionDir, base),
        compilerOptions: {
          typeRoots: [path.join(extensionDir, "node_modules", "@types")],
          rootDir: dir,
          outDir,
        },
        include: [],
        files: [path.join(dir, `${file}.ts`)],
      }),
    );
    const result = spawnSync(process.execPath, [tsc, "-p", config], {
      encoding: "utf8",
    });
    return { status: result.status, output: result.stdout + result.stderr };
  }

  for (const base of ["tsconfig.json", "tsconfig.build.json"]) {
    it(`${base} accepts well-typed code`, () => {
      const { status, output } = runTsc(base, "good");
      assert.equal(status, 0, output);
    });

    const failures: { file: Source; code: string }[] = [
      { file: "mismatch", code: "TS2322" },
      { file: "nullToString", code: "TS2322" },
      { file: "implicitAny", code: "TS7006" },
    ];
    for (const { file, code } of failures) {
      it(`${base} rejects ${file} with ${code}`, () => {
        const { status, output } = runTsc(base, file);
        assert.notEqual(status, 0, output);
        assert.match(
          output,
          new RegExp(`${file}\\.ts\\(\\d+,\\d+\\): error ${code}:`),
        );
      });
    }
  }

  it("tsconfig.build.json emits only when there is no error", () => {
    rmSync(outDir, { recursive: true, force: true });
    assert.equal(runTsc("tsconfig.build.json", "good").status, 0);
    assert.equal(existsSync(path.join(outDir, "good.js")), true);

    assert.notEqual(runTsc("tsconfig.build.json", "mismatch").status, 0);
    assert.equal(existsSync(path.join(outDir, "mismatch.js")), false);
  });
});
