import { readFileSync } from "node:fs";
import path from "node:path";

import { repoRoot } from "./paths.ts";

/** Reads a repository-relative file as UTF-8. */
export function readRepoFile(file: string): string {
  return readFileSync(path.join(repoRoot, file), "utf8");
}
