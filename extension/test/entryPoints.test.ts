import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import ts from "typescript";

import { extensionDir } from "./helpers/paths.ts";

// Static checks of where the extension may receive messages and send to the
// host. They read the syntax trees of src/, so a later change that moves a
// registration or a send fails here instead of slipping past review.

/** The extension source files under src/, as "/"-separated paths relative to extension/. */
function sourceFiles(): string[] {
  const src = path.join(extensionDir, "src");
  return readdirSync(src, { recursive: true, withFileTypes: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith(".ts"))
    .map((entry) =>
      path
        .relative(extensionDir, path.join(entry.parentPath, entry.name))
        .split(path.sep)
        .join("/"),
    )
    .sort();
}

function parse(file: string): ts.SourceFile {
  return ts.createSourceFile(
    file,
    readFileSync(path.join(extensionDir, file), "utf8"),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS,
  );
}

/** Every node of source that matches, in source order. */
function findAll<T extends ts.Node>(
  source: ts.SourceFile,
  match: (node: ts.Node) => node is T,
): T[] {
  const found: T[] = [];
  const visit = (node: ts.Node): void => {
    if (match(node)) {
      found.push(node);
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return found;
}

/** Whether call is name(...) or x.name(...). */
function calls(call: ts.CallExpression, name: string): boolean {
  const callee = call.expression;
  return (
    (ts.isIdentifier(callee) && callee.text === name) ||
    (ts.isPropertyAccessExpression(callee) && callee.name.text === name)
  );
}

/** Whether call is x.onMessage.addListener(...). */
function addsMessageListener(call: ts.CallExpression): boolean {
  const callee = call.expression;
  return (
    ts.isPropertyAccessExpression(callee) &&
    callee.name.text === "addListener" &&
    ts.isPropertyAccessExpression(callee.expression) &&
    callee.expression.name.text === "onMessage"
  );
}

/** The calls in src/ that match, with their files. */
function callsIn(
  files: readonly string[],
  match: (call: ts.CallExpression) => boolean,
): { file: string; call: ts.CallExpression; source: ts.SourceFile }[] {
  return files.flatMap((file) => {
    const source = parse(file);
    return findAll(source, ts.isCallExpression)
      .filter(match)
      .map((call) => ({ file, call, source }));
  });
}

/** The innermost function around node, or undefined at the top level. */
function enclosingFunction(
  node: ts.Node,
): ts.FunctionLikeDeclaration | undefined {
  for (let parent = node.parent; parent !== undefined; parent = parent.parent) {
    if (ts.isFunctionLike(parent) && !ts.isFunctionTypeNode(parent)) {
      return parent as ts.FunctionLikeDeclaration;
    }
  }
  return undefined;
}

/** The name a function is declared or bound with: function f, const f = () =>, f: () =>. */
function functionName(
  fn: ts.FunctionLikeDeclaration | undefined,
): string | undefined {
  if (fn === undefined) {
    return undefined;
  }
  if (fn.name !== undefined && ts.isIdentifier(fn.name)) {
    return fn.name.text;
  }
  const parent = fn.parent;
  if (
    (ts.isVariableDeclaration(parent) || ts.isPropertyAssignment(parent)) &&
    ts.isIdentifier(parent.name)
  ) {
    return parent.name.text;
  }
  return undefined;
}

/** Whether an await comes before node inside within. */
function awaitsBefore(within: ts.Node, node: ts.Node): boolean {
  let found = false;
  const visit = (child: ts.Node): void => {
    if (child.getStart() >= node.getStart()) {
      return;
    }
    if (
      ts.isAwaitExpression(child) ||
      (ts.isForOfStatement(child) && child.awaitModifier !== undefined)
    ) {
      found = true;
    }
    ts.forEachChild(child, visit);
  };
  ts.forEachChild(within, visit);
  return found;
}

const chromeDeps = "src/browser/chromeDeps.ts";
const registration = "listenForMessages";

describe("runtime message listener registration", () => {
  const files = sourceFiles();

  it("adds the onMessage listener only inside the registration wrapper", () => {
    const found = callsIn(files, addsMessageListener);
    assert.deepEqual(
      found.map(({ file, call }) => [
        file,
        functionName(enclosingFunction(call)),
      ]),
      [[chromeDeps, registration]],
    );
  });

  it("adds it synchronously, directly in the wrapper with no await before it", () => {
    const [found] = callsIn([chromeDeps], addsMessageListener);
    assert.ok(found);
    const wrapper = enclosingFunction(found.call);
    assert.ok(wrapper);
    assert.equal(functionName(wrapper), registration);
    assert.equal(
      (ts.canHaveModifiers(wrapper)
        ? ts.getModifiers(wrapper)
        : undefined
      )?.some((modifier) => modifier.kind === ts.SyntaxKind.AsyncKeyword) ??
        false,
      false,
      "the wrapper is async",
    );
    assert.equal(awaitsBefore(wrapper, found.call), false);
  });

  it("is imported and called by background.ts alone", () => {
    const importers = files.filter((file) =>
      findAll(parse(file), ts.isImportSpecifier).some(
        (specifier) =>
          (specifier.propertyName ?? specifier.name).text === registration,
      ),
    );
    assert.deepEqual(importers, ["src/background.ts"]);
    const called = callsIn(
      files.filter((file) => file !== chromeDeps),
      (call) => calls(call, registration),
    );
    assert.deepEqual(
      called.map(({ file }) => file),
      ["src/background.ts"],
    );
  });

  it("is called as a top-level statement of background.ts with no await before it", () => {
    const [found] = callsIn(["src/background.ts"], (call) =>
      calls(call, registration),
    );
    assert.ok(found);
    assert.ok(ts.isExpressionStatement(found.call.parent));
    assert.ok(ts.isSourceFile(found.call.parent.parent));
    assert.equal(awaitsBefore(found.source, found.call), false);
  });
});

describe("message entry points", () => {
  it("has no external message listener or externally_connectable", () => {
    const dirs = ["src", "static"].map((dir) => path.join(extensionDir, dir));
    for (const dir of dirs) {
      for (const entry of readdirSync(dir, {
        recursive: true,
        withFileTypes: true,
      })) {
        if (!entry.isFile()) {
          continue;
        }
        const text = readFileSync(
          path.join(entry.parentPath, entry.name),
          "utf8",
        );
        for (const name of ["onMessageExternal", "externally_connectable"]) {
          assert.ok(!text.includes(name), `${entry.name} names ${name}`);
        }
      }
    }
  });
});

describe("host sends", () => {
  const files = sourceFiles();

  it("builds the send dependencies only in the popup and service worker entries", () => {
    const found = callsIn(files, (call) => calls(call, "createSendDeps"));
    assert.deepEqual(
      [...new Set(found.map(({ file }) => file))],
      ["src/background.ts", "src/popup.ts"],
    );
  });

  it("builds a start message only in the handler of an accepted start request", () => {
    const found = callsIn(files, (call) => calls(call, "startMessage"));
    assert.deepEqual(
      found.map(({ file, call }) => [
        file,
        functionName(enclosingFunction(call)),
      ]),
      [["src/startCoordinator.ts", "runAcceptedStart"]],
    );
  });
});
