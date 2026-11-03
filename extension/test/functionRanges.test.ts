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

/** One line range, 1-based and inclusive. */
interface LineRange {
  readonly start: number;
  readonly end: number;
}

/** The ranges the helper reports for one source file. */
function rangesOf(contents: string): {
  functions: LineRange[];
  fieldGroups: LineRange[];
} {
  const root = makeRoot({ "src/a.ts": contents });
  const entry = collectFunctionRanges(root, ["src/a.ts"]).files[0];
  assert.ok(entry, "the answer has no entry for the requested file");
  return entry;
}

describe("collectFunctionRanges", () => {
  const cases: {
    name: string;
    source: string;
    functions: LineRange[];
    fieldGroups?: LineRange[];
  }[] = [
    {
      name: "finds a function declaration without its leading comment",
      source: [
        "/** Returns one. */",
        "export function one(): number {",
        "  return 1;",
        "}",
        "",
      ].join("\n"),
      functions: [{ start: 2, end: 4 }],
    },
    {
      name: "finds function expressions and arrow functions",
      source: [
        "export const f = function (): number {",
        "  return 1;",
        "};",
        "export const g = (): number => 2;",
        "",
      ].join("\n"),
      functions: [
        { start: 1, end: 3 },
        { start: 4, end: 4 },
      ],
    },
    {
      name: "finds nested functions and two functions that start on one line",
      source: [
        "export const f = (n: number): number => {",
        "  const g = (m: number): number => m + 1;",
        "  const h = (m: number): number => m; const k = (m: number): number => m;",
        "  return g(n) + h(n) + k(n);",
        "};",
        "",
      ].join("\n"),
      functions: [
        { start: 1, end: 5 },
        { start: 2, end: 2 },
        { start: 3, end: 3 },
        { start: 3, end: 3 },
      ],
    },
    {
      name: "finds methods, constructors, and accessors",
      source: [
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
      functions: [
        { start: 3, end: 5 },
        { start: 6, end: 8 },
        { start: 9, end: 11 },
        { start: 12, end: 14 },
      ],
      fieldGroups: [{ start: 2, end: 2 }],
    },
    {
      name: "finds the methods and fields of a class expression",
      source: [
        "export const A = class {",
        "  n = 0;",
        "  method(): void {}",
        "};",
        "",
      ].join("\n"),
      functions: [{ start: 3, end: 3 }],
      fieldGroups: [{ start: 2, end: 2 }],
    },
    {
      name: "skips a method signature without a body",
      source: [
        "export abstract class A {",
        "  abstract method(): void;",
        "  n = 0;",
        "}",
        "",
      ].join("\n"),
      functions: [],
      fieldGroups: [{ start: 3, end: 3 }],
    },
    {
      name: "groups instance fields from the first to the last without static ones",
      source: [
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
      functions: [],
      fieldGroups: [
        { start: 3, end: 4 },
        { start: 7, end: 7 },
      ],
    },
  ];

  for (const { name, source, functions, fieldGroups = [] } of cases) {
    it(name, () => {
      const entry = rangesOf(source);
      assert.deepEqual(entry.functions, functions);
      assert.deepEqual(entry.fieldGroups, fieldGroups);
    });
  }

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

  // Runs the script with the given arguments in cwd; returns its status and
  // stdout.
  function run(
    args: string[],
    cwd = extensionDir,
  ): {
    status: number | null;
    stdout: string;
  } {
    const result = spawnSync(process.execPath, [script, ...args], {
      cwd,
      encoding: "utf8",
    });
    return { status: result.status, stdout: result.stdout };
  }

  it("prints the ranges as one line holding a JSON object", () => {
    const root = makeRoot({ "src/a.ts": "export function f(): void {}\n" });
    const result = run(["--root", root, "--", "src/a.ts"]);
    assert.equal(result.status, 0);
    assert.equal(result.stdout.trimEnd().split("\n").length, 1);
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

  it("lists the requested files once each, in the requested order", () => {
    const root = makeRoot({
      "src/a.ts": "export const a = 1;\n",
      "scripts/b.ts": "export const b = 2;\n",
    });
    const result = run(["--root", root, "--", "src/a.ts", "scripts/b.ts"]);
    assert.equal(result.status, 0);
    const answer: { files: { path: string }[] } = JSON.parse(result.stdout);
    assert.deepEqual(
      answer.files.map((file) => file.path),
      ["src/a.ts", "scripts/b.ts"],
    );
  });

  const failures: { name: string; args: (root: string) => string[] }[] = [
    { name: "no arguments", args: () => [] },
    { name: "no --root option", args: () => ["--", "src/a.ts"] },
    { name: "no -- separator", args: (root) => ["--root", root, "src/a.ts"] },
    { name: "no path", args: (root) => ["--root", root, "--"] },
    {
      name: "a different option",
      args: (root) => ["--base", root, "--", "src/a.ts"],
    },
    {
      name: "a file that cannot be read",
      args: (root) => ["--root", root, "--", "src/missing.ts"],
    },
    { name: "a dot path", args: (root) => ["--root", root, "--", "."] },
  ];
  for (const { name, args } of failures) {
    it(`exits 1 and prints nothing for ${name}`, () => {
      const root = makeRoot({ "src/a.ts": "export const x = 1;\n" });
      const result = run(args(root));
      assert.equal(result.status, 1);
      assert.equal(result.stdout, "");
    });
  }

  it("rejects an empty root even when the working directory has the file", () => {
    const cwd = makeRoot({ "src/a.ts": "export const x = 1;\n" });
    const result = run(["--root", "", "--", "src/a.ts"], cwd);
    assert.equal(result.status, 1);
    assert.equal(result.stdout, "");
  });

  it("rejects a path outside the root even when the file exists", () => {
    const parent = mkdtempSync(path.join(tmpdir(), "function-ranges-parent-"));
    roots.push(parent);
    writeFileSync(path.join(parent, "outside.ts"), "export const x = 1;\n");
    const root = path.join(parent, "inner");
    mkdirSync(root);
    const result = run(["--root", root, "--", "../outside.ts"]);
    assert.equal(result.status, 1);
    assert.equal(result.stdout, "");
  });
});
