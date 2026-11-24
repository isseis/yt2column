import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { expandPromptMessage } from "../src/core/messages.ts";
import { installGuide } from "./helpers/installGuide.ts";
import { readRepoFile } from "./helpers/repoFile.ts";

/** The document's text with every whitespace run, line breaks included, as one space. */
function flatDoc(file: string): string {
  return readRepoFile(file).replace(/\s+/gu, " ");
}

/** Asserts the document holds each phrase, naming the first missing one. */
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

describe("install guide quotes the extension's Japanese catalog", () => {
  // The install guide is Japanese only, so it is compared with the Japanese
  // catalog. No English document is compared with the English catalog until
  // the English install guide exists. The whole document is read, not one
  // section, because the names and messages appear in several sections.
  const { path: guidePath, locale } = installGuide;
  const document = flatDoc(guidePath);
  const catalog = catalogs[locale];

  it("gives the expand prompt word for word", () => {
    // The guide quotes the popup's prompt so a user can match what they see;
    // the wording is the contract, so it is compared as text.
    expectPhrases(document, guidePath, [
      expandPromptMessage(locale).text,
      ...expandPromptMessage(locale).steps,
    ]);
  });

  it("names the generation steps as the extension shows them", () => {
    expectPhrases(document, guidePath, Object.values(catalog.stepNames));
  });

  it("quotes the popup and viewer names the catalog defines", () => {
    // The guide shows each name the screen displays in bold; compare those
    // quoted names with the catalog. The explanatory prose is checked by
    // reading it in the PR review, not by matching words here.
    expectPhrases(document, guidePath, [
      ...Object.values(catalog.popup.primary).map((name) => `**${name}**`),
      `**${catalog.popup.secondary}**`,
      `**${catalog.viewer.save}**`,
      `**${catalog.viewer.copy}**`,
      `**${catalog.checkNames.selection}**`,
      `**${catalog.checkNames.description}**`,
      `**${catalog.details.summary}**`,
    ]);
  });

  it("quotes the troubleshooting messages the catalog defines", () => {
    // Only catalog entries that are fixed strings are chosen; the type of
    // Message makes a chosen entry that is not a string fail to compile.
    // The collapsed-description message is checked by the expand prompt test.
    expectPhrases(document, guidePath, [
      catalog.keyRejected.text,
      catalog.failure.llm_failed.text,
      catalog.setup.api_key_missing.text,
      catalog.rejection["empty-selection"].text,
      catalog.startRejected.invalid_body.text,
      catalog.stopped.text,
    ]);
  });
});
