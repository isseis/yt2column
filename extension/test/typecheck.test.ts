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
  // What src/core/ may use (types/core-url.d.ts) and what it may not.
  usesUrl:
    'export const ids: string[] = new URL("https://a/?v=1").searchParams.getAll("v");\n',
  // What src/core/ may use (types/core-encoding.d.ts): text encoding and base64.
  usesTextEncoding:
    'export const n: number = new TextEncoder().encode("x").length;\n' +
    'export const s: string = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(new Uint8Array());\n' +
    'export const b: string = btoa(atob("YQ=="));\n',
  usesDocument: "export const t: string = document.title;\n",
  usesChrome: "export const id: string = chrome.runtime.id;\n",
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

  // Runs tsc with a configuration that extends base and checks only file
  // (plus the URL and encoding declarations tsconfig.core.json reads).
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
        files: [
          path.join(dir, `${file}.ts`),
          ...(base === "tsconfig.core.json"
            ? [
                path.join(extensionDir, "types", "core-url.d.ts"),
                path.join(extensionDir, "types", "core-encoding.d.ts"),
              ]
            : []),
        ],
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

  describe("core is checked without browser types", () => {
    for (const file of ["good", "usesUrl", "usesTextEncoding"] as const) {
      it(`tsconfig.core.json accepts ${file}`, () => {
        const { status, output } = runTsc("tsconfig.core.json", file);
        assert.equal(status, 0, output);
      });
    }

    const browserOnly: { file: Source; code: string }[] = [
      { file: "usesDocument", code: "TS2584" },
      { file: "usesChrome", code: "TS2304" },
    ];
    for (const { file, code } of browserOnly) {
      // The premise: the same file is well typed where browser types exist.
      it(`tsconfig.json accepts ${file}`, () => {
        const { status, output } = runTsc("tsconfig.json", file);
        assert.equal(status, 0, output);
      });
      it(`tsconfig.core.json rejects ${file} with ${code}`, () => {
        const { status, output } = runTsc("tsconfig.core.json", file);
        assert.notEqual(status, 0, output);
        assert.match(
          output,
          new RegExp(`${file}\\.ts\\(\\d+,\\d+\\): error ${code}:`),
        );
      });
    }
  });

  it("tsconfig.build.json emits only when there is no error", () => {
    rmSync(outDir, { recursive: true, force: true });
    assert.equal(runTsc("tsconfig.build.json", "good").status, 0);
    assert.equal(existsSync(path.join(outDir, "good.js")), true);

    assert.notEqual(runTsc("tsconfig.build.json", "mismatch").status, 0);
    assert.equal(existsSync(path.join(outDir, "mismatch.js")), false);
  });
});

/**
 * Defines a suite that type checks each of modules, a file made of header and
 * the module's body, against the extension's tsconfig.json. header gets a
 * function that turns a path under extension/src into the import specifier.
 * Only the type check runs, so a declared value needs no definition. A module
 * without a code is a control and must pass; a module with one must fail
 * with that error code.
 */
function typecheckSuite(
  title: string,
  header: (src: (file: string) => string) => string,
  modules: Readonly<Record<string, { body: string; code?: string }>>,
): void {
  describe(title, () => {
    let dir = "";

    before(() => {
      dir = mkdtempSync(path.join(tmpdir(), "typecheck-module-"));
      writeFileSync(path.join(dir, "package.json"), '{ "type": "module" }\n');
      const src = (file: string): string =>
        path
          .relative(dir, path.join(extensionDir, "src", file))
          .split(path.sep)
          .join("/");
      for (const [name, { body }] of Object.entries(modules)) {
        writeFileSync(path.join(dir, `${name}.ts`), header(src) + body);
      }
    });
    after(() => {
      rmSync(dir, { recursive: true, force: true });
    });

    // Unlike runTsc above, rootDir is not overridden: the module imports
    // a file under src/, which a rootDir of the temporary directory would
    // reject with TS6059.
    function typecheck(file: string): {
      status: number | null;
      output: string;
    } {
      const config = path.join(dir, `tsconfig.${file}.json`);
      writeFileSync(
        config,
        JSON.stringify({
          extends: path.join(extensionDir, "tsconfig.json"),
          compilerOptions: {
            typeRoots: [path.join(extensionDir, "node_modules", "@types")],
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

    for (const [file, { code }] of Object.entries(modules)) {
      if (code === undefined) {
        it(`accepts ${file}`, () => {
          const { status, output } = typecheck(file);
          assert.equal(status, 0, output);
        });
        continue;
      }
      it(`rejects ${file} with ${code}`, () => {
        const { status, output } = typecheck(file);
        assert.notEqual(status, 0, output);
        assert.match(
          output,
          new RegExp(`${file}\\.ts\\(\\d+,\\d+\\): error ${code}:`),
        );
      });
    }
  });
}

typecheckSuite(
  "a Catalog missing an item fails typecheck",
  (src) =>
    `import type { Catalog } from "${src("core/i18n/catalog.ts")}";\n` +
    "declare const full: Catalog;\n",
  {
    // The control: a value with every item is accepted.
    complete: { body: "export const c: Catalog = full;\n" },
    // A top-level item is missing.
    missingTopLevel: {
      body: 'export const c: Catalog = full as Omit<Catalog, "startResult">;\n',
      code: "TS2741",
    },
    // One key of a Record section is missing.
    missingRecordKey: {
      body:
        "export const c: Catalog = {\n" +
        "  ...full,\n" +
        '  rejection: full.rejection as Omit<Catalog["rejection"], "empty-title">,\n' +
        "};\n",
      code: "TS2741",
    },
  },
);

typecheckSuite(
  "sendNative accepts only HostMessage",
  (src) =>
    `import type { SendDeps } from "${src("send.ts")}";\n` +
    `import type { HostMessage } from "${src("core/hostMessage.ts")}";\n` +
    `import type { StartInput } from "${src("core/prepare.ts")}";\n` +
    "declare const deps: SendDeps;\n",
  {
    // The control: a message of the job API is accepted.
    hostMessage: {
      body:
        "declare const message: HostMessage;\n" +
        "export const sent = deps.sendNative(message);\n",
    },
    // The /articles body, the current form without v, is not a message.
    currentForm: {
      body:
        "declare const input: StartInput;\n" +
        "export const sent = deps.sendNative(input);\n",
      code: "TS2345",
    },
  },
);
