// Checks that moving the popup's colour variables and shared rules from
// style.css to theme.css leaves every value the popup shows before a send
// unchanged. Run from the repository root, after `git fetch origin main` and
// `make ext-install`:
//
//   node docs/tasks/0050_viewer_tab/_checks/presend_popup_styles.ts
//
// The pages are the before-send popup pages HEAD's popup-preview.ts writes
// (every collected and rejected preview case), taken from HEAD's commit, never
// from the working tree. Each page is styled twice: with the merge-base's
// style.css, and with HEAD's theme.css followed by HEAD's style.css, in the
// order popup.html loads them. For every element, jsdom's cascade gives the
// winning declaration of each property; a var() in it is replaced with the
// variable's value on the root element, so a changed colour is caught even
// where the declaration still names the same variable. The variables
// themselves are not compared: one that no element uses, such as a colour
// only the viewer needs, shows nothing in the popup.
//
// Each page is compared in sixteen variants: light and dark (the dark block's
// media condition rewritten to always or never apply, since jsdom does not
// evaluate prefers-color-scheme), with and without every element matching
// :focus-visible, with reduced motion on and off, and with the buttons
// disabled as drawn and enabled as the send path leaves a sendable one.
//
// Exits 0 when every value is identical, 1 when any differs (printing the
// differences), and 2 when the check cannot run.

import { execFileSync, spawnSync } from "node:child_process";
import {
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  symlinkSync,
} from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import path from "node:path";

/** The part of the jsdom API this check uses. */
interface JsdomModule {
  readonly JSDOM: new (html: string) => { readonly window: Window };
}

let workDir = "";

function fail(message: string): never {
  if (workDir !== "") {
    rmSync(workDir, { recursive: true, force: true });
  }
  process.stderr.write(`presend_popup_styles: ${message}\n`);
  process.exit(2);
}

function git(...args: string[]): string {
  return execFileSync("git", args, { encoding: "utf8" }).trim();
}

/** The file's text at rev. */
function show(rev: string, file: string): string {
  return execFileSync("git", ["show", `${rev}:${file}`], { encoding: "utf8" });
}

/** One way of applying a stylesheet to a page. */
interface Variant {
  readonly dark: boolean;
  readonly focus: boolean;
  readonly reducedMotion: boolean;
  readonly enabled: boolean;
}

const variants: Variant[] = [];
for (const dark of [false, true]) {
  for (const focus of [false, true]) {
    for (const reducedMotion of [false, true]) {
      for (const enabled of [false, true]) {
        variants.push({ dark, focus, reducedMotion, enabled });
      }
    }
  }
}

function variantName(variant: Variant): string {
  return [
    variant.dark ? "dark" : "light",
    variant.focus ? "focus" : "no-focus",
    variant.reducedMotion ? "reduced-motion" : "motion",
    variant.enabled ? "enabled" : "disabled",
  ].join(",");
}

/**
 * The stylesheet as the variant applies it: each media condition the variant
 * decides is rewritten to one that always or never applies, and :focus-visible
 * to an attribute the variant sets on every element.
 */
function applyVariant(css: string, variant: Variant): string {
  const always = "all";
  const never = "not all";
  return css
    .replace(
      /\(\s*prefers-color-scheme\s*:\s*dark\s*\)/g,
      variant.dark ? always : never,
    )
    .replace(
      /\(\s*prefers-reduced-motion\s*:\s*reduce\s*\)/g,
      variant.reducedMotion ? always : never,
    )
    .replaceAll(":focus-visible", "[data-focus-visible]");
}

/** A path that names the element by its position, the same in both runs. */
function elementPath(element: Element): string {
  const parts: string[] = [];
  let current: Element | null = element;
  while (current !== null) {
    const parent: Element | null = current.parentElement;
    const index =
      parent === null
        ? 0
        : Array.prototype.indexOf.call(parent.children, current);
    parts.unshift(`${current.tagName.toLowerCase()}[${index}]`);
    current = parent;
  }
  return parts.join(">");
}

/**
 * Every element's winning declarations on the page, styled with css under the
 * variant, one line per element and property, var() resolved.
 */
