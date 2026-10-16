import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { ESLint } from "eslint";

import { extensionDir } from "./helpers/paths.ts";

const eslint = new ESLint({ cwd: extensionDir });

/** Rule IDs reported for code, linted as if it were the file at filePath. */
async function ruleIds(
  code: string,
  filePath: string,
): Promise<(string | null)[]> {
  const [result] = await eslint.lintText(code, {
    filePath: `${extensionDir}/${filePath}`,
  });
  assert.ok(result, "ESLint returned no result");
  return result.messages.map((message) => message.ruleId);
}

// Each snippet is linted as a source file and as a test file: the rules
// cover the whole extension, tests included.
const paths = ["src/sample.ts", "test/sample.test.ts"];

/** Declares one test per case and file, asserting the rule is reported. */
function expectViolation(
  cases: { name: string; code: string; rule: string }[],
): void {
  for (const { name, code, rule } of cases) {
    for (const filePath of paths) {
      it(`${name} in ${filePath} violates ${rule}`, async () => {
        assert.ok((await ruleIds(code, filePath)).includes(rule), code);
      });
    }
  }
}

describe("code from strings", () => {
  expectViolation([
    { name: "eval", code: 'eval("1");\n', rule: "no-eval" },
    {
      name: "new Function",
      code: 'new Function("return 1");\n',
      rule: "no-new-func",
    },
    {
      name: "setTimeout with a string",
      code: 'setTimeout("alert(1)", 0);\n',
      rule: "no-implied-eval",
    },
  ]);
});

describe("html parsing APIs", () => {
  const declarations =
    "declare const el: HTMLElement;\ndeclare const s: string;\ndeclare const range: Range;\ndeclare const frame: HTMLIFrameElement;\n";
  expectViolation(
    [
      { name: "innerHTML", code: "el.innerHTML = s;\n" },
      { name: "computed innerHTML", code: 'el["innerHTML"] = s;\n' },
      { name: "outerHTML", code: "el.outerHTML = s;\n" },
      {
        name: "insertAdjacentHTML",
        code: 'el.insertAdjacentHTML("beforeend", s);\n',
      },
      { name: "document.write", code: "document.write(s);\n" },
      { name: "document.writeln", code: "document.writeln(s);\n" },
      { name: "ownerDocument.write", code: "el.ownerDocument.write(s);\n" },
      {
        name: "DOMParser",
        code: 'new DOMParser().parseFromString(s, "text/html");\n',
      },
      {
        name: "createContextualFragment",
        code: "range.createContextualFragment(s);\n",
      },
      { name: "setHTMLUnsafe", code: "el.setHTMLUnsafe(s);\n" },
      { name: "parseHTMLUnsafe", code: "Document.parseHTMLUnsafe(s);\n" },
      { name: "srcdoc", code: "frame.srcdoc = s;\n" },
      { name: "template-literal innerHTML", code: "el[`innerHTML`] = s;\n" },
      {
        name: "Object.assign innerHTML",
        code: "Object.assign(el, { innerHTML: s });\n",
      },
      {
        name: "destructured innerHTML",
        code: "export const { innerHTML } = el;\n",
      },
      {
        name: "setAttribute srcdoc",
        code: 'frame.setAttribute("srcdoc", s);\n',
      },
      {
        name: "setAttribute onerror",
        code: 'el.setAttribute("onerror", s);\n',
      },
      { name: "computed document.write", code: 'document["write"](s);\n' },
      { name: "window.document.write", code: "window.document.write(s);\n" },
    ].map(({ name, code }) => ({
      name,
      code: declarations + code,
      rule: "no-restricted-syntax",
    })),
  );

  for (const filePath of paths) {
    it(`textContent in ${filePath} is allowed`, async () => {
      const code =
        "export function show(el: HTMLElement, s: string): void {\n  el.textContent = s;\n}\n";
      assert.deepEqual(await ruleIds(code, filePath), []);
    });
  }
});

describe("dynamic import", () => {
  expectViolation([
    {
      name: "import()",
      code: 'export const m = import("./x.js");\n',
      rule: "no-restricted-syntax",
    },
  ]);
});

interface Brand {
  readonly type: string;
  readonly creator: string;
}

const brands: Brand[] = [
  { type: "CollectedInput", creator: "src/core/collect.ts" },
  { type: "SharedToken", creator: "src/core/settings.ts" },
  { type: "ServerPort", creator: "src/core/settings.ts" },
  { type: "PreparedRequest", creator: "src/core/prepare.ts" },
  { type: "RequestId", creator: "src/core/response.ts" },
];

