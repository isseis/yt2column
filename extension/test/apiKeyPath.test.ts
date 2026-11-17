import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import ts from "typescript";

import { extensionDir } from "./helpers/paths.ts";

// The API key reaches the extension's code only through the fixed functions of
// browser/chromeDeps.ts: the options page writes it, and the running viewer
// reads it. This test checks the source for the paths the lint rules cannot
// see: which chrome.storage calls are made and with which arguments, which
// change events are listened to, and what is logged. Like the lint rules, it
// guards against accidental use, not deliberate evasion.

/** The one file that may reference chrome.storage. */
const chromeDepsFile = "src/browser/chromeDeps.ts";

/** The two items the options page writes and the running viewer reads. */
const settingsItems = ["anthropic.apiKey", "anthropic.workspaceId"] as const;

/**
 * The chrome.storage.local methods the extension calls, and the one function
 * of chromeDepsFile each may be called in. No other method of the area (clear,
 * onChanged) and no other use of the area object is accepted.
 */
const localCalls: Readonly<Record<string, string>> = {
  get: "readAnthropicSettings",
  set: "writeAnthropicSettings",
  remove: "removeAnthropicSettings",
  getKeys: "localKeys",
  setAccessLevel: "setLocalAccessLevel",
};

/** The chrome.storage areas the extension uses. sync is not one of them. */
const areas = new Set(["local", "session"]);

/** Method names that write a log line: console's and the Logger's. */
const logMethods = new Set(["log", "info", "warn", "error", "debug"]);

/** An identifier that would hold the API key's value: apiKey, anthropicApiKey, api_key. */
const apiKeyName = /api_?key$/i;

/** The API key's item name in chrome.storage.local and its member name in the module's request. */
const apiKeyStrings = new Set(["anthropic.apiKey", "anthropicApiKey"]);

/** The name of a member access in the dotted form; undefined for the other forms. */
function dottedName(node: ts.Node): string | undefined {
  return ts.isPropertyAccessExpression(node) ? node.name.text : undefined;
}

/**
 * The member name an access refers to, in the dotted form and in the computed
 * form with a string or template literal; undefined for any other node.
 */
function memberName(node: ts.Node): string | undefined {
  if (ts.isPropertyAccessExpression(node)) {
    return node.name.text;
  }
  if (
    ts.isElementAccessExpression(node) &&
    (ts.isStringLiteral(node.argumentExpression) ||
      ts.isNoSubstitutionTemplateLiteral(node.argumentExpression))
  ) {
    return node.argumentExpression.text;
  }
  return undefined;
}

/** The name a destructuring element takes from its object, or undefined. */
function bindingName(node: ts.Node): string | undefined {
  if (!ts.isBindingElement(node) || !ts.isObjectBindingPattern(node.parent)) {
    return undefined;
  }
  const property = node.propertyName ?? node.name;
  return ts.isIdentifier(property) ||
    ts.isStringLiteral(property) ||
    ts.isNoSubstitutionTemplateLiteral(property)
    ? property.text
    : undefined;
}

/** The text of a chain of identifiers joined by dots, such as chrome.storage.local; undefined otherwise. */
function chainText(node: ts.Node): string | undefined {
  if (ts.isIdentifier(node)) {
    return node.text;
  }
  if (ts.isPropertyAccessExpression(node)) {
    const object = chainText(node.expression);
    return object === undefined ? undefined : `${object}.${node.name.text}`;
  }
  return undefined;
}

/** The name of the nearest named function around node, or undefined at the top level. */
function enclosingFunction(node: ts.Node): string | undefined {
  for (let current = node.parent; current !== undefined;) {
    if (ts.isFunctionDeclaration(current) && current.name !== undefined) {
      return current.name.text;
    }
    if (
      (ts.isArrowFunction(current) || ts.isFunctionExpression(current)) &&
      ts.isVariableDeclaration(current.parent) &&
      ts.isIdentifier(current.parent.name)
    ) {
      return current.parent.name.text;
    }
    current = current.parent;
  }
  return undefined;
}

