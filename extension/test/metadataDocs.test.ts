import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { expandPromptMessage } from "../src/core/messages.ts";
import { markdownSection, readRepoFile } from "./helpers/repoFile.ts";

/** The section's text with every whitespace run, line breaks included, as one space. */
function flatSection(file: string, heading: string): string {
  return markdownSection(readRepoFile(file), heading).replace(/\s+/gu, " ");
}

/** Asserts the section holds each phrase, naming the first missing one. */
function expectPhrases(
  text: string,
  where: string,
  phrases: readonly string[],
): void {
  assert.notEqual(text.trim(), "", `${where} is missing`);
  for (const phrase of phrases) {
    assert.ok(text.includes(phrase), `${where} does not say: ${phrase}`);
  }
}

describe("video metadata documentation", () => {
  it("README says the extension sends the video metadata", () => {
    expectPhrases(
      flatSection("README.md", "### Browser extension"),
      "README's Browser extension section",
      [
        "reads the video metadata — the video title, the channel name, and the description — from the video page",
        "the URL, and the video metadata to the localhost server",
        "Some values may not be available",
        "that value is left out of the request",
        "The extension still sends the selection and the URL",
      ],
    );
  });

  it("README gives the expand prompt word for word", () => {
    // The README quotes the popup's prompt so a user can match what they see;
    // the wording is the contract, so it is compared as text.
    expectPhrases(
      flatSection("README.md", "### Browser extension"),
      "README's Browser extension section",
      [expandPromptMessage.text, ...expandPromptMessage.steps],
    );
  });

  it("security.md 8.1 records how the video metadata is read and handled", () => {
    expectPhrases(
      flatSection("docs/dev/security.md", "### 8.1. ブラウザ拡張（#110）"),
      "security.md 8.1",
      [
        "動画ページの DOM から読む",
        "新しい権限を加えず",
        "ページを操作するコードを持たない",
        "取得に失敗しても起動を失敗させず",
        "untrusted テキストとして扱う",
        "HTML として解釈せず",
        "本文へは JSON の文字列として入れる",
      ],
    );
  });
});
