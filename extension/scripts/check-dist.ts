// Verifies that dist/ holds only what the build is meant to produce: one .js
// per src/ .ts file plus the static/ files, linked by relative static
// imports, with the manifest and HTML references resolving inside dist/.
// Without a bundler, dependency code can reach dist/ only as an extra file, a
// symbolic link, or a non-relative module specifier, and all three fail here.
// The reference check catches a mistyped manifest or HTML path before the
// browser loads the extension.
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

/** The regular files under dir, as "/"-separated paths; empty when dir is absent. */
function listFiles(dir: string): string[] {
  return listEntries(dir).files;
}

/** The dist/ paths the build should produce from src/ and static/. */
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

/** The value of a quoted attribute in a tag's text, or undefined when absent. */
function attributeValue(tag: string, name: string): string | undefined {
  const pattern = new RegExp(`\\s${name}\\s*=\\s*(?:"([^"]*)"|'([^']*)')`, "i");
  const match = pattern.exec(tag);
  if (match === null) {
    return undefined;
  }
  return match[1] ?? match[2];
}

/** The script src and link href values of an HTML document's text. */
function htmlReferences(html: string): string[] {
  const references: string[] = [];
  const tags: [string, string][] = [
    ["script", "src"],
    ["link", "href"],
  ];
  for (const [name, attribute] of tags) {
    const tag = new RegExp(`<${name}\\b(?:[^>"']|"[^"]*"|'[^']*')*>`, "gi");
    for (const match of html.matchAll(tag)) {
      const value = attributeValue(match[0], attribute);
      if (value !== undefined) {
        references.push(value);
      }
    }
  }
  return references;
}

/**
 * References the manifest and HTML files make that must resolve inside dist/.
 * The manifest points at the service worker and the popup page; an HTML page
 * points at its scripts and stylesheets.
 */
function referenceViolations(root: string, actual: Set<string>): string[] {
  const violations: string[] = [];
  if (actual.has("manifest.json")) {
    const manifest = JSON.parse(
      readFileSync(path.join(root, "dist", "manifest.json"), "utf8"),
    ) as {
      background?: { service_worker?: unknown };
      action?: { default_popup?: unknown };
    };
    const entries: [string, unknown][] = [
      ["background.service_worker", manifest.background?.service_worker],
      ["action.default_popup", manifest.action?.default_popup],
    ];
    for (const [label, reference] of entries) {
      if (typeof reference === "string" && !actual.has(reference)) {
        violations.push(
          `manifest.json: ${label} "${reference}" is not in dist/`,
        );
      }
    }
  }
  for (const file of actual) {
    if (!file.endsWith(".html")) {
      continue;
    }
    const html = readFileSync(path.join(root, "dist", file), "utf8");
    for (const reference of htmlReferences(html)) {
      // A browser resolves src/href against the document URL.
      const baseOrigin = "https://extension.invalid";
      const resolved = new URL(reference, `${baseOrigin}/${file}`);
      if (resolved.origin !== baseOrigin) {
        continue;
      }
      const target = resolved.pathname.slice(1);
      if (!actual.has(target)) {
        violations.push(`dist/${file}: "${reference}" is not in dist/`);
      }
    }
  }
  return violations;
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
  violations.push(...referenceViolations(root, actual));
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