/** True when node is an array literal of exactly the two settings items, in order. */
function isSettingsItemList(node: ts.Node): boolean {
  return (
    ts.isArrayLiteralExpression(node) &&
    node.elements.length === settingsItems.length &&
    node.elements.every(
      (element, i) =>
        ts.isStringLiteral(element) && element.text === settingsItems[i],
    )
  );
}

/** True when node is an object literal of exactly the two settings items as string-literal keys. */
function isSettingsObject(node: ts.Node): boolean {
  if (
    !ts.isObjectLiteralExpression(node) ||
    node.properties.length !== settingsItems.length
  ) {
    return false;
  }
  const names = node.properties.map((property) =>
    ts.isPropertyAssignment(property) && ts.isStringLiteral(property.name)
      ? property.name.text
      : undefined,
  );
  return settingsItems.every((item) => names.includes(item));
}

/** Whether the arguments of a chrome.storage.local call are the ones its method allows. */
function acceptedArguments(
  method: string,
  args: ts.NodeArray<ts.Expression>,
): boolean {
  switch (method) {
    case "get":
    case "remove":
      return args.length === 1 && args[0] !== undefined
        ? isSettingsItemList(args[0])
        : false;
    case "set":
      return args.length === 1 && args[0] !== undefined
        ? isSettingsObject(args[0])
        : false;
    case "getKeys":
      return args.length === 0;
    case "setAccessLevel":
      return args.length === 1;
    default:
      return false;
  }
}

/**
 * The violations of a use of chrome.storage.local (node is the area object):
 * it must be the object of a dotted call of one allowed method, in that
 * method's function, with the arguments the method allows.
 */
function localUseViolations(node: ts.Node): string[] {
  const access = node.parent;
  const method =
    access !== undefined && ts.isPropertyAccessExpression(access)
      ? dottedName(access)
      : undefined;
  const call = access?.parent;
  if (
    method === undefined ||
    !Object.hasOwn(localCalls, method) ||
    call === undefined ||
    !ts.isCallExpression(call) ||
    call.expression !== access
  ) {
    return [
      `chrome.storage.local is used other than by a call of ${Object.keys(localCalls).join(", ")}`,
    ];
  }
  const violations: string[] = [];
  const owner = localCalls[method];
  if (enclosingFunction(call) !== owner) {
    violations.push(
      `chrome.storage.local.${method} is called outside ${owner}`,
    );
  }
  if (!acceptedArguments(method, call.arguments)) {
    violations.push(
      `chrome.storage.local.${method} is called with other arguments than the fixed ones`,
    );
  }
  return violations;
}

/**
 * True when some identifier or member name under node ends with the API key's
 * name, or a string literal under node is the API key's item or member name.
 * A failure code such as "api_key_rejected" is a value, not the key, so it is
 * allowed.
 */
function namesApiKey(node: ts.Node): boolean {
  if (
    (ts.isIdentifier(node) || ts.isPrivateIdentifier(node)) &&
    apiKeyName.test(node.text)
  ) {
    return true;
  }
  if (
    (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) &&
    apiKeyStrings.has(node.text)
  ) {
    return true;
  }
  return ts.forEachChild(node, namesApiKey) === true;
}

/**
 * The API key path violations in text, the content of file (a "/"-separated
 * path relative to the extension directory).
 */
