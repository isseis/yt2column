// The type of static/buildKind.js, the module that declares the build's kind.
// The release build copies it as it is; the development build replaces it with
// one that declares "dev" (scripts/build-variant.ts). Only the entry page
// scripts import it, and they pass the value on as a BuildKind.

/** The build's kind, declared by the module the build writes. */
export type BuildKind = "release" | "dev";

export declare const buildKind: BuildKind;