const creatorFiles = [...new Set(brands.map((brand) => brand.creator))];
const otherFiles = ["src/sample.ts", "scripts/sample.ts"];

const assertion = (type: string): string =>
  `declare const value: unknown;\nexport const x = value as ${type};\n`;

describe("branded type assertions", () => {
  for (const { type, creator } of brands) {
    it(`allows the ${type} assertion in its creator ${creator}`, async () => {
      assert.deepEqual(await ruleIds(assertion(type), creator), []);
    });

    it(`allows the ${type} assertion in a test`, async () => {
      assert.deepEqual(
        await ruleIds(assertion(type), "test/sample.test.ts"),
        [],
      );
    });

    for (const filePath of [...otherFiles, ...creatorFiles]) {
      if (filePath === creator) {
        continue;
      }
      it(`forbids the ${type} assertion in ${filePath}`, async () => {
        assert.ok(
          (await ruleIds(assertion(type), filePath)).includes(
            "no-restricted-syntax",
          ),
          type,
        );
      });
    }
  }

  it("allows an assertion of a non-branded type in a source file", async () => {
    const code =
      "declare const value: unknown;\nexport const x = value as Record<string, unknown>;\n";
    assert.deepEqual(await ruleIds(code, "src/sample.ts"), []);
  });

  const forms: { name: string; code: (type: string) => string }[] = [
    {
      name: "an angle-bracket assertion",
      code: (type) =>
        `declare const value: unknown;\nexport const x = <${type}>value;\n`,
    },
    {
      name: "an assertion with a union",
      code: (type) =>
        `declare const value: unknown;\nexport const x = value as ${type} | undefined;\n`,
    },
    {
      name: "an assertion with a generic wrapper",
      code: (type) =>
        `declare const value: unknown;\nexport const x = value as Readonly<${type}>;\n`,
    },
    {
      name: "a namespace assertion",
      code: (type) =>
        `declare const value: unknown;\nexport const x = value as settings.${type};\n`,
    },
    {
      name: "a type predicate",
      code: (type) =>
        `export function f(v: unknown): v is ${type} {\n  return true;\n}\n`,
    },
    { name: "a type alias", code: (type) => `export type Alias = ${type};\n` },
    {
      name: "a renaming import",
      code: (type) =>
        `import type { ${type} as Renamed } from "./x.ts";\nexport type Y = Renamed;\n`,
    },
  ];
  for (const { type } of brands) {
    for (const { name, code } of forms) {
      it(`forbids ${name} of ${type} in a source file`, async () => {
        assert.ok(
          (await ruleIds(code(type), "src/sample.ts")).includes(
            "no-restricted-syntax",
          ),
          name,
        );
      });
    }
  }
});

describe("storage.sync", () => {
  const violations: { name: string; code: string }[] = [
    {
      name: "the dotted reference",
      code: 'export const p = chrome.storage.sync.get("x");\n',
    },
    {
      name: "the computed string reference",
      code: 'export const p = chrome.storage["sync"].get("x");\n',
    },
    {
      name: "the computed template reference",
      code: 'export const p = chrome.storage[`sync`].get("x");\n',
    },
    {
      name: "the destructured reference",
      code: "export const { sync } = chrome.storage;\n",
    },
  ];
  for (const { name, code } of violations) {
    expectViolation([{ name, code, rule: "no-restricted-syntax" }]);
  }

  for (const filePath of paths) {
    it(`allows chrome.storage.local in ${filePath}`, async () => {
      const code = 'export const p = chrome.storage.local.get("x");\n';
      assert.deepEqual(await ruleIds(code, filePath), []);
    });
  }
});

describe("common selectors in creator files", () => {
  const violations: { name: string; code: string }[] = [
    {
      name: "innerHTML",
      code: "declare const el: HTMLElement;\ndeclare const s: string;\nexport const f = (): void => {\n  el.innerHTML = s;\n};\n",
    },
    { name: "import()", code: 'export const m = import("./x.js");\n' },
    {
      name: "storage.sync",
      code: 'export const p = chrome.storage.sync.get("x");\n',
    },
  ];
  for (const creator of creatorFiles) {
    for (const { name, code } of violations) {
      it(`forbids ${name} in ${creator}`, async () => {
        assert.ok(
          (await ruleIds(code, creator)).includes("no-restricted-syntax"),
          name,
        );
      });
    }
  }
});
