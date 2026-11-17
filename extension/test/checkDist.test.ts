import assert from "node:assert/strict";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  renameSync,
  rmSync,
  symlinkSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, beforeEach, describe, it } from "node:test";

import { buildKindSource, devManifest } from "../scripts/build-variant.ts";
import { checkDist } from "../scripts/check-dist.ts";
import type { DistExpectation, ScenarioValues } from "../scripts/check-dist.ts";

/** Writes content to file under root, creating parent directories. */
function write(root: string, file: string, content = ""): void {
  const full = path.join(root, file);
  mkdirSync(path.dirname(full), { recursive: true });
  writeFileSync(full, content);
}

/** Writes a manifest into static/ and its copy in dist/, pointing at the popup. */
function writeManifest(
  root: string,
  defaultPopup: string,
  serviceWorker?: string,
): void {
  const content = JSON.stringify({
    manifest_version: 3,
    action: { default_popup: defaultPopup },
    ...(serviceWorker === undefined
      ? {}
      : { background: { service_worker: serviceWorker, type: "module" } }),
  });
  write(root, "static/manifest.json", content);
  write(root, "dist/manifest.json", content);
}

/** The bytes the fake toolchain's wasm_exec.js holds. */
const toolchainWasmExec = "// the toolchain's wasm_exec.js\n";

const modulePath = "example.com/mod";
const goVersion = "go1.99.1";

/** A scenario whose every value differs from the defaults of make ext-build-dev. */
const scenario: ScenarioValues = {
  article: "long",
  failure: "empty",
  failAt: "shorten",
  delaySeconds: "3",
  settingsCheck: "key_rejected",
};

/** What a correct module of each kind reports from buildInfo. */
const releaseInfo = {
  goVersion,
  vcsRevision: "",
  vcsModified: true,
  mainPath: `${modulePath}/cmd/yt2column-wasm`,
  llm: { kind: "claude" },
};
const devInfo = {
  ...releaseInfo,
  mainPath: `${modulePath}/cmd/yt2column-wasm-dev`,
  llm: { kind: "fake", scenario: { ...scenario, delaySeconds: 3 } },
};

/** The expectation of a release build whose module reports info. */
function release(info: unknown = releaseInfo): DistExpectation {
  return {
    target: { kind: "release" },
    outDir: "dist",
    modulePath,
    goVersion,
    toolchainWasmExec: Buffer.from(toolchainWasmExec),
    readBuildInfo: () => Promise.resolve(JSON.stringify(info)),
  };
}

/** The expectation of a development build whose module reports info. */
function dev(info: unknown = devInfo): DistExpectation {
  return {
    ...release(info),
    target: { kind: "dev", scenario },
    outDir: "dist-dev",
  };
}

/** Writes an HTML page with content into static/ and its copy in dist/. */
function writePage(root: string, file: string, content: string): void {
  write(root, `static/${file}`, content);
  write(root, `dist/${file}`, content);
}

