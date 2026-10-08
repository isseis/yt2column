// Verifies that dist/ holds only what the build is meant to produce: one .js
// per src/ .ts file plus the static/ files, linked by relative static
// imports. Without a bundler, dependency code can reach dist/ only as an
// extra file or through a non-relative module specifier, and both fail here.
import { existsSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import ts from "typescript";

/** Lists the files under dir as sorted "/"-separated relative paths; [] when dir is absent. */
function listFiles(dir: string): string[] {
  if (!existsSync(dir)) {
    return [];
  }
  return readdirSync(dir, { recursive: true, withFileTypes: true })
    .filter((entry) => entry.isFile())
    .map((entry) =>
      path
        .relative(dir, path.join(entry.parentPath, entry.name))
        .split(path.sep)
        .join("/"),
    )
    .sort();
}

function expectedFiles(root: string): Set<string> {
  const expected = new Set<string>();
  for (const file of listFiles(path.join(root, "src"))) {
    if (file.endsWith(".ts") && !file.endsWith(".d.ts")) {
      expected.add(file.slice(0, -".ts".length) + ".js");
    }
  }
  for (const file of listFiles(path.join(root, "static"))) {
    expected.add(file);
  }
  return expected;
}

/** Module specifiers of a module, and whether it calls import(). */
function moduleReferences(
  fileName: string,
  text: string,
): { specifiers: string[]; dynamicImport: boolean } {
  const source = ts.createSourceFile(
    fileName,
    text,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.JS,
  );
  const specifiers: string[] = [];
  let dynamicImport = false;
  const visit = (node: ts.Node): void => {
    if (
      (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
      node.moduleSpecifier !== undefined &&
      ts.isStringLiteral(node.moduleSpecifier)
    ) {
      specifiers.push(node.moduleSpecifier.text);
    } else if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword
    ) {
      dynamicImport = true;
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return { specifiers, dynamicImport };
}

/** Returns one message per problem found in root/dist; empty when dist/ is as expected. */
export function checkDist(root: string): string[] {
  const dist = path.join(root, "dist");
  if (!existsSync(dist)) {
    return ["dist/ does not exist"];
  }
  const violations: string[] = [];
  const actual = new Set(listFiles(dist));
  const expected = expectedFiles(root);
  for (const file of actual) {
    if (!expected.has(file)) {
      violations.push(`dist/${file}: not built from src/ or static/`);
    }
  }
  for (const file of expected) {
    if (!actual.has(file)) {
      violations.push(`dist/${file}: missing`);
    }
  }
  for (const file of actual) {
    if (!file.endsWith(".js")) {
      continue;
    }
    const { specifiers, dynamicImport } = moduleReferences(
      file,
      readFileSync(path.join(dist, file), "utf8"),
    );
    if (dynamicImport) {
      violations.push(`dist/${file}: uses import()`);
    }
    for (const specifier of specifiers) {
      if (!specifier.startsWith("./") && !specifier.startsWith("../")) {
        violations.push(
          `dist/${file}: imports "${specifier}", which is not a relative path`,
        );
        continue;
      }
      const target = path.posix.normalize(
        path.posix.join(path.posix.dirname(file), specifier),
      );
      if (!actual.has(target)) {
        violations.push(
          `dist/${file}: imports "${specifier}", which is not in dist/`,
        );
      }
    }
  }
  return violations;
}

if (import.meta.main) {
  const violations = checkDist(".");
  if (violations.length > 0) {
    console.error("check-dist: dist/ does not match the build:");
    for (const violation of violations) {
      console.error(`  ${violation}`);
    }
    process.exit(1);
  }
}
