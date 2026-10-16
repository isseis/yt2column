import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { extensionDir } from "./helpers/paths.ts";

// The extension must not hold an API key or a Webhook URL: these words would
// name such a setting, input field or storage key. Matched case-insensitively.
const forbidden = [
  "webhook",
  "api key",
  "apikey",
  "api_key",
  "DEEPSEEK_API_KEY",
  "ANTHROPIC_API_KEY",
  "SLACK_WEBHOOK_URL",
];

/** Every regular file under dir, as absolute paths. */
function listFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(dir, {
    recursive: true,
    withFileTypes: true,
  })) {
    if (entry.isFile()) {
      files.push(path.join(entry.parentPath, entry.name));
    }
  }
  return files.sort();
}

describe("no API key or Webhook settings", () => {
  it("names no secret setting in src/ or static/", () => {
    const files = [
      ...listFiles(path.join(extensionDir, "src")),
      ...listFiles(path.join(extensionDir, "static")),
    ];
    assert.notEqual(files.length, 0, "found no extension files to scan");
    for (const file of files) {
      const text = readFileSync(file, "utf8").toLowerCase();
      for (const word of forbidden) {
        assert.equal(
          text.includes(word.toLowerCase()),
          false,
          `${path.relative(extensionDir, file)} contains ${word}`,
        );
      }
    }
  });
});
