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
  // Only the Japanese README is compared with the catalog: no English document
  // is checked against the English catalog until the English install guide exists.
  const japaneseReadmes = readmeVersions.filter(
    ({ locale }) => locale === "ja",
  );
  for (const { path: readmePath, locale } of japaneseReadmes) {
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

    it(`${readmePath} names the generation steps as the extension shows them`, () => {
      expectPhrases(section, where, Object.values(catalog.stepNames));
    });

    it(`${readmePath} quotes the popup names the catalog defines`, () => {
      // The README shows each name the screen displays in bold; compare those
      // quoted names with the catalog. The explanatory prose is checked by
      // reading it in the PR review, not by matching words here.
      expectPhrases(section, where, [
        ...Object.values(catalog.popup.primary).map((name) => `**${name}**`),
        `**${catalog.popup.secondary}**`,
        `**${catalog.viewer.save}**`,
        `**${catalog.viewer.copy}**`,
        `**${catalog.checkNames.selection}**`,
        `**${catalog.checkNames.description}**`,
        `**${catalog.details.summary}**`,
      ]);
    });
  }
});
