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

/** Writes a manifest into static/ and its copy in dist/, pointing at files. */
function writeManifest(
  root: string,
  defaultPopup: string,
  optionsPage: string,
): void {
  const content = JSON.stringify({
    manifest_version: 3,
    action: { default_popup: defaultPopup },
    options_ui: { page: optionsPage, open_in_tab: true },
  });
  write(root, "static/manifest.json", content);
  write(root, "dist/manifest.json", content);
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

  it("accepts a manifest whose references exist", () => {
    write(root, "static/popup.html");
    write(root, "dist/popup.html");
    write(root, "static/options.html");
    write(root, "dist/options.html");
    writeManifest(root, "popup.html", "options.html");
    assert.deepEqual(checkDist(root), []);
  });

  // One row per manifest reference: the base tree has dist/main.js and
  // dist/page.html, so each row leaves exactly one reference unresolved.
  const missingReferences: {
    name: string;
    popup: string;
    optionsPage: string;
    violation: string;
  }[] = [
    {
      name: "popup",
      popup: "popup.html",
      optionsPage: "page.html",
      violation:
        'manifest.json: action.default_popup "popup.html" is not in dist/',
    },
    {
      name: "options page",
      popup: "page.html",
      optionsPage: "options.html",
      violation:
        'manifest.json: options_ui.page "options.html" is not in dist/',
    },
  ];
  for (const { name, popup, optionsPage, violation } of missingReferences) {
    it(`rejects a manifest ${name} that is not in dist/`, () => {
      writeManifest(root, popup, optionsPage);
      assert.deepEqual(checkDist(root), [violation]);
    });
  }

  it("rejects a dist/ manifest whose bytes differ from static/", () => {
    writeManifest(root, "page.html", "page.html");
    // Only whitespace changes, so the parsed JSON is the same and only a
    // byte comparison can tell the two files apart.
    const content = JSON.stringify(
      {
        manifest_version: 3,
        action: { default_popup: "page.html" },
        options_ui: { page: "page.html", open_in_tab: true },
      },
      null,
      2,
    );
    write(root, "dist/manifest.json", content);
    assert.deepEqual(checkDist(root), [
      "dist/manifest.json: differs from static/manifest.json",
    ]);
  });

  // With only one of the two manifests, the set comparison reports it and
  // the copy check must not add a second violation.
  const oneManifest: { name: string; remove: string; violation: string }[] = [
    {
      name: "static/",
      remove: "dist/manifest.json",
      violation: "dist/manifest.json: missing",
    },
    {
      name: "dist/",
      remove: "static/manifest.json",
      violation: "dist/manifest.json: not built from src/ or static/",
    },
  ];
  for (const { name, remove, violation } of oneManifest) {
    it(`reports only the set violation for a manifest only in ${name}`, () => {
      writeManifest(root, "page.html", "page.html");
      unlinkSync(path.join(root, remove));
      assert.deepEqual(checkDist(root), [violation]);
    });
  }

  it("accepts an HTML page whose references exist", () => {
    write(root, "static/style.css");
    write(root, "dist/style.css");
    write(root, "static/script.js");
    write(root, "dist/script.js");
    write(
      root,
      "dist/page.html",
      '<link rel="stylesheet" href="style.css"><script type="module" src="script.js"></script>',
    );
    assert.deepEqual(checkDist(root), []);
  });

  it("accepts single-quoted HTML references", () => {
    write(root, "static/style.css");
    write(root, "dist/style.css");
    write(root, "dist/page.html", "<link rel='stylesheet' href='style.css'>");
    assert.deepEqual(checkDist(root), []);
  });

  it("resolves references relative to the HTML file's directory", () => {
    write(root, "static/sub/page.html");
    write(
      root,
      "dist/sub/page.html",
      '<link rel="stylesheet" href="style.css">',
    );
    write(root, "static/sub/style.css");
    write(root, "dist/sub/style.css");
    assert.deepEqual(checkDist(root), []);
  });

  it("rejects a subdirectory page reference that is missing", () => {
    write(root, "static/sub/page.html");
    write(
      root,
      "dist/sub/page.html",
      '<script type="module" src="script.js"></script>',
    );
    assert.deepEqual(checkDist(root), [
      'dist/sub/page.html: "script.js" is not in dist/',
    ]);
  });

  it("rejects an HTML script that is not in dist/", () => {
    write(
      root,
      "dist/page.html",
      '<script type="module" src="missing.js"></script>',
    );
    assert.deepEqual(checkDist(root), [
      'dist/page.html: "missing.js" is not in dist/',
    ]);
  });

  it("rejects an HTML stylesheet that is not in dist/", () => {
    write(root, "dist/page.html", '<link rel="stylesheet" href="missing.css">');
    assert.deepEqual(checkDist(root), [
      'dist/page.html: "missing.css" is not in dist/',
    ]);
  });

  it("rejects a missing script after a > inside a quoted attribute", () => {
    write(
      root,
      "dist/page.html",
      '<script data-note=">" src="missing.js"></script>',
    );
    assert.deepEqual(checkDist(root), [
      'dist/page.html: "missing.js" is not in dist/',
    ]);
  });

  it("ignores a reference inside an HTML comment", () => {
    write(
      root,
      "dist/page.html",
      '<!-- <script src="missing.js"></script> -->',
    );
    assert.deepEqual(checkDist(root), []);
  });

  it("accepts a root-absolute script reference from a subdirectory page", () => {
    write(root, "static/sub/page.html");
    write(
      root,
      "dist/sub/page.html",
      '<script type="module" src="/script.js"></script>',
    );
    write(root, "static/script.js");
    write(root, "dist/script.js");
    assert.deepEqual(checkDist(root), []);
  });

  it("accepts a query-suffixed stylesheet reference", () => {
    write(root, "static/style.css");
    write(root, "dist/style.css");
    write(
      root,
      "dist/page.html",
      '<link rel="stylesheet" href="style.css?v=1">',
    );
    assert.deepEqual(checkDist(root), []);
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
