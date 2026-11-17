// Verifies that a build's output directory holds only what the build is meant
// to produce, with the expectations of its kind (release in dist/, dev in
// dist-dev/): one .js per src/ .ts file plus the static/ files, linked by
// relative static imports, with the manifest and HTML references resolving
// inside the output directory, plus the article generation module and the Go
// toolchain's wasm_exec.js.
// Without a bundler, dependency code can reach the output only as an extra
// file, a symbolic link, or a non-relative module specifier, and all three
// fail here. The reference check catches a mistyped manifest or HTML path
// before the browser loads the extension.
// The two Go files are the declared exception to "only code this repository
// wrote": the module is checked by what it reports (its main package, its Go
// version, and its LLM implementation), and wasm_exec.js byte for byte
// against the toolchain's copy.
import { existsSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { isDeepStrictEqual } from "node:util";
import { JSDOM } from "jsdom";
import ts from "typescript";

import {
  buildKindFileName,
  buildKindSource,
  devManifest,
  outputDir,
  wasmExecFileName,
  wasmFileName,
  wasmMainPackage,
} from "./build-variant.ts";
import type { BuildKind } from "./build-variant.ts";

/** The five scenario values a development build embeds, as given to the build. */
export interface ScenarioValues {
  readonly article: string;
  readonly failure: string;
  readonly failAt: string;
  readonly delaySeconds: string;
  readonly settingsCheck: string;
}

/** A build's kind, with the scenario a development build embeds. */
export type BuildTarget =
  | { readonly kind: "release" }
  | { readonly kind: "dev"; readonly scenario: ScenarioValues };

/** What one build's output is checked against. */
export interface DistExpectation {
  readonly target: BuildTarget;
  /** The output directory checked, relative to the root. */
  readonly outDir: string;
  /** The Go module path, from go.mod's module line. */
  readonly modulePath: string;
  /** The Go version the module must report, such as "go1.26.5". */
  readonly goVersion: string;
  /** The toolchain's wasm_exec.js, which the output's copy must equal. */
  readonly toolchainWasmExec: Uint8Array;
  /**
   * Loads the module with wasm_exec.js and returns what its buildInfo
   * function returned. Injected, so the check's tests need no Go.
   */
  readonly readBuildInfo: (
    wasmPath: string,
    wasmExecPath: string,
  ) => Promise<string>;
}

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

/**
 * The output paths the build should produce: one .js per src/ .ts, the
 * static/ files, the build kind module, the article generation module, and
 * wasm_exec.js.
 */
function expectedFiles(root: string): Set<string> {
  const expected = new Set<string>([
    buildKindFileName,
    wasmFileName,
    wasmExecFileName,
  ]);
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

/**
 * The script src and link href values of an HTML document's real elements.
 * Parsing with a DOM keeps commented-out tags out and reads attributes with
 * the browser's own quoting rules, so a regex cannot miss or invent a
 * reference.
 */
function htmlReferences(html: string): string[] {
  const document = new JSDOM(html).window.document;
  const references: string[] = [];
  for (const element of document.querySelectorAll("script[src], link[href]")) {
    const reference =
      element.getAttribute("src") ?? element.getAttribute("href");
    if (reference !== null) {
      references.push(reference);
    }
  }
  return references;
}

/** The violations of the static imports of the output's own modules. */
function moduleViolations(out: string, actual: Set<string>): string[] {
  const violations: string[] = [];
  for (const file of actual) {
    // wasm_exec.js is the toolchain's script, checked by its bytes instead.
    if (!file.endsWith(".js") || file === wasmExecFileName) {
      continue;
    }
    const { specifiers, dynamicImport } = moduleReferences(
      file,
      readFileSync(path.join(out, file), "utf8"),
    );
    const outName = path.basename(out);
    if (dynamicImport) {
      violations.push(`${outName}/${file}: uses import()`);
    }
    for (const specifier of specifiers) {
      if (!specifier.startsWith("./") && !specifier.startsWith("../")) {
        violations.push(
          `${outName}/${file}: imports "${specifier}", which is not a relative path`,
        );
        continue;
      }
      const target = path.posix.normalize(
        path.posix.join(path.posix.dirname(file), specifier),
      );
      if (!actual.has(target)) {
        violations.push(
          `${outName}/${file}: imports "${specifier}", which is not in ${outName}/`,
        );
      }
    }
  }
  return violations;
}

/**
 * References the manifest and HTML files make that must resolve inside the
 * output. The manifest points at the popup page and the service worker; an
 * HTML page points at its scripts and stylesheets.
 */
function referenceViolations(out: string, actual: Set<string>): string[] {
  const outName = path.basename(out);
  const violations: string[] = [];
  if (actual.has("manifest.json")) {
    const manifest = JSON.parse(
      readFileSync(path.join(out, "manifest.json"), "utf8"),
    ) as {
      action?: { default_popup?: unknown };
      background?: { service_worker?: unknown };
    };
    const entries: [string, unknown][] = [
      ["action.default_popup", manifest.action?.default_popup],
      ["background.service_worker", manifest.background?.service_worker],
    ];
    for (const [label, reference] of entries) {
      if (typeof reference === "string" && !actual.has(reference)) {
        violations.push(
          `manifest.json: ${label} "${reference}" is not in ${outName}/`,
        );
      }
    }
  }
  for (const file of actual) {
    if (!file.endsWith(".html")) {
      continue;
    }
    const html = readFileSync(path.join(out, file), "utf8");
    for (const reference of htmlReferences(html)) {
      // A browser resolves src/href against the document URL.
      const baseOrigin = "https://extension.invalid";
      const resolved = new URL(reference, `${baseOrigin}/${file}`);
      if (resolved.origin !== baseOrigin) {
        continue;
      }
      const target = resolved.pathname.slice(1);
      if (!actual.has(target)) {
        violations.push(
          `${outName}/${file}: "${reference}" is not in ${outName}/`,
        );
      }
    }
  }
  return violations;
}

/**
 * Reports the output's manifest when it is not what the kind writes: for
 * release, a byte-for-byte copy of static/manifest.json, so the checks on the
 * static manifest hold for the one the browser loads; for dev, the
 * development manifest made from it (devManifest). Reports nothing unless
 * both are regular files: the missing, not-built and not-a-regular-file
 * checks already cover the other cases.
 */
function manifestViolations(
  root: string,
  out: string,
  kind: BuildKind,
  actual: Set<string>,
  expected: Set<string>,
): string[] {
  if (!actual.has("manifest.json") || !expected.has("manifest.json")) {
    return [];
  }
  const outName = path.basename(out);
  const source = readFileSync(path.join(root, "static", "manifest.json"));
  const copy = readFileSync(path.join(out, "manifest.json"));
  switch (kind) {
    case "release":
      return source.equals(copy)
        ? []
        : [`${outName}/manifest.json: differs from static/manifest.json`];
    case "dev": {
      let want: string;
      try {
        want = devManifest(source.toString("utf8"));
      } catch (error) {
        return [
          `static/manifest.json: cannot make the development manifest: ${String(error)}`,
        ];
      }
      return copy.equals(Buffer.from(want, "utf8"))
        ? []
        : [
            `${outName}/manifest.json: is not the development manifest made from static/manifest.json`,
          ];
    }
  }
}

/**
 * Reports each static/ file whose copy in the output differs from it. The
 * manifest and the build kind module are checked by their own rules, which
 * let a development build change them.
 */
function staticCopyViolations(
  root: string,
  out: string,
  actual: Set<string>,
): string[] {
  const violations: string[] = [];
  for (const file of listFiles(path.join(root, "static"))) {
    if (
      file === "manifest.json" ||
      file === buildKindFileName ||
      !actual.has(file)
    ) {
      continue;
    }
    const source = readFileSync(path.join(root, "static", file));
    if (!source.equals(readFileSync(path.join(out, file)))) {
      violations.push(
        `${path.basename(out)}/${file}: differs from static/${file}`,
      );
    }
  }
  return violations;
}

/** Reports the build kind module when it does not declare kind. */
function buildKindViolations(
  out: string,
  kind: BuildKind,
  actual: Set<string>,
): string[] {
  if (!actual.has(buildKindFileName)) {
    return [];
  }
  const text = readFileSync(path.join(out, buildKindFileName), "utf8");
  return text === buildKindSource(kind)
    ? []
    : [
        `${path.basename(out)}/${buildKindFileName}: does not declare "${kind}"`,
      ];
}

/** Reports wasm_exec.js when it is not the toolchain's file. */
function wasmExecViolations(
  out: string,
  actual: Set<string>,
  toolchainWasmExec: Uint8Array,
): string[] {
  if (!actual.has(wasmExecFileName)) {
    return [];
  }
  const copy = readFileSync(path.join(out, wasmExecFileName));
  return copy.equals(toolchainWasmExec)
    ? []
    : [
        `${path.basename(out)}/${wasmExecFileName}: differs from the toolchain's wasm_exec.js`,
      ];
}

/** True when value is a plain JSON object. */
function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * The delay the module reports for the spelling the build was given: the
 * module accepts only a canonical decimal spelling and reports it as a JSON
 * number. Any other spelling expects itself as text, which no module reports.
 */
function expectedDelay(spelling: string): number | string {
  return /^(0|[1-9][0-9]*)$/.test(spelling) ? Number(spelling) : spelling;
}

/** The llm member the module of target must report. */
function expectedLLM(target: BuildTarget): unknown {
  switch (target.kind) {
    case "release":
      return { kind: "claude" };
    case "dev":
      return {
        kind: "fake",
        scenario: {
          article: target.scenario.article,
          failure: target.scenario.failure,
          failAt: target.scenario.failAt,
          delaySeconds: expectedDelay(target.scenario.delaySeconds),
          settingsCheck: target.scenario.settingsCheck,
        },
      };
  }
}

/**
 * Loads the article generation module and checks what its buildInfo reports:
 * the main package (built from go.mod's module path), the Go version, and the
 * LLM implementation, with a development build's scenario. vcsRevision and
 * vcsModified are not checked: they change with the working tree.
 */
async function buildInfoViolations(
  out: string,
  expectation: DistExpectation,
  actual: Set<string>,
): Promise<string[]> {
  if (!actual.has(wasmFileName) || !actual.has(wasmExecFileName)) {
    return [];
  }
  const label = `${path.basename(out)}/${wasmFileName}: buildInfo`;
  let info: unknown;
  try {
    info = JSON.parse(
      await expectation.readBuildInfo(
        path.join(out, wasmFileName),
        path.join(out, wasmExecFileName),
      ),
    );
  } catch (error) {
    return [`${label}: cannot be read: ${String(error)}`];
  }
  if (!isObject(info)) {
    return [`${label}: is not a JSON object`];
  }
  const violations: string[] = [];
  const mainPath = `${expectation.modulePath}/${wasmMainPackage(expectation.target.kind)}`;
  if (info["mainPath"] !== mainPath) {
    violations.push(
      `${label}: mainPath is ${JSON.stringify(info["mainPath"])}, want "${mainPath}"`,
    );
  }
  if (info["goVersion"] !== expectation.goVersion) {
    violations.push(
      `${label}: goVersion is ${JSON.stringify(info["goVersion"])}, want "${expectation.goVersion}"`,
    );
  }
  const want = expectedLLM(expectation.target);
  // Compared as values, so the members' order does not matter.
  if (!isDeepStrictEqual(info["llm"], want)) {
    violations.push(
      `${label}: llm is ${JSON.stringify(info["llm"])}, want ${JSON.stringify(want)}`,
    );
  }
  return violations;
}

/**
 * Returns one message per problem found in the output directory of root;
 * empty when the output is what a build of the kind produces. A kind checked
 * against another kind's directory is itself a problem, and nothing else is
 * checked then.
 */
export async function checkDist(
  root: string,
  expectation: DistExpectation,
): Promise<string[]> {
  const { outDir } = expectation;
  const kind = expectation.target.kind;
  if (outputDir(kind) !== outDir) {
    return [
      `the ${kind} build is checked in ${outputDir(kind)}/, not ${outDir}/`,
    ];
  }
  const out = path.join(root, outDir);
  if (!existsSync(out)) {
    return [`${outDir}/ does not exist`];
  }
  const violations: string[] = [];
  // A symbolic link can point anywhere, node_modules included, and a
  // browser loading the output follows it; only regular files are accepted.
  for (const dir of ["static", outDir]) {
    for (const other of listEntries(path.join(root, dir)).others) {
      violations.push(`${dir}/${other}: not a regular file`);
    }
  }
  const actual = new Set(listFiles(out));
  const expected = expectedFiles(root);
  for (const file of actual) {
    if (!expected.has(file)) {
      violations.push(`${outDir}/${file}: not built from src/ or static/`);
    }
  }
  for (const file of expected) {
    if (!actual.has(file)) {
      violations.push(`${outDir}/${file}: missing`);
    }
  }
  violations.push(...moduleViolations(out, actual));
  violations.push(...manifestViolations(root, out, kind, actual, expected));
  violations.push(...buildKindViolations(out, kind, actual));
  violations.push(...staticCopyViolations(root, out, actual));
  violations.push(
    ...wasmExecViolations(out, actual, expectation.toolchainWasmExec),
  );
  violations.push(...referenceViolations(out, actual));
  violations.push(...(await buildInfoViolations(out, expectation, actual)));
  return violations;
}