function computedLines(
  jsdom: JsdomModule,
  page: string,
  css: string,
  variant: Variant,
): string[] {
  const window = new jsdom.JSDOM(page).window;
  const doc = window.document;
  for (const link of doc.querySelectorAll("link[rel=stylesheet]")) {
    link.remove();
  }
  const style = doc.createElement("style");
  style.textContent = applyVariant(css, variant);
  doc.head.append(style);
  for (const element of doc.querySelectorAll("*")) {
    if (variant.focus) {
      element.setAttribute("data-focus-visible", "");
    }
    if (variant.enabled && element.tagName.toLowerCase() === "button") {
      element.removeAttribute("disabled");
    }
  }
  const root = window.getComputedStyle(doc.documentElement);
  const resolve = (value: string): string =>
    value.replace(/var\(\s*(--[a-z0-9-]+)\s*\)/gi, (whole, name: string) => {
      const resolved = root.getPropertyValue(name).trim();
      return resolved === "" ? `${whole}=unset` : `${whole}=${resolved}`;
    });
  const lines: string[] = [];
  for (const element of doc.querySelectorAll("*")) {
    const computed = window.getComputedStyle(element);
    const declarations: string[] = [];
    for (let i = 0; i < computed.length; i++) {
      const property = computed.item(i);
      // A variable is compared where a declaration uses it, resolved: a
      // variable no before-send element uses is not a value the popup shows.
      if (property.startsWith("--")) {
        continue;
      }
      declarations.push(
        `${property}: ${resolve(computed.getPropertyValue(property))}`,
      );
    }
    declarations.sort();
    for (const declaration of declarations) {
      lines.push(`${elementPath(element)} ${declaration}`);
    }
  }
  return lines;
}

/** Writes HEAD's preview pages into a temporary directory and returns them by file name. */
function previewPages(head: string, extensionDir: string): Map<string, string> {
  const dir = mkdtempSync(path.join(tmpdir(), "presend-popup-styles-"));
  workDir = dir;
  const archive = execFileSync("git", [
    "archive",
    head,
    "extension/src",
    "extension/static",
    "extension/scripts",
    "extension/package.json",
  ]);
  execFileSync("tar", ["-x", "-C", dir], { input: archive });
  const extension = path.join(dir, "extension");
  symlinkSync(
    path.join(extensionDir, "node_modules"),
    path.join(extension, "node_modules"),
  );
  const output = path.join(dir, "pages");
  const result = spawnSync(
    process.execPath,
    ["scripts/popup-preview.ts", output],
    { cwd: extension, encoding: "utf8" },
  );
  if (result.status !== 0) {
    fail(`popup-preview.ts failed at ${head}:\n${result.stderr}`);
  }
  const pages = new Map<string, string>();
  for (const file of readdirSync(output).sort()) {
    if (file.endsWith(".html")) {
      pages.set(file, readFileSync(path.join(output, file), "utf8"));
    }
  }
  rmSync(dir, { recursive: true, force: true });
  workDir = "";
  return pages;
}

function check(): void {
  const root = git("rev-parse", "--show-toplevel");
  process.chdir(root);
  const extensionDir = path.join(root, "extension");
  const jsdom = createRequire(path.join(extensionDir, "package.json"))(
    "jsdom",
  ) as JsdomModule;
  const head = git("rev-parse", "HEAD");
  const base = git("merge-base", "origin/main", "HEAD");
  process.stdout.write(`merge-base ${base}\nhead       ${head}\n`);

  const before = show(base, "extension/static/style.css");
  const after = [
    show(head, "extension/static/theme.css"),
    show(head, "extension/static/style.css"),
  ].join("\n");

  const pages = previewPages(head, extensionDir);
  if (pages.size === 0) {
    fail("popup-preview.ts wrote no page");
  }

  const differing: string[] = [];
  let compared = 0;
  for (const [file, page] of pages) {
    for (const variant of variants) {
      const beforeLines = computedLines(jsdom, page, before, variant);
      const afterLines = computedLines(jsdom, page, after, variant);
      compared += beforeLines.length;
      const afterSet = new Set(afterLines);
      const beforeSet = new Set(beforeLines);
      for (const line of beforeLines) {
        if (!afterSet.has(line)) {
          differing.push(
            `${file} (${variantName(variant)}) merge-base only: ${line}`,
          );
        }
      }
      for (const line of afterLines) {
        if (!beforeSet.has(line)) {
          differing.push(
            `${file} (${variantName(variant)}) head only: ${line}`,
          );
        }
      }
    }
  }

  if (differing.length > 0) {
    process.stdout.write(
      `FAIL: ${differing.length} computed declarations differ\n${differing.slice(0, 50).join("\n")}\n`,
    );
    process.exit(1);
  }
  process.stdout.write(
    `PASS: ${compared} computed declarations identical across ${pages.size} pages x ${variants.length} variants\n`,
  );
  process.exit(0);
}

try {
  check();
} catch (error) {
  fail(error instanceof Error ? error.message : String(error));
}
