import { readFileSync } from "node:fs";
import path from "node:path";

import { repoRoot } from "./paths.ts";

/** Reads a repository-relative file as UTF-8. */
export function readRepoFile(file: string): string {
  return readFileSync(path.join(repoRoot, file), "utf8");
}

/** The body of a Markdown section, from its heading to the next ### heading. */
export function markdownSection(markdown: string, heading: string): string {
  const start = markdown.indexOf(heading);
  if (start < 0) {
    return "";
  }
  const rest = markdown.slice(start + heading.length);
  const next = rest.indexOf("\n### ");
  return next < 0 ? rest : rest.slice(0, next);
}
