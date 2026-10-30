import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { expandPromptMessage } from "../src/core/messages.ts";
import {
  browserExtensionHeading,
  readmeVersions,
} from "./helpers/readmeVersions.ts";
import { repoRoot } from "./helpers/paths.ts";
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
  for (const { path, locale } of readmeVersions) {
    const heading = browserExtensionHeading[locale];
    const section = flatSection(path, heading);
    const where = `${path} ${heading}`;
    const catalog = catalogs[locale];

    it(`${path} gives the expand prompt word for word`, () => {
      // The README quotes the popup's prompt so a user can match what they
      // see; the wording is the contract, so it is compared as text.
      expectPhrases(section, where, [
        expandPromptMessage(locale).text,
        ...expandPromptMessage(locale).steps,
      ]);
    });

    it(`${path} quotes the popup names the catalog defines`, () => {
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

describe("socket directory documentation reference", () => {
  it("the English message points at README.en.md, which exists", () => {
    // The English popup sends the user to the English README for the socket
    // directory's permissions; the Japanese one keeps pointing at README.md.
    const steps = catalogs.en.notSent["server-unavailable"].steps.join(" ");
    assert.ok(
      steps.includes("README.en.md"),
      `the English server-unavailable steps do not name README.en.md: ${steps}`,
    );
    assert.ok(existsSync(path.join(repoRoot, "README.en.md")));
  });
});
