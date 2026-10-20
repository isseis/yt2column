import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { extensionDir } from "./helpers/paths.ts";

// The extension must not hold an API key or a Webhook URL: these words would
// name such a setting, input field or storage key. Matched case-insensitively.
const secretWords = [
  "webhook",
  "api key",
  "apikey",
  "api_key",
  "DEEPSEEK_API_KEY",
  "ANTHROPIC_API_KEY",
  "SLACK_WEBHOOK_URL",
];

// The extension sends to the native messaging host and has no token or port
// to set, store or show: these words would name such a setting, its storage
// key, or the loopback address it was for. No word is exempt, not even to
// delete what an earlier version stored. Matched case-insensitively.
const tokenAndPortWords = [
  "token",
  "serverPort",
  "server port",
  "YT2COLUMN_SERVER_PORT",
  "Authorization",
  "127.0.0.1",
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

/** Asserts no file under src/ or static/ contains any of words. */
function expectAbsent(words: readonly string[]): void {
  const files = [
    ...listFiles(path.join(extensionDir, "src")),
    ...listFiles(path.join(extensionDir, "static")),
  ];
  assert.notEqual(files.length, 0, "found no extension files to scan");
  for (const file of files) {
    const text = readFileSync(file, "utf8").toLowerCase();
    for (const word of words) {
      assert.equal(
        text.includes(word.toLowerCase()),
        false,
        `${path.relative(extensionDir, file)} contains ${word}`,
      );
    }
  }
}

describe("no secret settings", () => {
  it("names no API key or Webhook setting in src/ or static/", () => {
    expectAbsent(secretWords);
  });

  it("names no token or port setting in src/ or static/", () => {
    expectAbsent(tokenAndPortWords);
  });
});
