import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  mkdirSync,
  mkdtempSync,
  rmSync,
  symlinkSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, beforeEach, describe, it } from "node:test";

import { checkDist } from "../scripts/check-dist.ts";
import { extensionDir } from "./helpers/paths.ts";

/** Writes content to file under root, creating parent directories. */
function write(root: string, file: string, content = ""): void {
  const full = path.join(root, file);
  mkdirSync(path.dirname(full), { recursive: true });
  writeFileSync(full, content);
}

describe("checkDist", () => {
  const roots: string[] = [];
  let root = "";

  // A consistent tree: two sources, one static file, and their outputs.
  beforeEach(() => {
    root = mkdtempSync(path.join(tmpdir(), "check-dist-"));
    roots.push(root);
    write(root, "src/main.ts");
    write(root, "src/core/util.ts");
    write(root, "static/page.html");
    write(
      root,
      "dist/main.js",
      'import { f } from "./core/util.js";\nexport * from "./core/util.js";\nf();\n',
    );
    write(root, "dist/core/util.js", "export function f() {}\n");
    write(root, "dist/page.html");
  });
  after(() => {
    for (const dir of roots) {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  it("accepts dist/ built from src/ and static/", () => {
    assert.deepEqual(checkDist(root), []);
  });

  it("accepts a tree without static/", () => {
    rmSync(path.join(root, "static"), { recursive: true });
    unlinkSync(path.join(root, "dist/page.html"));
    assert.deepEqual(checkDist(root), []);
  });

  it("ignores declaration files in src/", () => {
    write(root, "src/globals.d.ts");
    assert.deepEqual(checkDist(root), []);
  });

  it("rejects a missing dist/", () => {
    rmSync(path.join(root, "dist"), { recursive: true });
    assert.deepEqual(checkDist(root), ["dist/ does not exist"]);
  });

  it("rejects an extra file", () => {
    write(root, "dist/node_modules/pkg/index.js");
    assert.deepEqual(checkDist(root), [
      "dist/node_modules/pkg/index.js: not built from src/ or static/",
    ]);
  });

  it("rejects a symbolic link in dist/", () => {
    write(root, "node_modules/pkg/index.js");
    symlinkSync("../node_modules/pkg/index.js", path.join(root, "dist/lib.js"));
    assert.deepEqual(checkDist(root), ["dist/lib.js: not a regular file"]);
  });

  it("rejects a symbolic link in static/ and its copy in dist/", () => {
    write(root, "node_modules/pkg/index.js");
    for (const dir of ["static", "dist"]) {
      symlinkSync(
        "../node_modules/pkg/index.js",
        path.join(root, dir, "lib.js"),
      );
    }
    assert.deepEqual(checkDist(root), [
      "static/lib.js: not a regular file",
      "dist/lib.js: not a regular file",
    ]);
  });

  it("exits 1 from the command line when dist/ has a violation", () => {
    const script = path.join(extensionDir, "scripts", "check-dist.ts");
    assert.equal(
      spawnSync(process.execPath, [script], { cwd: root }).status,
      0,
    );
    write(root, "dist/extra.js");
    assert.equal(
      spawnSync(process.execPath, [script], { cwd: root }).status,
      1,
    );
  });

  it("rejects a missing compiled file", () => {
    write(root, "src/extra.ts");
    assert.deepEqual(checkDist(root), ["dist/extra.js: missing"]);
  });

  it("rejects a missing static file", () => {
    write(root, "static/style.css");
    assert.deepEqual(checkDist(root), ["dist/style.css: missing"]);
  });

  const imports: { name: string; code: string; violation: string }[] = [
    {
      name: "a bare import",
      code: 'import "jsdom";\n',
      violation:
        'dist/core/util.js: imports "jsdom", which is not a relative path',
    },
    {
      name: "a bare import with bindings",
      code: 'import { JSDOM } from "jsdom";\nJSDOM;\n',
      violation:
        'dist/core/util.js: imports "jsdom", which is not a relative path',
    },
    {
      name: "a bare re-export",
      code: 'export { x } from "pkg";\n',
      violation:
        'dist/core/util.js: imports "pkg", which is not a relative path',
    },
    {
      name: "an absolute path",
      code: 'import "/main.js";\n',
      violation:
        'dist/core/util.js: imports "/main.js", which is not a relative path',
    },
    {
      name: "a relative path outside dist/",
      code: 'import "../../src/main.ts";\n',
      violation:
        'dist/core/util.js: imports "../../src/main.ts", which is not in dist/',
    },
    {
      name: "a relative path that is not built",
      code: 'import "./missing.js";\n',
      violation:
        'dist/core/util.js: imports "./missing.js", which is not in dist/',
    },
    {
      name: "a dynamic import",
      code: 'export const m = import("../main.js");\n',
      violation: "dist/core/util.js: uses import()",
    },
  ];
  for (const { name, code, violation } of imports) {
    it(`rejects ${name}`, () => {
      write(root, "dist/core/util.js", `export function f() {}\n${code}`);
      assert.deepEqual(checkDist(root), [violation]);
    });
  }
});
