import path from "node:path";
import { fileURLToPath } from "node:url";

/** Absolute path of extension/, independent of the working directory. */
export const extensionDir = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
);

/** Absolute path of the repository root. */
export const repoRoot = path.dirname(extensionDir);