function apiKeyPathViolations(file: string, text: string): string[] {
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true);
  const inChromeDeps = file === chromeDepsFile;
  const violations: string[] = [];
  const visit = (node: ts.Node): void => {
    const name = memberName(node) ?? bindingName(node);
    if (name === "onChanged") {
      // Only the session area's change event: the local area's and the
      // storage-wide one carry the API key's value to every listener.
      if (
        !inChromeDeps ||
        chainText(node) !== "chrome.storage.session.onChanged"
      ) {
        violations.push(
          "onChanged is referenced other than as chrome.storage.session.onChanged in chromeDeps.ts",
        );
      }
    }
    if (name === "storage") {
      const parent = node.parent;
      const area =
        parent !== undefined && ts.isPropertyAccessExpression(parent)
          ? dottedName(parent)
          : undefined;
      if (
        !inChromeDeps ||
        chainText(node) !== "chrome.storage" ||
        area === undefined ||
        !areas.has(area)
      ) {
        violations.push(
          "storage is referenced other than as chrome.storage.local or chrome.storage.session in chromeDeps.ts",
        );
      } else if (area === "local" && parent !== undefined) {
        violations.push(...localUseViolations(parent));
      }
    }
    if (ts.isCallExpression(node)) {
      const method = memberName(node.expression);
      if (
        method !== undefined &&
        logMethods.has(method) &&
        node.arguments.some(namesApiKey)
      ) {
        violations.push(`a log call (${method}) is given the API key`);
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return violations;
}

/** The words in text, compared case-insensitively. */
function wordsIn(text: string, words: readonly string[]): string[] {
  const lower = text.toLowerCase();
  return words.filter((word) => lower.includes(word.toLowerCase()));
}

// Settings the extension does not have: the environment variable names that
// belong to the Go configuration, and a Webhook URL. Matched
// case-insensitively. The word "webhook" alone is allowed: it names no setting
// by itself, so only its pairing with "url" is refused.
const absentSettingWords = ["DEEPSEEK_API_KEY", "ANTHROPIC_API_KEY"];

// "webhook" and "url" joined by nothing or by one separator a setting name or
// label uses: "webhookUrl", "webhook_url", "webhook-url", "webhook.url" and
// "Webhook URL". Matched case-insensitively.
const webhookUrlPattern = /webhook[ _.-]?url/i;

// The values an earlier version stored and the server connection it was for:
// a token and a port. The extension neither reads nor deletes what an earlier
// version stored, so no word is exempt. Matched case-insensitively.
const tokenAndPortWords = [
  "token",
  "sharedToken",
  "serverPort",
  "server port",
  "YT2COLUMN_SERVER_PORT",
  "Authorization",
  "127.0.0.1",
];

/** The absent settings' words in text. */
function absentSettingsIn(text: string): string[] {
  const found = wordsIn(text, absentSettingWords);
  const url = webhookUrlPattern.exec(text);
  return url === null ? found : [...found, url[0]];
}

/** Every regular file under dir, as "/"-separated paths relative to the extension directory. */
function listFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(path.join(extensionDir, dir), {
    recursive: true,
    withFileTypes: true,
  })) {
    if (entry.isFile()) {
      files.push(
        path
          .relative(extensionDir, path.join(entry.parentPath, entry.name))
          .split(path.sep)
          .join("/"),
      );
    }
  }
  return files.sort();
}

/** Asserts find reports nothing for any of files. */
function expectNone(
  files: readonly string[],
  find: (file: string, text: string) => string[],
): void {
  assert.notEqual(files.length, 0, "found no extension files to scan");
  for (const file of files) {
    const found = find(
      file,
      readFileSync(path.join(extensionDir, file), "utf8"),
    );
    assert.deepEqual(found, [], `${file}: ${found.join("; ")}`);
  }
}

const sourceFiles = listFiles("src").filter((file) => file.endsWith(".ts"));

describe("the API key path in the extension's source", () => {
  it("has no API key path violation in src/", () => {
    expectNone(sourceFiles, apiKeyPathViolations);
  });

  it("names no absent setting in src/ or static/", () => {
    expectNone([...sourceFiles, ...listFiles("static")], (_file, text) =>
      absentSettingsIn(text),
    );
  });

  it("names no token or port in src/ or static/", () => {
    expectNone([...sourceFiles, ...listFiles("static")], (_file, text) =>
      wordsIn(text, tokenAndPortWords),
    );
  });
});

