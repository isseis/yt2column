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
  { type: "PreparedRequest", creator: "src/core/prepare.ts" },
  { type: "RequestId", creator: "src/core/response.ts" },
  { type: "ArticleKey", creator: "src/core/articleKey.ts" },
];

const creatorFiles = [...new Set(brands.map((brand) => brand.creator))];
const otherFiles = [
  "src/sample.ts",
  "scripts/sample.ts",
  "src/browser/chromeDeps.ts",
];

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
    {
      name: "a renaming re-export",
      code: (type) => `export type { ${type} as Renamed } from "./x.ts";\n`,
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

  // A type alias may name a brand as an object type member's type, as an
  // interface may; every position that aliases the brand itself stays
  // forbidden.
  const named = (type: string): string =>
    `import type { ${type} } from "./x.ts";\n`;
  const namespace = 'import type * as ns from "./x.ts";\n';
  const allowedAliases: { name: string; code: (type: string) => string }[] = [
    {
      name: "an object type member",
      code: (type) =>
        `${named(type)}export type A = { readonly id: ${type} };\n`,
    },
    {
      name: "a member of an object type in a union",
      code: (type) =>
        `${named(type)}export type A =\n  | { readonly kind: "a"; readonly id: ${type} }\n  | { readonly kind: "b" };\n`,
    },
    {
      name: "a nested object type member",
      code: (type) =>
        `${named(type)}export type A = { readonly b: { readonly id: ${type} } };\n`,
    },
    {
      name: "a generic wrapper inside an object type member",
      code: (type) =>
        `${named(type)}export type A = { readonly id: Readonly<${type}> };\n`,
    },
    {
      name: "a namespace reference inside an object type member",
      code: (type) =>
        `${namespace}export type A = { readonly id: ns.${type} };\n`,
    },
  ];
  const forbiddenAliases: { name: string; code: (type: string) => string }[] = [
    {
      name: "a union member next to an object type",
      code: (type) =>
        `${named(type)}export type A = { readonly id: ${type} } | ${type};\n`,
    },
    {
      name: "an intersection",
      code: (type) => `${named(type)}export type A = ${type} & {};\n`,
    },
    {
      name: "a generic argument",
      code: (type) => `${named(type)}export type A = Readonly<${type}>;\n`,
    },
    {
      name: "an indexed access of an object type member",
      code: (type) =>
        `${named(type)}export type A = { readonly id: ${type} }["id"];\n`,
    },
    {
      name: "an object type member in a conditional type",
      code: (type) =>
        `${named(type)}export type A = string extends number ? { readonly id: ${type} } : never;\n`,
    },
    {
      name: "a namespace reference outside an object type member",
      code: (type) => `${namespace}export type A = ns.${type} | undefined;\n`,
    },
    {
      name: "a namespace reference in an indexed access",
      code: (type) =>
        `${namespace}export type A = { readonly id: ns.${type} }["id"];\n`,
    },
    {
      name: "a namespace reference in a conditional type",
      code: (type) =>
        `${namespace}export type A = string extends number ? { readonly id: ns.${type} } : never;\n`,
    },
  ];
  for (const { type } of brands) {
    for (const { name, code } of allowedAliases) {
      it(`allows a type alias with ${type} in ${name} in a source file`, async () => {
        assert.deepEqual(await ruleIds(code(type), "src/sample.ts"), []);
      });
    }
    for (const { name, code } of forbiddenAliases) {
      it(`forbids a type alias with ${type} in ${name} in a source file`, async () => {
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

/** Declares one violation test per reference form of obj.name, in src and test files. */
function expectReferenceForms(obj: string, name: string): void {
  const forms: { form: string; code: string }[] = [
    {
      form: "the dotted reference",
      code: `export const p = ${obj}.${name};\n`,
    },
    {
      form: "the computed string reference",
      code: `export const p = ${obj}["${name}"];\n`,
    },
    {
      form: "the computed template reference",
      code: `export const p = ${obj}[\`${name}\`];\n`,
    },
    {
      form: "the destructured reference",
      code: `export const { ${name} } = ${obj};\n`,
    },
    {
      form: "the string-keyed destructured reference",
      code: `export const { "${name}": x } = ${obj};\n`,
    },
  ];
  for (const { form, code } of forms) {
    expectViolation([
      { name: `${name} as ${form}`, code, rule: "no-restricted-syntax" },
    ]);
  }
}

describe("chrome.storage", () => {
  // Every area is forbidden, not only sync: the extension has no storage
  // permission.
  expectViolation([
    {
      name: "chrome.storage.local",
      code: 'export const p = chrome.storage.local.get("x");\n',
      rule: "no-restricted-syntax",
    },
  ]);
  expectReferenceForms("chrome", "storage");
});

describe("connections", () => {
  for (const name of ["fetch", "XMLHttpRequest", "WebSocket", "EventSource"]) {
    expectViolation([
      {
        name: `global ${name}`,
        code: `export const p = ${name};\n`,
        rule: "no-restricted-globals",
      },
    ]);
    for (const obj of ["window", "self", "globalThis"]) {
      expectViolation([
        {
          name: `${obj}.${name}`,
          code: `declare const ${obj}: Record<string, unknown>;\nexport const p = ${obj}.${name};\n`,
          rule: "no-restricted-syntax",
        },
        {
          name: `template ${obj}[\`${name}\`]`,
          code: `declare const ${obj}: Record<string, unknown>;\nexport const p = ${obj}[\`${name}\`];\n`,
          rule: "no-restricted-syntax",
        },
        {
          name: `destructured ${name} from ${obj}`,
          code: `declare const ${obj}: Record<string, unknown>;\nexport const { ${name} } = ${obj};\n`,
          rule: "no-restricted-syntax",
        },
        {
          name: `computed ${obj}["${name}"]`,
          code: `declare const ${obj}: Record<string, unknown>;\nexport const p = ${obj}["${name}"];\n`,
          rule: "no-restricted-syntax",
        },
      ]);
    }
  }

  // The names are anchored, and only the global objects are covered: a
  // longer name on a global object and a same-named member of another
  // object (an injected dependency) stay allowed.
  const allowed = [
    "declare const window: Record<string, unknown>;\nexport const p = window.fetchAll;\n",
    "declare const deps: { fetch: number };\nexport const p = deps.fetch;\n",
  ];
  for (const code of allowed) {
    for (const filePath of paths) {
      it(`allows ${code.split("\n")[1]} in ${filePath}`, async () => {
        assert.deepEqual(await ruleIds(code, filePath), []);
      });
    }
  }
});

describe("chrome.runtime.connectNative", () => {
  expectReferenceForms("chrome.runtime", "connectNative");
});

describe("chrome.runtime.sendNativeMessage", () => {
  const call =
    'export const p = chrome.runtime.sendNativeMessage("h", {}, () => {});\n';

  for (const filePath of [
    "src/sample.ts",
    "src/core/collect.ts",
    "src/core/prepare.ts",
    "src/core/response.ts",
    "src/core/articleKey.ts",
  ]) {
    it(`forbids the call in ${filePath}`, async () => {
      assert.ok(
        (await ruleIds(call, filePath)).includes("no-restricted-syntax"),
      );
    });
  }

  for (const form of [
    'export const p = chrome.runtime["sendNativeMessage"];\n',
    "export const { sendNativeMessage } = chrome.runtime;\n",
  ]) {
    it(`forbids ${form.trim()} in src/sample.ts`, async () => {
      assert.ok(
        (await ruleIds(form, "src/sample.ts")).includes("no-restricted-syntax"),
      );
    });
  }

  it("allows the call in src/browser/chromeDeps.ts", async () => {
    assert.deepEqual(await ruleIds(call, "src/browser/chromeDeps.ts"), []);
  });

  it("still applies the other rules in src/browser/chromeDeps.ts", async () => {
    const code = 'export const p = chrome.storage.local.get("x");\n';
    assert.ok(
      (await ruleIds(code, "src/browser/chromeDeps.ts")).includes(
        "no-restricted-syntax",
      ),
    );
  });

  it("allows a fake with the name in a test", async () => {
    const code = "export const fake = { sendNativeMessage() {} };\n";
    assert.deepEqual(await ruleIds(code, "test/sample.test.ts"), []);
  });
});

describe("removed menu-path APIs", () => {
  const references: { api: string; name: string }[] = [
    { api: "chrome", name: "contextMenus" },
    { api: "chrome.x", name: "session" },
    { api: "chrome", name: "windows" },
    { api: "chrome.action", name: "setBadgeText" },
  ];
  for (const { api, name } of references) {
    const violations: { form: string; code: string }[] = [
      {
        form: "the dotted reference",
        code: `export const p = ${api}.${name};\n`,
      },
      {
        form: "the computed string reference",
        code: `export const p = ${api}["${name}"];\n`,
      },
      {
        form: "the computed template reference",
        code: `export const p = ${api}[\`${name}\`];\n`,
      },
      {
        form: "the destructured reference",
        code: `export const { ${name} } = ${api};\n`,
      },
      {
        form: "the string-keyed destructured reference",
        code: `export const { "${name}": x } = ${api};\n`,
      },
    ];
    for (const { form, code } of violations) {
      expectViolation([
        { name: `${name} as ${form}`, code, rule: "no-restricted-syntax" },
      ]);
    }
  }

  // The pattern is anchored at both ends: a longer name that only starts or
  // ends with a forbidden one stays allowed.
  const longerNames = [
    "export const p = chrome.action.setBadgeTextColor;\n",
    "export const { sessionId } = chrome.x;\n",
    "export const { usersession } = chrome.x;\n",
    "export const p = chrome.x.subwindows;\n",
  ];
  for (const code of longerNames) {
    for (const filePath of paths) {
      it(`allows ${code.trim()} in ${filePath}`, async () => {
        assert.deepEqual(await ruleIds(code, filePath), []);
      });
    }
  }

  // A type-only reference does not call the API, so it stays allowed.
  const typeReferences = [
    "export type T = chrome.contextMenus.OnClickData;\n",
    "export type T = typeof chrome.x.session;\n",
    "export type T = chrome.windows.Window;\n",
    "export type T = typeof chrome.action.setBadgeText;\n",
  ];
  for (const code of typeReferences) {
    for (const filePath of paths) {
      it(`allows the type reference ${code.trim()} in ${filePath}`, async () => {
        assert.deepEqual(await ruleIds(code, filePath), []);
      });
    }
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
      name: "chrome.storage",
      code: 'export const p = chrome.storage.local.get("x");\n',
    },
    {
      name: "connectNative",
      code: 'export const p = chrome.runtime.connectNative("h");\n',
    },
    {
      name: "sendNativeMessage",
      code: "export const p = chrome.runtime.sendNativeMessage;\n",
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
