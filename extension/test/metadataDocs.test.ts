import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { expandPromptMessage } from "../src/core/messages.ts";
import {
  browserExtensionHeading,
  readmeVersions,
} from "./helpers/readmeVersions.ts";
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
  // Each version of the README quotes the popup in its own language: README.md
  // the Japanese catalog, README.en.md the English one.
  for (const { path: readmePath, locale } of readmeVersions) {
    const heading = browserExtensionHeading[locale];
    const section = flatSection(readmePath, heading);
    const where = `${readmePath} ${heading}`;
    const catalog = catalogs[locale];

    it(`${readmePath} gives the expand prompt word for word`, () => {
      // The README quotes the popup's prompt so a user can match what they
      // see; the wording is the contract, so it is compared as text.
      expectPhrases(section, where, [
        expandPromptMessage(locale).text,
        ...expandPromptMessage(locale).steps,
      ]);
    });

    it(`${readmePath} quotes the popup names the catalog defines`, () => {
      // The README shows each name the screen displays in bold; compare those
      // quoted names with the catalog. The explanatory prose is checked by
      // reading it in the PR review, not by matching words here.
      expectPhrases(section, where, [
        `**${catalog.primaryButton}**`,
        `**${catalog.checkNames.selection}**`,
        `**${catalog.checkNames.description}**`,
        `**${catalog.details.summary}**`,
      ]);
    });
  }
});
