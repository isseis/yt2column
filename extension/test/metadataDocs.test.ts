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
  const readme = flatSection("README.md", "### Browser extension");
  const readmeWhere = "README's Browser extension section";

  it("README gives the expand prompt word for word", () => {
    // The README quotes the popup's prompt so a user can match what they see;
    // the wording is the contract, so it is compared as text.
    expectPhrases(readme, readmeWhere, [
      expandPromptMessage.text,
      ...expandPromptMessage.steps,
    ]);
  });
});
