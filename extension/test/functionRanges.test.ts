import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { after, describe, it } from "node:test";

import { collectFunctionRanges } from "../scripts/function-ranges.ts";
import { extensionDir } from "./helpers/paths.ts";

const roots: string[] = [];

/** Creates a temporary tree holding the given files. */
function makeRoot(files: Record<string, string>): string {
  const root = mkdtempSync(path.join(tmpdir(), "function-ranges-"));
  roots.push(root);
  for (const [name, content] of Object.entries(files)) {
    const full = path.join(root, name);
    mkdirSync(path.dirname(full), { recursive: true });
    writeFileSync(full, content);
  }
  return root;
}

after(() => {
  for (const root of roots) {
    rmSync(root, { recursive: true, force: true });
  }
});

/** The ranges the helper reports for one source file. */
function rangesOf(contents: string): {
  functions: { start: number; end: number }[];
  fieldGroups: { start: number; end: number }[];
} {
  const root = makeRoot({ "src/a.ts": contents });
  const entry = collectFunctionRanges(root, ["src/a.ts"]).files[0];
  assert.ok(entry, "the answer has no entry for the requested file");
  assert.equal(entry.path, "src/a.ts");
  return entry;
}

describe("collectFunctionRanges", () => {
  it("finds a function declaration without its leading comment", () => {
    const entry = rangesOf(
      [
        "/** Returns one. */",
        "export function one(): number {",
        "  return 1;",
        "}",
        "",
      ].join("\n"),
    );
    assert.deepEqual(entry.functions, [{ start: 2, end: 4 }]);
  });

  it("finds function expressions and arrow functions", () => {
    const entry = rangesOf(
      [
        "export const f = function (): number {",
        "  return 1;",
        "};",
        "export const g = (): number => 2;",
        "",
      ].join("\n"),
    );
    assert.deepEqual(entry.functions, [
      { start: 1, end: 3 },
      { start: 4, end: 4 },
    ]);
  });

  it("finds nested functions and two functions that start on one line", () => {
    const entry = rangesOf(
      [
        "export const f = (n: number): number => {",
        "  const g = (m: number): number => m + 1;",
        "  const h = (m: number): number => m; const k = (m: number): number => m;",
        "  return g(n) + h(n) + k(n);",
        "};",
        "",
      ].join("\n"),
    );
    assert.deepEqual(entry.functions, [
      { start: 1, end: 5 },
      { start: 2, end: 2 },
      { start: 3, end: 3 },
      { start: 3, end: 3 },
    ]);
  });

  it("finds methods, constructors, and accessors", () => {
    const entry = rangesOf(
      [
        "export class A {",
        "  n = 0;",
        "  constructor(n: number) {",
        "    this.n = n;",
        "  }",
        "  get value(): number {",
        "    return this.n;",
        "  }",
        "  set value(n: number) {",
        "    this.n = n;",
        "  }",
        "  method(): number {",
        "    return this.n;",
        "  }",
        "}",
        "",
      ].join("\n"),
    );
    assert.deepEqual(entry.functions, [
      { start: 3, end: 5 },
      { start: 6, end: 8 },
      { start: 9, end: 11 },
      { start: 12, end: 14 },
    ]);
  });

  it("groups instance fields from the first to the last without static ones", () => {
    const entry = rangesOf(
      [
        "export class A {",
        "  static first = 0;",
        "  a = 1;",
        "  c = 2;",
        "}",
        "export class B {",
        "  b = 3;",
        "  static last = 4;",
        "}",
        "",
      ].join("\n"),
    );
    assert.deepEqual(entry.fieldGroups, [
      { start: 3, end: 4 },
      { start: 7, end: 7 },
    ]);
  });

  it("reads the tree given as the root", () => {
    const first = makeRoot({
      "src/a.ts": "export function long(): number {\n  return 1;\n}\n",
    });
    const second = makeRoot({
      "src/a.ts": "export const short = (): number => 1;\n",
    });
    const one = collectFunctionRanges(first, ["src/a.ts"]).files[0];
    const other = collectFunctionRanges(second, ["src/a.ts"]).files[0];
    assert.deepEqual(one?.functions, [{ start: 1, end: 3 }]);
    assert.deepEqual(other?.functions, [{ start: 1, end: 1 }]);
  });
});

describe("function-ranges command", () => {
  const script = path.join(extensionDir, "scripts", "function-ranges.ts");

  // Runs the script with the given arguments; returns its status and stdout.
  function run(args: string[]): {
    status: number | null;
    stdout: string;
  } {
    const result = spawnSync(process.execPath, [script, ...args], {
      encoding: "utf8",
    });
    return { status: result.status, stdout: result.stdout };
  }

  it("prints the ranges as one JSON object", () => {
    const root = makeRoot({ "src/a.ts": "export function f(): void {}\n" });
    const result = run(["--root", root, "--", "src/a.ts"]);
    assert.equal(result.status, 0);
    assert.deepEqual(JSON.parse(result.stdout), {
      files: [
        {
          path: "src/a.ts",
          functions: [{ start: 1, end: 1 }],
          fieldGroups: [],
        },
      ],
    });
  });

  const failures: { name: string; args: (root: string) => string[] }[] = [
    { name: "no arguments", args: () => [] },
    { name: "no --root option", args: () => ["--", "src/a.ts"] },
    { name: "no -- separator", args: (root) => ["--root", root, "src/a.ts"] },
    {
      name: "a different option",
      args: (root) => ["--base", root, "--", "src/a.ts"],
    },
    {
      name: "a file that cannot be read",
      args: (root) => ["--root", root, "--", "src/missing.ts"],
    },
  ];
  for (const { name, args } of failures) {
    it(`exits non-zero and prints nothing for ${name}`, () => {
      const root = makeRoot({ "src/a.ts": "export const x = 1;\n" });
      const result = run(args(root));
      assert.notEqual(result.status, 0);
      assert.equal(result.stdout, "");
    });
  }

  it("rejects a path outside the root even when the file exists", () => {
    const parent = mkdtempSync(path.join(tmpdir(), "function-ranges-parent-"));
    roots.push(parent);
    writeFileSync(path.join(parent, "outside.ts"), "export const x = 1;\n");
    const root = path.join(parent, "inner");
    mkdirSync(root);
    const result = run(["--root", root, "--", "../outside.ts"]);
    assert.notEqual(result.status, 0);
    assert.equal(result.stdout, "");
  });
});
