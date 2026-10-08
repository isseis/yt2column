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
