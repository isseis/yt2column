// What the release build and the development build differ in: the output
// directory, the article generation module's main package, the manifest, and
// the build kind module. The build (scripts/build.ts) writes these and the
// output check (scripts/check-dist.ts) expects them, from this one place.
import type { BuildKind } from "../src/buildKind.d.ts";

export type { BuildKind };

/** The article generation module's file name in the output directory. */
export const wasmFileName = "yt2column.wasm";

/** The Go toolchain's JavaScript support file, copied next to the module. */
export const wasmExecFileName = "wasm_exec.js";

/** The build kind module's file name, in static/ and the output directory. */
export const buildKindFileName = "buildKind.js";

/** The catalog message that names the development build in its manifest. */
const devNameMessage = "__MSG_extNameDev__";

/** The catalog message that names the release build in static/manifest.json. */
const releaseNameMessage = "__MSG_extName__";

/** Returns the kind a command-line word names; any other word is rejected. */
export function parseBuildKind(text: string): BuildKind {
  switch (text) {
    case "release":
    case "dev":
      return text;
    default:
      throw new Error(`unknown build kind "${text}"; want release or dev`);
  }
}

/** The output directory of a kind, relative to the extension directory. */
export function outputDir(kind: BuildKind): string {
  switch (kind) {
    case "release":
      return "dist";
    case "dev":
      return "dist-dev";
  }
}

/** The article generation module's main package, relative to the module root. */
export function wasmMainPackage(kind: BuildKind): string {
  switch (kind) {
    case "release":
      return "cmd/yt2column-wasm";
    case "dev":
      return "cmd/yt2column-wasm-dev";
  }
}

/**
 * The text of the build kind module for kind. static/buildKind.js holds the
 * release text, so the release build copies it unchanged.
 */
export function buildKindSource(kind: BuildKind): string {
  return [
    "// The kind of this build. The development build replaces this file.",
    `export const buildKind = "${kind}";`,
    "",
  ].join("\n");
}

/**
 * The development build's manifest, from the text of static/manifest.json:
 * the same members without "key", so the development build gets an extension
 * ID (and a chrome.storage.local) of its own, and named by the catalog
 * message that marks it as the development build. It rejects a manifest
 * without a key or named otherwise than the release manifest is, rather than
 * writing a manifest that would pass for the release one.
 */
export function devManifest(text: string): string {
  const parsed: unknown = JSON.parse(text);
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    throw new Error("manifest.json is not a JSON object");
  }
  const manifest = { ...(parsed as Record<string, unknown>) };
  if (typeof manifest["key"] !== "string") {
    throw new Error('manifest.json has no "key" to remove');
  }
  if (manifest["name"] !== releaseNameMessage) {
    throw new Error(`manifest.json "name" is not ${releaseNameMessage}`);
  }
  delete manifest["key"];
  manifest["name"] = devNameMessage;
  return `${JSON.stringify(manifest, null, 2)}\n`;
}
