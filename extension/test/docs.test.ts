import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { extensionIdFromManifestKey } from "./helpers/extensionId.ts";
import { repoRoot } from "./helpers/paths.ts";

/** Repository-relative paths of the documents this guard checks. */
const readme = "README.md";
const claude = "CLAUDE.md";
const projectOverview = "docs/dev/project_overview.md";
const security = "docs/dev/security.md";
const plan =
  "docs/tasks/0007_browser_extension_skeleton/03_implementation_plan.md";
const architecture =
  "docs/tasks/0007_browser_extension_skeleton/02_architecture.md";

/** Reads a repository-relative file as UTF-8. */
function readDoc(file: string): string {
  return readFileSync(path.join(repoRoot, file), "utf8");
}

/**
 * The text under the first heading whose trimmed line equals heading, up to the
 * next heading of the same or a higher level. Empty when no line matches.
 */
function sectionByHeading(doc: string, heading: string): string {
  const lines = doc.split("\n");
  const start = lines.findIndex((line) => line.trim() === heading);
  if (start < 0) {
    return "";
  }
  const level = lines[start]?.match(/^#+/)?.[0].length ?? 0;
  const body: string[] = [];
  for (const line of lines.slice(start + 1)) {
    const next = /^(#+)\s/.exec(line);
    if (next && (next[1]?.length ?? 0) <= level) {
      break;
    }
    body.push(line);
  }
  return body.join("\n");
}

/** The text between the first start line and the following end line. */
function sliceBetween(doc: string, start: string, end: string): string {
  const from = doc.indexOf(start);
  if (from < 0) {
    return "";
  }
  const to = doc.indexOf(end, from + start.length);
  return to < 0 ? doc.slice(from) : doc.slice(from, to);
}

/** Every Markdown table row in text, as trimmed cells; separators dropped. */
function tableRows(text: string): string[][] {
  const rows: string[][] = [];
  for (const line of text.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed.startsWith("|") || !trimmed.endsWith("|")) {
      continue;
    }
    const cells = trimmed
      .slice(1, -1)
      .split("|")
      .map((cell) => cell.trim());
    if (cells.length > 0 && cells.every((cell) => /^:?-+:?$/.test(cell))) {
      continue;
    }
    rows.push(cells);
  }
  return rows;
}

const manifest = JSON.parse(readDoc("extension/static/manifest.json")) as {
  key: string;
  permissions: string[];
};
const extensionId = extensionIdFromManifestKey(manifest.key);

describe("documents", () => {
  it("records the fixed extension ID in README and security.md", () => {
    for (const file of [readme, security]) {
      assert.ok(
        readDoc(file).includes(extensionId),
        `${file} does not record ${extensionId}`,
      );
    }
  });

  it("documents every ext- target the Makefile defines", () => {
    const targets = [...readDoc("Makefile").matchAll(/^(ext-[a-z0-9-]+):/gm)]
      .map((match) => match[1])
      .filter((target): target is string => target !== undefined);
    assert.ok(targets.length > 0, "the Makefile defines no ext- targets");
    const text = readDoc(claude);
    for (const target of new Set(targets)) {
      assert.ok(text.includes(target), `CLAUDE.md does not mention ${target}`);
    }
  });

  it("lists every manifest permission in the extension section of security.md", () => {
    const section = sectionByHeading(
      readDoc(security),
      "### 8.1. ブラウザ拡張（#110）",
    );
    assert.notEqual(section, "", "security.md has no 8.1 extension section");
    for (const permission of manifest.permissions) {
      assert.ok(
        section.includes(permission),
        `security.md 8.1 does not mention ${permission}`,
      );
    }
  });

  it("adds TypeScript and extension/ to the project overview", () => {
    const overview = readDoc(projectOverview);
    const policies = sectionByHeading(overview, "## 決定済みの方針");
    assert.ok(
      policies.includes("TypeScript"),
      "決定済みの方針 does not mention TypeScript",
    );
    const layout = sectionByHeading(overview, "## 想定ディレクトリ構成");
    assert.ok(
      layout.includes("extension/"),
      "想定ディレクトリ構成 does not list extension/",
    );
  });
});

describe("investigation is recorded", () => {
  it("records sections 3.12.1 to 3.12.3 with the extension ID in 3.12.1", () => {
    const doc = readDoc(architecture);
    for (const section of ["3.12.1", "3.12.2", "3.12.3"]) {
      assert.ok(
        doc.includes(`#### ${section}.`),
        `the architecture has no section ${section}`,
      );
    }
    const first = sliceBetween(doc, "#### 3.12.1.", "#### 3.12.2.");
    assert.ok(
      first.includes(extensionId),
      "architecture 3.12.1 does not record the extension ID",
    );
  });
});

describe("manual checks are recorded", () => {
  it("fills the result of every row in the plan's manual-check table", () => {
    const section = sectionByHeading(
      readDoc(plan),
      "### 5.1. 手動の確認と記録",
    );
    assert.notEqual(section, "", "the plan has no section 5.1");
    const rows = tableRows(section).slice(1);
    assert.ok(rows.length > 0, "section 5.1 has no table rows");
    for (const row of rows) {
      const result = row.at(-1) ?? "";
      assert.notEqual(result, "", `section 5.1 row ${row[0] ?? ""} is empty`);
    }
  });
});