describe("apiKeyPathViolations", () => {
  const items = '["anthropic.apiKey", "anthropic.workspaceId"]';
  const settingsObject =
    '{ "anthropic.apiKey": k, "anthropic.workspaceId": w }';
  const inFunction = (name: string, body: string): string =>
    `export function ${name}(): unknown {\n  return ${body};\n}\n`;

  const allowed: { name: string; file: string; text: string }[] = [
    {
      name: "the fixed get",
      file: chromeDepsFile,
      text: inFunction(
        "readAnthropicSettings",
        `chrome.storage.local.get(${items})`,
      ),
    },
    {
      name: "the fixed set",
      file: chromeDepsFile,
      text: inFunction(
        "writeAnthropicSettings",
        `chrome.storage.local.set(${settingsObject})`,
      ),
    },
    {
      name: "the fixed remove",
      file: chromeDepsFile,
      text: inFunction(
        "removeAnthropicSettings",
        `chrome.storage.local.remove(${items})`,
      ),
    },
    {
      name: "getKeys",
      file: chromeDepsFile,
      text: inFunction("localKeys", "chrome.storage.local.getKeys()"),
    },
    {
      name: "setAccessLevel",
      file: chromeDepsFile,
      text: inFunction(
        "setLocalAccessLevel",
        'chrome.storage.local.setAccessLevel({ accessLevel: "TRUSTED_CONTEXTS" })',
      ),
    },
    {
      name: "a get in an arrow function bound to the owner's name",
      file: chromeDepsFile,
      text: `export const readAnthropicSettings = () => chrome.storage.local.get(${items});\n`,
    },
    {
      name: "the session area's get of every item",
      file: chromeDepsFile,
      text: "export const r = () => chrome.storage.session.get(null);\n",
    },
    {
      name: "the session area's change event",
      file: chromeDepsFile,
      text: "chrome.storage.session.onChanged.addListener(() => {});\n",
    },
    {
      name: "a log call without the key",
      file: "src/sample.ts",
      text: 'logger.info("viewer.run", { failure: "api_key_missing", apiKeyMissing: true });\n',
    },
  ];
  for (const { name, file, text } of allowed) {
    it(`allows ${name}`, () => {
      assert.deepEqual(apiKeyPathViolations(file, text), []);
    });
  }

  const refused: { name: string; file: string; text: string }[] = [
    {
      name: "a get of every item with null",
      file: chromeDepsFile,
      text: inFunction(
        "readAnthropicSettings",
        "chrome.storage.local.get(null)",
      ),
    },
    {
      name: "a get with no argument",
      file: chromeDepsFile,
      text: inFunction("readAnthropicSettings", "chrome.storage.local.get()"),
    },
    {
      name: "a get with undefined",
      file: chromeDepsFile,
      text: inFunction(
        "readAnthropicSettings",
        "chrome.storage.local.get(undefined)",
      ),
    },
    {
      name: "a get of one item",
      file: chromeDepsFile,
      text: inFunction(
        "readAnthropicSettings",
        'chrome.storage.local.get(["anthropic.apiKey"])',
      ),
    },
    {
      name: "a get of the items through a variable",
      file: chromeDepsFile,
      text: inFunction(
        "readAnthropicSettings",
        "chrome.storage.local.get(names)",
      ),
    },
    {
      name: "a get of a third item",
      file: chromeDepsFile,
      text: inFunction(
        "readAnthropicSettings",
        'chrome.storage.local.get(["anthropic.apiKey", "anthropic.workspaceId", "other"])',
      ),
    },
    {
      name: "the fixed get outside its function",
      file: chromeDepsFile,
      text: inFunction("readSomething", `chrome.storage.local.get(${items})`),
    },
    {
      name: "a remove of every item",
      file: chromeDepsFile,
      text: inFunction(
        "removeAnthropicSettings",
        "chrome.storage.local.remove(null)",
      ),
    },
    {
      name: "a set of another item",
      file: chromeDepsFile,
      text: inFunction(
        "writeAnthropicSettings",
        'chrome.storage.local.set({ "anthropic.apiKey": k, other: w })',
      ),
    },
    {
      name: "clear",
      file: chromeDepsFile,
      text: inFunction(
        "removeAnthropicSettings",
        "chrome.storage.local.clear()",
      ),
    },
    {
      name: "the local area passed on",
      file: chromeDepsFile,
      text: "export const area = chrome.storage.local;\n",
    },
    {
      name: "the local area's get passed on",
      file: chromeDepsFile,
      text: inFunction("readAnthropicSettings", "chrome.storage.local.get"),
    },
    {
      name: "a computed local area",
      file: chromeDepsFile,
      text: inFunction(
        "readAnthropicSettings",
        `chrome.storage["local"].get(${items})`,
      ),
    },
    {
      name: "the sync area",
      file: chromeDepsFile,
      text: 'export const r = () => chrome.storage.sync.get("x");\n',
    },
    {
      name: "chrome.storage passed on",
      file: chromeDepsFile,
      text: "export const s = chrome.storage;\n",
    },
    {
      name: "chrome.storage outside chromeDeps.ts",
      file: "src/sample.ts",
      text: inFunction(
        "readAnthropicSettings",
        `chrome.storage.local.get(${items})`,
      ),
    },
    {
      name: "the storage-wide change event",
      file: chromeDepsFile,
      text: "chrome.storage.onChanged.addListener(() => {});\n",
    },
    {
      name: "the local area's change event",
      file: chromeDepsFile,
      text: "chrome.storage.local.onChanged.addListener(() => {});\n",
    },
    {
      name: "the session area's change event outside chromeDeps.ts",
      file: "src/sample.ts",
      text: "chrome.storage.session.onChanged.addListener(() => {});\n",
    },
    {
      name: "a change event through another object",
      file: chromeDepsFile,
      text: "area.onChanged.addListener(() => {});\n",
    },
    {
      name: "a computed change event",
      file: "src/sample.ts",
      text: 'area["onChanged"].addListener(() => {});\n',
    },
    {
      name: "a destructured change event",
      file: chromeDepsFile,
      text: "const { onChanged } = area;\n",
    },
    {
      name: "the API key given to console.info",
      file: "src/sample.ts",
      text: 'console.info("label", apiKey);\n',
    },
    {
      name: "the API key member given to a logger",
      file: "src/sample.ts",
      text: 'logger.error("label", { value: settings.anthropicApiKey });\n',
    },
    {
      name: "the API key item given to console.log",
      file: "src/sample.ts",
      text: 'console.log(items["anthropic.apiKey"]);\n',
    },
  ];
  for (const { name, file, text } of refused) {
    it(`refuses ${name}`, () => {
      assert.notDeepEqual(apiKeyPathViolations(file, text), []);
    });
  }
});

