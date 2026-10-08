// Verifies that dist/ holds only what the build is meant to produce: one .js
// per src/ .ts file plus the static/ files, linked by relative static
// imports. Without a bundler, dependency code can reach dist/ only as an
// extra file, a symbolic link, or a non-relative module specifier, and all
// three fail here.
import { existsSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import ts from "typescript";

interface Listing {
  /** Regular files as sorted "/"-separated paths relative to the directory. */
  readonly files: string[];
  /** Entries that are neither regular files nor directories (symbolic links and the like). */
  readonly others: string[];
}

/** Lists the entries under dir; both lists are empty when dir is absent. */
function listEntries(dir: string): Listing {
  const files: string[] = [];
  const others: string[] = [];
  if (!existsSync(dir)) {
    return { files, others };
  }
  for (const entry of readdirSync(dir, {
    recursive: true,
    withFileTypes: true,
  })) {
    const relative = path
      .relative(dir, path.join(entry.parentPath, entry.name))
      .split(path.sep)
      .join("/");
    if (entry.isFile()) {
      files.push(relative);
    } else if (!entry.isDirectory()) {
      others.push(relative);
    }
  }
  return { files: files.sort(), others: others.sort() };
}

function listFiles(dir: string): string[] {
  return listEntries(dir).files;
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
  // A symbolic link can point anywhere, node_modules included, and a
  // browser loading dist/ follows it; only regular files are accepted.
  for (const dir of ["static", "dist"]) {
    for (const other of listEntries(path.join(root, dir)).others) {
      violations.push(`${dir}/${other}: not a regular file`);
    }
  }
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
