import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
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
      expandPromptMessage("en").text,
      ...expandPromptMessage("en").steps,
    ]);
  });

  it("README names the parts of the new popup the catalog defines", () => {
    // The README names the screen's controls so a user can find them. The
    // row names are matched with the word "row" so a stray occurrence of the
    // bare word cannot stand in for the documented row.
    const plain = readme.replaceAll("**", "");
    expectPhrases(plain, readmeWhere, [
      catalogs.en.primaryButton,
      `${catalogs.en.checkNames.selection} row`,
      `${catalogs.en.checkNames.description} row`,
      catalogs.en.details.summary,
    ]);
  });
});