describe("absent settings and old values", () => {
  const absentSettings: { name: string; text: string }[] = [
    { name: "a webhook url label", text: "<label>Webhook URL</label>" },
    { name: "a webhookUrl setting", text: "const webhookUrl = '';" },
    { name: "a webhook-url input id", text: '<input id="webhook-url">' },
    { name: "a webhook.url path", text: "const p = 'settings.webhook.url';" },
    {
      name: "the Webhook variable name",
      text: "const n = 'SLACK_WEBHOOK_URL';",
    },
    {
      name: "an API key variable name",
      text: "const n = 'ANTHROPIC_API_KEY';",
    },
  ];
  for (const { name, text } of absentSettings) {
    it(`refuses ${name}`, () => {
      assert.notDeepEqual(absentSettingsIn(text), []);
    });
  }

  const oldValues: { name: string; text: string }[] = [
    { name: "the old token", text: "const t = items.sharedToken;" },
    { name: "the old port", text: 'const p = items["serverPort"];' },
    { name: "a loopback address", text: "http://127.0.0.1:8787/" },
  ];
  for (const { name, text } of oldValues) {
    it(`refuses ${name}`, () => {
      assert.notDeepEqual(wordsIn(text, tokenAndPortWords), []);
    });
  }

  it("allows the word webhook alone", () => {
    assert.deepEqual(absentSettingsIn('const m = { publish: "webhook" };'), []);
  });

  it("allows the stored items' names", () => {
    assert.deepEqual(
      absentSettingsIn(
        'const n = ["anthropic.apiKey", "anthropic.workspaceId"];',
      ),
      [],
    );
  });
});
