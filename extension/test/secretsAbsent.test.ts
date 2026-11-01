import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import ts from "typescript";

import { extensionDir } from "./helpers/paths.ts";

// The extension must not hold an API key or a Webhook URL: these words would
// name such a setting, input field or storage key. Matched case-insensitively.
// The word "webhook" alone is allowed: the job API's member, reasons and the
// messages about posting name the Webhook without naming its URL.
const secretWords = [
  "api key",
  "apikey",
  "api_key",
  "DEEPSEEK_API_KEY",
  "ANTHROPIC_API_KEY",
  "SLACK_WEBHOOK_URL",
];

// "webhook" and "url" joined by nothing or by one separator a setting name or
// label uses: "webhookUrl", "webhook_url", "webhook-url", "webhook.url" and
// "Webhook URL". Matched case-insensitively.
const webhookUrlPattern = /webhook[ _.-]?url/i;

// The server's environment variable a message may tell the user to set. It is
// allowed only inside the string literals of the message files, so the same
// name as code (an identifier, a comment, a property name) is still refused.
const messageEnvName = "SLACK_WEBHOOK_URL";
const messageFiles = ["src/core/i18n/en.ts", "src/core/i18n/ja.ts"];

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

/**
 * True when node is a string literal or template text used as a value: not
 * the name of a declaration or member, a computed or accessed key, or a
 * literal type.
 */
function isMessageText(node: ts.Node): boolean {
  const isText =
    ts.isStringLiteral(node) ||
    ts.isNoSubstitutionTemplateLiteral(node) ||
    ts.isTemplateHead(node) ||
    ts.isTemplateMiddle(node) ||
    ts.isTemplateTail(node);
  if (!isText) {
    return false;
  }
  const parent = node.parent;
  const named = (parent as { readonly name?: ts.Node }).name === node;
  return (
    !named &&
    !ts.isLiteralTypeNode(parent) &&
    !ts.isComputedPropertyName(parent) &&
    !ts.isElementAccessExpression(parent)
  );
}

/** text with messageEnvName removed from its string literals only. */
function withoutLiteralEnvName(file: string, text: string): string {
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true);
  let result = "";
  let last = 0;
  const visit = (node: ts.Node): void => {
    if (isMessageText(node)) {
      const start = node.getStart(source);
      const end = node.getEnd();
      result +=
        text.slice(last, start) +
        text.slice(start, end).replaceAll(messageEnvName, "");
      last = end;
      return;
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return result + text.slice(last);
}

/**
 * The secret words in text, the content of file (a "/"-separated path
 * relative to the extension directory). A message file is scanned after the
 * environment name is taken out of its string literals.
 */
function secretWordsIn(file: string, text: string): string[] {
  const scanned = messageFiles.includes(file)
    ? withoutLiteralEnvName(file, text)
    : text;
  const found = wordsIn(scanned, secretWords);
  const url = webhookUrlPattern.exec(scanned);
  return url === null ? found : [...found, url[0]];
}

/** The words that text contains, compared case-insensitively. */
function wordsIn(text: string, words: readonly string[]): string[] {
  const lower = text.toLowerCase();
  return words.filter((word) => lower.includes(word.toLowerCase()));
}

/** Asserts no file under src/ or static/ holds any word found by find. */
function expectAbsent(find: (file: string, text: string) => string[]): void {
  const files = [
    ...listFiles(path.join(extensionDir, "src")),
    ...listFiles(path.join(extensionDir, "static")),
  ];
  assert.notEqual(files.length, 0, "found no extension files to scan");
  for (const file of files) {
    const relative = path
      .relative(extensionDir, file)
      .split(path.sep)
      .join("/");
    const found = find(relative, readFileSync(file, "utf8"));
    assert.deepEqual(found, [], `${relative} contains ${found.join(", ")}`);
  }
}

describe("no secret settings", () => {
  it("names no API key or Webhook URL setting in src/ or static/", () => {
    expectAbsent(secretWordsIn);
  });

  it("names no token or port setting in src/ or static/", () => {
    expectAbsent((_file, text) => wordsIn(text, tokenAndPortWords));
  });
});

describe("secretWordsIn", () => {
  const refused: { name: string; file: string; text: string }[] = [
    {
      name: "a webhook url label",
      file: "static/popup.html",
      text: "<label>Webhook URL</label>",
    },
    {
      name: "a webhookUrl setting",
      file: "src/sample.ts",
      text: "const webhookUrl = '';",
    },
    {
      name: "a webhook-url input id",
      file: "static/popup.html",
      text: '<input id="webhook-url">',
    },
    {
      name: "a webhook.url path",
      file: "src/sample.ts",
      text: "const path = 'settings.webhook.url';",
    },
    {
      name: "a webhook_url storage key",
      file: "src/sample.ts",
      text: "const key = 'webhook_url';",
    },
    {
      name: "the environment name in a literal outside the message files",
      file: "src/core/response.ts",
      text: `const name = "${messageEnvName}";`,
    },
    {
      name: "the environment name as an identifier in a message file",
      file: "src/core/i18n/en.ts",
      text: `const ${messageEnvName} = "x";`,
    },
    {
      name: "the environment name in a comment in a message file",
      file: "src/core/i18n/ja.ts",
      text: `// ${messageEnvName}\nconst m = "x";`,
    },
    {
      name: "the environment name as a property name in a message file",
      file: "src/core/i18n/en.ts",
      text: `const m = { "${messageEnvName}": "x" };`,
    },
    {
      name: "the environment name as an interface key in a message file",
      file: "src/core/i18n/en.ts",
      text: `interface M { "${messageEnvName}": string }`,
    },
    {
      name: "the environment name as an accessed key in a message file",
      file: "src/core/i18n/ja.ts",
      text: `const v = m["${messageEnvName}"];`,
    },
    {
      name: "the environment name as a computed key in a message file",
      file: "src/core/i18n/ja.ts",
      text: `const m = { ["${messageEnvName}"]: "x" };`,
    },
    {
      name: "the environment name as a literal type in a message file",
      file: "src/core/i18n/en.ts",
      text: `type K = "${messageEnvName}";`,
    },
    {
      name: "the environment name as an enum member in a message file",
      file: "src/core/i18n/en.ts",
      text: `enum E { "${messageEnvName}" = 1 }`,
    },
  ];
  for (const { name, file, text } of refused) {
    it(`refuses ${name}`, () => {
      assert.notDeepEqual(secretWordsIn(file, text), []);
    });
  }

  const allowed: { name: string; file: string; text: string }[] = [
    {
      name: "the job API's Webhook names",
      file: "src/core/response.ts",
      text: 'const m = { webhookConfigured: true, reason: "webhook_not_configured", publish: "webhook" };',
    },
    {
      name: "the environment name in a message string",
      file: "src/core/i18n/en.ts",
      text: `const m = { setUp: "Set ${messageEnvName} and restart." };`,
    },
    {
      name: "the environment name in a message's steps",
      file: "src/core/i18n/en.ts",
      text: `const m = { steps: ["Set ${messageEnvName}."] };`,
    },
    {
      name: "the environment name in message template text",
      file: "src/core/i18n/ja.ts",
      text: `const f = (n: number): string => \`${messageEnvName} \${n} ${messageEnvName}\`;`,
    },
  ];
  for (const { name, file, text } of allowed) {
    it(`allows ${name}`, () => {
      assert.deepEqual(secretWordsIn(file, text), []);
    });
  }
});