/** Writes a release manifest with a key into static/ and its copy in dist/. */
function writeManifestWithKey(root: string): void {
  const content = JSON.stringify({
    manifest_version: 3,
    name: "__MSG_extName__",
    key: "a-key",
    action: { default_popup: "page.html" },
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
    // What every build adds: the build kind module and the two Go files.
    write(root, "static/buildKind.js", buildKindSource("release"));
    write(root, "dist/buildKind.js", buildKindSource("release"));
    write(root, "dist/yt2column.wasm");
    write(root, "dist/wasm_exec.js", toolchainWasmExec);
  });
  after(() => {
    for (const dir of roots) {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  it("accepts dist/ built from src/ and static/", async () => {
    assert.deepEqual(await checkDist(root, release()), []);
  });

  it("accepts a tree without static/", async () => {
    rmSync(path.join(root, "static"), { recursive: true });
    unlinkSync(path.join(root, "dist/page.html"));
    assert.deepEqual(await checkDist(root, release()), []);
  });

  it("ignores declaration files in src/", async () => {
    write(root, "src/globals.d.ts");
    assert.deepEqual(await checkDist(root, release()), []);
  });

  it("rejects a missing dist/", async () => {
    rmSync(path.join(root, "dist"), { recursive: true });
    assert.deepEqual(await checkDist(root, release()), [
      "dist/ does not exist",
    ]);
  });

  it("rejects an extra file", async () => {
    write(root, "dist/node_modules/pkg/index.js");
    assert.deepEqual(await checkDist(root, release()), [
      "dist/node_modules/pkg/index.js: not built from src/ or static/",
    ]);
  });

  it("rejects a symbolic link in dist/", async () => {
    write(root, "node_modules/pkg/index.js");
    symlinkSync("../node_modules/pkg/index.js", path.join(root, "dist/lib.js"));
    assert.deepEqual(await checkDist(root, release()), [
      "dist/lib.js: not a regular file",
    ]);
  });

  it("rejects a symbolic link in static/ and its copy in dist/", async () => {
    write(root, "node_modules/pkg/index.js");
    for (const dir of ["static", "dist"]) {
      symlinkSync(
        "../node_modules/pkg/index.js",
        path.join(root, dir, "lib.js"),
      );
    }
    assert.deepEqual(await checkDist(root, release()), [
      "static/lib.js: not a regular file",
      "dist/lib.js: not a regular file",
    ]);
  });

  it("rejects a missing compiled file", async () => {
    write(root, "src/extra.ts");
    assert.deepEqual(await checkDist(root, release()), [
      "dist/extra.js: missing",
    ]);
  });

  it("rejects a missing static file", async () => {
    write(root, "static/style.css");
    assert.deepEqual(await checkDist(root, release()), [
      "dist/style.css: missing",
    ]);
  });

  it("accepts a manifest whose references exist", async () => {
    write(root, "static/popup.html");
    write(root, "dist/popup.html");
    writeManifest(root, "popup.html");
    assert.deepEqual(await checkDist(root, release()), []);
  });

  it("rejects a manifest popup that is not in dist/", async () => {
    // The base tree has dist/main.js and dist/page.html but no popup.html.
    writeManifest(root, "popup.html");
    assert.deepEqual(await checkDist(root, release()), [
      'manifest.json: action.default_popup "popup.html" is not in dist/',
    ]);
  });

  it("accepts a manifest service worker that is in dist/", async () => {
    // The base tree builds src/main.ts into dist/main.js.
    writeManifest(root, "page.html", "main.js");
    assert.deepEqual(await checkDist(root, release()), []);
  });

  it("rejects a manifest service worker that is in neither src/ nor dist/", async () => {
    writeManifest(root, "page.html", "background.js");
    assert.deepEqual(await checkDist(root, release()), [
      'manifest.json: background.service_worker "background.js" is not in dist/',
    ]);
  });

  it("rejects a dist/ manifest whose bytes differ from static/", async () => {
    writeManifest(root, "page.html");
    // Re-indent the dist/ copy: the parsed JSON stays the same, so only a
    // byte comparison can tell the two files apart.
    const original = readFileSync(
      path.join(root, "static/manifest.json"),
      "utf8",
    );
    const content = JSON.stringify(JSON.parse(original), null, 2);
    assert.notEqual(content, original);
    write(root, "dist/manifest.json", content);
    assert.deepEqual(await checkDist(root, release()), [
      "dist/manifest.json: differs from static/manifest.json",
    ]);
  });

  // With only one of the two manifests, the set comparison reports it and
  // the copy check must not add a violation of its own.
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
    it(`reports only the set violation for a manifest only in ${name}`, async () => {
      writeManifest(root, "page.html");
      unlinkSync(path.join(root, remove));
      assert.deepEqual(await checkDist(root, release()), [violation]);
    });
  }

  it("accepts an HTML page whose references exist", async () => {
    write(root, "static/style.css");
    write(root, "dist/style.css");
    write(root, "static/script.js");
    write(root, "dist/script.js");
    writePage(
      root,
      "page.html",
      '<link rel="stylesheet" href="style.css"><script type="module" src="script.js"></script>',
    );
    assert.deepEqual(await checkDist(root, release()), []);
  });

  it("accepts single-quoted HTML references", async () => {
    write(root, "static/style.css");
    write(root, "dist/style.css");
    writePage(root, "page.html", "<link rel='stylesheet' href='style.css'>");
    assert.deepEqual(await checkDist(root, release()), []);
  });

  it("resolves references relative to the HTML file's directory", async () => {
    write(root, "static/sub/page.html");
    writePage(
      root,
      "sub/page.html",
      '<link rel="stylesheet" href="style.css">',
    );
    write(root, "static/sub/style.css");
    write(root, "dist/sub/style.css");
    assert.deepEqual(await checkDist(root, release()), []);
  });

  it("rejects a subdirectory page reference that is missing", async () => {
    write(root, "static/sub/page.html");
    writePage(
      root,
      "sub/page.html",
      '<script type="module" src="script.js"></script>',
    );
    assert.deepEqual(await checkDist(root, release()), [
      'dist/sub/page.html: "script.js" is not in dist/',
    ]);
  });

  it("rejects an HTML script that is not in dist/", async () => {
    writePage(
      root,
      "page.html",
      '<script type="module" src="missing.js"></script>',
    );
    assert.deepEqual(await checkDist(root, release()), [
      'dist/page.html: "missing.js" is not in dist/',
    ]);
  });

  it("rejects an HTML stylesheet that is not in dist/", async () => {
    writePage(root, "page.html", '<link rel="stylesheet" href="missing.css">');
    assert.deepEqual(await checkDist(root, release()), [
      'dist/page.html: "missing.css" is not in dist/',
    ]);
  });

  it("rejects a missing script after a > inside a quoted attribute", async () => {
    writePage(
      root,
      "page.html",
      '<script data-note=">" src="missing.js"></script>',
    );
    assert.deepEqual(await checkDist(root, release()), [
      'dist/page.html: "missing.js" is not in dist/',
    ]);
  });

  it("ignores a reference inside an HTML comment", async () => {
    writePage(root, "page.html", '<!-- <script src="missing.js"></script> -->');
    assert.deepEqual(await checkDist(root, release()), []);
  });

  it("accepts a root-absolute script reference from a subdirectory page", async () => {
    write(root, "static/sub/page.html");
    writePage(
      root,
      "sub/page.html",
      '<script type="module" src="/script.js"></script>',
    );
    write(root, "static/script.js");
    write(root, "dist/script.js");
    assert.deepEqual(await checkDist(root, release()), []);
  });

  it("accepts a query-suffixed stylesheet reference", async () => {
    write(root, "static/style.css");
    write(root, "dist/style.css");
    writePage(
      root,
      "page.html",
      '<link rel="stylesheet" href="style.css?v=1">',
    );
    assert.deepEqual(await checkDist(root, release()), []);
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
    it(`rejects ${name}`, async () => {
      write(root, "dist/core/util.js", `export function f() {}\n${code}`);
      assert.deepEqual(await checkDist(root, release()), [violation]);
    });
  }

  /** Turns the base release tree into a development build's dist-dev/. */
  function makeDevTree(): void {
    writeManifestWithKey(root);
    renameSync(path.join(root, "dist"), path.join(root, "dist-dev"));
    write(
      root,
      "dist-dev/manifest.json",
      devManifest(
        readFileSync(path.join(root, "static/manifest.json"), "utf8"),
      ),
    );
    write(root, "dist-dev/buildKind.js", buildKindSource("dev"));
  }

  it("accepts a development build in dist-dev/", async () => {
    makeDevTree();
    assert.deepEqual(await checkDist(root, dev()), []);
  });

  // Each kind is checked only in its own output directory.
  const misplaced: { name: string; expectation: () => DistExpectation }[] = [
    {
      name: "release in dist-dev/",
      expectation: () => ({ ...release(), outDir: "dist-dev" }),
    },
    { name: "dev in dist/", expectation: () => ({ ...dev(), outDir: "dist" }) },
  ];
  for (const { name, expectation } of misplaced) {
    it(`rejects checking ${name}`, async () => {
      const violations = await checkDist(root, expectation());
      assert.equal(violations.length, 1, violations.join("\n"));
      assert.match(violations[0] ?? "", /is checked in/);
    });
  }

  it("rejects a release build whose build kind module declares dev", async () => {
    write(root, "dist/buildKind.js", buildKindSource("dev"));
    assert.deepEqual(await checkDist(root, release()), [
      'dist/buildKind.js: does not declare "release"',
    ]);
  });

  it("rejects a development build whose build kind module was not replaced", async () => {
    makeDevTree();
    write(root, "dist-dev/buildKind.js", buildKindSource("release"));
    assert.deepEqual(await checkDist(root, dev()), [
      'dist-dev/buildKind.js: does not declare "dev"',
    ]);
  });

  it("rejects a development manifest copied from static/ with its key", async () => {
    makeDevTree();
    write(
      root,
      "dist-dev/manifest.json",
      readFileSync(path.join(root, "static/manifest.json"), "utf8"),
    );
    assert.deepEqual(await checkDist(root, dev()), [
      "dist-dev/manifest.json: is not the development manifest made from static/manifest.json",
    ]);
  });

  it("rejects a wasm_exec.js that is not the toolchain's", async () => {
    write(root, "dist/wasm_exec.js", "// another wasm_exec.js\n");
    assert.deepEqual(await checkDist(root, release()), [
      "dist/wasm_exec.js: differs from the toolchain's wasm_exec.js",
    ]);
  });

  it("does not read wasm_exec.js as one of the extension's modules", async () => {
    // A dynamic import would fail any module of the extension; the
    // toolchain's script is checked only against the toolchain's bytes.
    const script = 'globalThis.load = () => import("x");\n';
    write(root, "dist/wasm_exec.js", script);
    const expectation = {
      ...release(),
      toolchainWasmExec: Buffer.from(script),
    };
    assert.deepEqual(await checkDist(root, expectation), []);
  });

  for (const file of ["yt2column.wasm", "wasm_exec.js"]) {
    it(`rejects a build without ${file}, without reading buildInfo`, async () => {
      unlinkSync(path.join(root, "dist", file));
      let read = false;
      const expectation = {
        ...release(),
        readBuildInfo: () => {
          read = true;
          return Promise.resolve(JSON.stringify(releaseInfo));
        },
      };
      assert.deepEqual(await checkDist(root, expectation), [
        `dist/${file}: missing`,
      ]);
      assert.equal(read, false);
    });
  }

  it("passes the output's module and wasm_exec.js to readBuildInfo", async () => {
    const calls: string[][] = [];
    const expectation = {
      ...release(),
      readBuildInfo: (wasm: string, wasmExec: string) => {
        calls.push([wasm, wasmExec]);
        return Promise.resolve(JSON.stringify(releaseInfo));
      },
    };
    assert.deepEqual(await checkDist(root, expectation), []);
    assert.deepEqual(calls, [
      [
        path.join(root, "dist", "yt2column.wasm"),
        path.join(root, "dist", "wasm_exec.js"),
      ],
    ]);
  });

  // One wrong buildInfo member per row, for each kind; every other member
  // is right, so the row's member alone is reported.
  const wrongInfo: {
    name: string;
    kind: "release" | "dev";
    info: unknown;
    member: string;
  }[] = [
    {
      name: "the development main package in a release build",
      kind: "release",
      info: { ...releaseInfo, mainPath: devInfo.mainPath },
      member: "mainPath",
    },
    {
      name: "the release main package in a development build",
      kind: "dev",
      info: { ...devInfo, mainPath: releaseInfo.mainPath },
      member: "mainPath",
    },
    {
      name: "another Go version",
      kind: "release",
      info: { ...releaseInfo, goVersion: "go1.99.0" },
      member: "goVersion",
    },
    {
      name: "the fake LLM in a release build",
      kind: "release",
      info: { ...releaseInfo, llm: devInfo.llm },
      member: "llm",
    },
    {
      name: "no LLM",
      kind: "release",
      info: { ...releaseInfo, llm: undefined },
      member: "llm",
    },
    {
      name: "Claude in a development build",
      kind: "dev",
      info: { ...devInfo, llm: releaseInfo.llm },
      member: "llm",
    },
    ...(
      [
        ["article", "standard"],
        ["failure", "none"],
        ["failAt", "none"],
        ["delaySeconds", 0],
        ["settingsCheck", "ok"],
      ] as const
    ).map(([field, value]) => ({
      name: `another scenario ${field}`,
      kind: "dev" as const,
      info: {
        ...devInfo,
        llm: {
          kind: "fake",
          scenario: { ...devInfo.llm.scenario, [field]: value },
        },
      },
      member: "llm",
    })),
    {
      name: "the delay as text instead of a number",
      kind: "dev",
      info: {
        ...devInfo,
        llm: {
          kind: "fake",
          scenario: { ...devInfo.llm.scenario, delaySeconds: "3" },
        },
      },
      member: "llm",
    },
    {
      name: "an extra scenario member",
      kind: "dev",
      info: {
        ...devInfo,
        llm: {
          kind: "fake",
          scenario: { ...devInfo.llm.scenario, extra: "x" },
        },
      },
      member: "llm",
    },
  ];
  for (const { name, kind, info, member } of wrongInfo) {
    it(`rejects a module that reports ${name}`, async () => {
      if (kind === "dev") {
        makeDevTree();
      }
      const expectation = kind === "dev" ? dev(info) : release(info);
      const violations = await checkDist(root, expectation);
      assert.equal(violations.length, 1, violations.join("\n"));
      assert.match(
        violations[0] ?? "",
        new RegExp(`: buildInfo: ${member} is `),
      );
    });
  }

  const unreadable: { name: string; read: () => Promise<string> }[] = [
    { name: "fails", read: () => Promise.reject(new Error("no buildInfo")) },
    { name: "is not JSON", read: () => Promise.resolve("{") },
    { name: "is not an object", read: () => Promise.resolve("[]") },
  ];
  for (const { name, read } of unreadable) {
    it(`rejects a module whose buildInfo ${name}`, async () => {
      const violations = await checkDist(root, {
        ...release(),
        readBuildInfo: read,
      });
      assert.equal(violations.length, 1, violations.join("\n"));
      assert.match(violations[0] ?? "", /^dist\/yt2column\.wasm: buildInfo: /);
    });
  }
  it("rejects a static file whose copy differs, in either kind", async () => {
    write(root, "dist/page.html", "<p>changed</p>");
    assert.deepEqual(await checkDist(root, release()), [
      "dist/page.html: differs from static/page.html",
    ]);
    makeDevTree();
    assert.deepEqual(await checkDist(root, dev()), [
      "dist-dev/page.html: differs from static/page.html",
    ]);
  });
});
