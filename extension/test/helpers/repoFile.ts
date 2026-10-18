import { readFileSync } from "node:fs";
import path from "node:path";

import { repoRoot } from "./paths.ts";

/** Reads a repository-relative file as UTF-8. */
export function readRepoFile(file: string): string {
  return readFileSync(path.join(repoRoot, file), "utf8");
}

/**
 * The body of a level-3 Markdown section, from its heading line to the next
 * heading of level 3 or higher; "" when no line is exactly the heading.
 */
export function markdownSection(markdown: string, heading: string): string {
  const text = `\n${markdown}`;
  const marker = `\n${heading}\n`;
  const start = text.indexOf(marker);
  if (start < 0) {
    return "";
  }
  const rest = text.slice(start + marker.length);
  const next = rest.search(/^#{1,3} /mu);
  return next < 0 ? rest : rest.slice(0, next);
}
