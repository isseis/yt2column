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
  { type: "UsdAmount", creator: "src/core/response.ts" },
  { type: "UsdAmount", creator: "src/core/wasmResult.ts" },
  { type: "LinkUrl", creator: "src/core/articleMarkdown.ts" },
];

const creatorFiles = [...new Set(brands.map((brand) => brand.creator))];

/** Every file that may create type: a brand may have more than one creator. */
const creatorsOf = (type: string): string[] =>
  brands.filter((brand) => brand.type === type).map((brand) => brand.creator);
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
      if (creatorsOf(type).includes(filePath)) {
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

const chromeDeps = "src/browser/chromeDeps.ts";
const wasmWorker = "src/wasmWorker.ts";

describe("chrome.storage", () => {
  // Every area is forbidden outside browser/chromeDeps.ts, sources and tests
  // alike: only its fixed functions use chrome.storage.
  expectViolation([
    {
      name: "chrome.storage.local",
      code: 'export const p = chrome.storage.local.get("x");\n',
      rule: "no-restricted-syntax",
    },
    {
      name: "chrome.storage.session",
      code: "export const p = chrome.storage.session.get(null);\n",
      rule: "no-restricted-syntax",
    },
  ]);
  expectReferenceForms("chrome", "storage");

  for (const code of [
    'export const p = chrome.storage.local.get(["anthropic.apiKey", "anthropic.workspaceId"]);\n',
    "export const p = chrome.storage.session.get(null);\n",
    "chrome.storage.session.onChanged.addListener(() => {});\n",
  ]) {
    it(`allows ${code.trim()} in ${chromeDeps}`, async () => {
      assert.deepEqual(await ruleIds(code, chromeDeps), []);
    });
  }

  for (const filePath of [
    wasmWorker,
    "src/core/creator.ts",
    "src/options.ts",
  ]) {
    it(`forbids chrome.storage in ${filePath}`, async () => {
      const code = "export const p = chrome.storage.local.getKeys();\n";
      assert.ok(
        (await ruleIds(code, filePath)).includes("no-restricted-syntax"),
      );
    });
  }
});

// The fixed chrome.storage functions and the entry page scripts that may
// import each; eslint.config.js holds the same table.
const storageFunctionImporters: Readonly<Record<string, readonly string[]>> = {
  readAnthropicSettings: ["src/viewer.ts"],
  writeAnthropicSettings: ["src/options.ts"],
  removeAnthropicSettings: ["src/options.ts"],
  localKeys: ["src/background.ts", "src/options.ts"],
  setLocalAccessLevel: ["src/background.ts", "src/popup.ts"],
  readSessionItems: ["src/background.ts", "src/popup.ts", "src/viewer.ts"],
  writeSessionItems: ["src/background.ts"],
  removeSessionItems: ["src/background.ts"],
  onSessionChanged: ["src/popup.ts", "src/viewer.ts"],
};
const entryFiles = [
  "src/background.ts",
  "src/popup.ts",
  "src/viewer.ts",
  "src/options.ts",
];

describe("importing the chrome.storage functions", () => {
  for (const [name, importers] of Object.entries(storageFunctionImporters)) {
    const code = `import { ${name} } from "./browser/chromeDeps.ts";\nexport const f = ${name};\n`;
    for (const filePath of entryFiles) {
      if (importers.includes(filePath)) {
        it(`allows importing ${name} in ${filePath}`, async () => {
          assert.deepEqual(await ruleIds(code, filePath), []);
        });
      } else {
        it(`forbids importing ${name} in ${filePath}`, async () => {
          assert.ok(
            (await ruleIds(code, filePath)).includes("no-restricted-imports"),
          );
        });
      }
    }
    it(`forbids importing ${name} in src/sample.ts`, async () => {
      assert.ok(
        (await ruleIds(code, "src/sample.ts")).includes(
          "no-restricted-imports",
        ),
      );
    });
    it(`forbids importing ${name} from a nested module`, async () => {
      const nested = `import { ${name} } from "../browser/chromeDeps.ts";\nexport const f = ${name};\n`;
      assert.ok(
        (await ruleIds(nested, "src/core/sample.ts")).includes(
          "no-restricted-imports",
        ),
      );
    });
    it(`forbids importing ${name} with a .js specifier`, async () => {
      // nodenext resolves "./browser/chromeDeps.js" to the .ts file too.
      for (const [specifier, filePath] of [
        ["./browser/chromeDeps.js", "src/sample.ts"],
        ["../browser/chromeDeps.js", "src/core/sample.ts"],
        // A module next to chromeDeps.ts imports it without browser/.
        ["./chromeDeps.ts", "src/browser/sample.ts"],
        ["./chromeDeps.js", "src/browser/sample.ts"],
        ["../chromeDeps.ts", "src/browser/sub/sample.ts"],
      ] as const) {
        const js = `import { ${name} } from "${specifier}";\nexport const f = ${name};\n`;
        assert.ok(
          (await ruleIds(js, filePath)).includes("no-restricted-imports"),
          specifier,
        );
      }
    });
    it(`allows a type-only import of ${name} in src/sample.ts`, async () => {
      const typeOnly = `import type { ${name} } from "./browser/chromeDeps.ts";\nexport type F = typeof ${name};\n`;
      assert.deepEqual(await ruleIds(typeOnly, "src/sample.ts"), []);
    });
  }

  it("allows importing the other chromeDeps.ts functions in src/sample.ts", async () => {
    const code =
      'import { createLogger, storedAnthropicSettings } from "./browser/chromeDeps.ts";\nexport const f = [createLogger, storedAnthropicSettings];\n';
    assert.deepEqual(await ruleIds(code, "src/sample.ts"), []);
  });
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

  // The Worker's loader fetches the article generation module; the other
  // connections stay forbidden there.
  for (const code of [
    'export const p = fetch("yt2column.wasm");\n',
    "declare const self: Record<string, unknown>;\nexport const p = self.fetch;\n",
  ]) {
    it(`allows ${code.trim().split("\n").at(-1)} in ${wasmWorker}`, async () => {
      assert.deepEqual(await ruleIds(code, wasmWorker), []);
    });
  }
  for (const name of ["XMLHttpRequest", "WebSocket", "EventSource"]) {
    it(`forbids the global ${name} in ${wasmWorker}`, async () => {
      assert.ok(
        (await ruleIds(`export const p = ${name};\n`, wasmWorker)).includes(
          "no-restricted-globals",
        ),
      );
    });
    it(`forbids self.${name} in ${wasmWorker}`, async () => {
      const code = `declare const self: Record<string, unknown>;\nexport const p = self.${name};\n`;
      assert.ok(
        (await ruleIds(code, wasmWorker)).includes("no-restricted-syntax"),
      );
    });
  }
  for (const filePath of [chromeDeps, "src/wasmWorkerClient.ts"]) {
    it(`forbids fetch in ${filePath}`, async () => {
      assert.ok(
        (await ruleIds("export const p = fetch;\n", filePath)).includes(
          "no-restricted-globals",
        ),
      );
    });
  }
});

describe("the other rules in the files with an exception", () => {
  // browser/chromeDeps.ts may use storage, windows and sendNativeMessage, and
  // src/wasmWorker.ts may fetch; every other rule still applies to both.
  const violations: { name: string; code: string; files: string[] }[] = [
    {
      name: "innerHTML",
      code: "declare const el: HTMLElement;\ndeclare const s: string;\nexport const f = (): void => {\n  el.innerHTML = s;\n};\n",
      files: [chromeDeps, wasmWorker],
    },
    {
      name: "import()",
      code: 'export const m = import("./x.js");\n',
      files: [chromeDeps, wasmWorker],
    },
    {
      name: "chrome.session",
      code: "export const p = chrome.session;\n",
      files: [chromeDeps, wasmWorker],
    },
    {
      name: "connectNative",
      code: 'export const p = chrome.runtime.connectNative("h");\n',
      files: [chromeDeps, wasmWorker],
    },
    {
      name: "window.fetch",
      code: "declare const window: Record<string, unknown>;\nexport const p = window.fetch;\n",
      files: [chromeDeps],
    },
    {
      name: "chrome.windows",
      code: "export const p = chrome.windows;\n",
      files: [wasmWorker],
    },
    {
      name: "sendNativeMessage",
      code: "export const p = chrome.runtime.sendNativeMessage;\n",
      files: [wasmWorker],
    },
  ];
  for (const { name, code, files } of violations) {
    for (const filePath of files) {
      it(`forbids ${name} in ${filePath}`, async () => {
        assert.ok(
          (await ruleIds(code, filePath)).includes("no-restricted-syntax"),
          name,
        );
      });
    }
  }
});

describe("inline configuration comments", () => {
  it("cannot switch a rule off", async () => {
    const code =
      '// eslint-disable-next-line no-restricted-imports\nimport { readAnthropicSettings } from "./browser/chromeDeps.ts";\nexport const f = readAnthropicSettings;\n';
    assert.ok(
      (await ruleIds(code, "src/popup.ts")).includes("no-restricted-imports"),
    );
  });
});

describe("web storage and channels", () => {
  // The settings live only in chrome.storage, and only browser/chromeDeps.ts
  // opens the viewers' channel; src/ files are covered, tests are not.
  const stores = ["localStorage", "sessionStorage", "indexedDB", "caches"];
  for (const filePath of ["src/sample.ts", chromeDeps, wasmWorker]) {
    for (const name of stores) {
      it(`forbids the global ${name} in ${filePath}`, async () => {
        assert.ok(
          (await ruleIds(`export const p = ${name};\n`, filePath)).includes(
            "no-restricted-globals",
          ),
        );
      });
    }
    it(`forbids window.localStorage in ${filePath}`, async () => {
      const code =
        "declare const window: Record<string, unknown>;\nexport const p = window.localStorage;\n";
      assert.ok(
        (await ruleIds(code, filePath)).includes("no-restricted-syntax"),
      );
    });
  }
  const channel = 'export const c = new BroadcastChannel("c");\n';
  for (const filePath of [
    "src/sample.ts",
    wasmWorker,
    "src/core/wasmResult.ts",
  ]) {
    it(`forbids a BroadcastChannel in ${filePath}`, async () => {
      assert.ok(
        (await ruleIds(channel, filePath)).includes("no-restricted-globals"),
      );
    });
  }
  it(`allows a BroadcastChannel in ${chromeDeps}`, async () => {
    assert.deepEqual(await ruleIds(channel, chromeDeps), []);
  });
  it("allows a BroadcastChannel in a test", async () => {
    assert.deepEqual(await ruleIds(channel, "test/sample.test.ts"), []);
  });
});

describe("html parsing APIs in a creator file of the extension-only path", () => {
  // wasmResult.ts has a config of its own (it creates UsdAmount), which must
  // keep the common rules.
  it("forbids innerHTML in src/core/wasmResult.ts", async () => {
    const code =
      "declare const el: HTMLElement;\ndeclare const s: string;\nexport const f = (): void => {\n  el.innerHTML = s;\n};\n";
    assert.ok(
      (await ruleIds(code, "src/core/wasmResult.ts")).includes(
        "no-restricted-syntax",
      ),
    );
  });
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

  it("allows a fake with the name in a test", async () => {
    const code = "export const fake = { sendNativeMessage() {} };\n";
    assert.deepEqual(await ruleIds(code, "test/sample.test.ts"), []);
  });
});

describe("removed menu-path APIs", () => {
  const references: { api: string; name: string }[] = [
    { api: "chrome", name: "contextMenus" },
    { api: "chrome", name: "session" },
    { api: "chrome", name: "windows" },
    { api: "chrome.action", name: "setBadgeText" },
  ];
  /** The five reference forms of api.name, which the rule forbids alike. */
  const referenceForms = (
    api: string,
    name: string,
  ): { form: string; code: string }[] => [
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
  for (const { api, name } of references) {
    for (const { form, code } of referenceForms(api, name)) {
      expectViolation([
        { name: `${name} as ${form}`, code, rule: "no-restricted-syntax" },
      ]);
    }
  }

  // The window focus wrapper in browser/chromeDeps.ts is the one place that
  // may reference windows; the other names stay forbidden there. The forms
  // come from the same selector generator as above, so one form proves each
  // forbidden name and every form is tried for the allowed one.
  for (const { api, name } of references) {
    const forms = referenceForms(api, name);
    for (const { form, code } of name === "windows"
      ? forms
      : forms.slice(0, 1)) {
      if (name === "windows") {
        it(`allows ${name} as ${form} in ${chromeDeps}`, async () => {
          assert.deepEqual(await ruleIds(code, chromeDeps), []);
        });
      } else {
        it(`forbids ${name} as ${form} in ${chromeDeps}`, async () => {
          assert.ok(
            (await ruleIds(code, chromeDeps)).includes("no-restricted-syntax"),
          );
        });
      }
    }
  }

  // session is forbidden only on chrome itself: chrome.storage.session holds
  // the job records, and another object's session member is not the API.
  for (const code of [
    "declare const globalThis: { chrome: unknown };\nexport const p = globalThis.chrome.session;\n",
    "export const { session } = chrome;\n",
  ]) {
    for (const filePath of paths) {
      it(`forbids ${code.trim().split("\n").at(-1)} in ${filePath}`, async () => {
        assert.ok(
          (await ruleIds(code, filePath)).includes("no-restricted-syntax"),
        );
      });
    }
  }
  for (const code of [
    "declare const deps: { session: number };\nexport const p = deps.session;\n",
    "declare const x: { session: number };\nexport const { session } = x;\n",
  ]) {
    for (const filePath of paths) {
      it(`allows ${code.trim().split("\n").at(-1)} in ${filePath}`, async () => {
        assert.deepEqual(await ruleIds(code, filePath), []);
      });
    }
  }

  // The pattern is anchored at both ends: a longer name that only starts or
  // ends with a forbidden one stays allowed.
  const longerNames = [
    "export const p = chrome.action.setBadgeTextColor;\n",
    "export const { sessionId } = chrome.x;\n",
    "export const { usersession } = chrome.x;\n",
    "export const p = chrome.x.subwindows;\n",
    "export const p = chrome.sessions;\n",
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
    "export type T = typeof chrome.session;\n",
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
