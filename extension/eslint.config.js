import js from "@eslint/js";
import { defineConfig } from "eslint/config";
import tseslint from "typescript-eslint";

// DOM APIs that parse a string as HTML. Text read from the page is untrusted,
// so the extension builds elements and sets textContent instead.
const htmlParsingProperties = [
  "innerHTML",
  "outerHTML",
  "insertAdjacentHTML",
  "createContextualFragment",
  "setHTMLUnsafe",
  "parseHTMLUnsafe",
  "srcdoc",
];
const htmlParsingPattern = `/^(${htmlParsingProperties.join("|")})$/`;
const documentWritePattern = "/^(write|writeln)$/";
const htmlParsingMessage =
  "Do not parse strings as HTML; build elements and set textContent.";

/** Selectors that forbid a member name in the four reference forms: dotted, computed string, computed template and destructured. */
function memberNameSelectors(names, message) {
  const pattern = `/^(${names.join("|")})$/`;
  return [
    {
      selector: `MemberExpression[computed=false][property.name=${pattern}]`,
      message,
    },
    {
      selector: `MemberExpression[computed=true][property.value=${pattern}]`,
      message,
    },
    {
      selector: `MemberExpression[computed=true][property.type="TemplateLiteral"][property.quasis.0.value.cooked=${pattern}]`,
      message,
    },
    {
      selector: `Property[key.name=${pattern}], Property[key.value=${pattern}]`,
      message,
    },
  ];
}

// chrome.storage holds the API key, so only browser/chromeDeps.ts may use it,
// through the fixed functions it exports; their importers are limited below.
// The type declarations contain chrome.storage, so tsc alone would accept a
// reference anywhere. The destructuring form matches any { storage } key by
// name, so the rule is intentionally name-based and would also reject an
// unrelated property named storage.
const storageSelectors = memberNameSelectors(
  ["storage"],
  "Use chrome.storage only through the fixed functions of browser/chromeDeps.ts.",
);

// The extension sends to the network only from the article generation
// module, which runs in the Worker that src/wasmWorker.ts loads: the module's
// Go code keeps the API key in a secret and refuses redirects, which a fetch
// in the extension's TypeScript would bypass. src/wasmWorker.ts may use fetch
// to load the module; the other names stay forbidden there, and the native
// messaging host is reached only through chrome.runtime.sendNativeMessage in
// browser/chromeDeps.ts. Like the storage rule, these guard against
// accidental reuse, not deliberate evasion.
const networkNames = ["fetch", "XMLHttpRequest", "WebSocket", "EventSource"];
const networkMessage =
  "Do not open connections; only src/wasmWorker.ts may fetch, to load the article generation module.";
// The names src/wasmWorker.ts still may not use.
const workerNetworkNames = networkNames.filter((name) => name !== "fetch");
/** no-restricted-globals entries for names. */
function globalEntries(names, message) {
  return names.map((name) => ({ name, message }));
}
const globalObject = "/^(window|self|globalThis)$/";
/** Selectors that forbid names as members of the global objects, dotted, computed or destructured. */
function globalMemberSelectors(names, message) {
  const pattern = `/^(${names.join("|")})$/`;
  return [
    {
      // window.fetch, self.fetch and globalThis.fetch, dotted or computed.
      selector: `MemberExpression:matches([property.name=${pattern}], [property.value=${pattern}])[object.name=${globalObject}]`,
      message,
    },
    {
      selector: `MemberExpression[computed=true][property.type="TemplateLiteral"][property.quasis.0.value.cooked=${pattern}][object.name=${globalObject}]`,
      message,
    },
    {
      // const { fetch } = window;
      selector: `VariableDeclarator[init.name=${globalObject}] > ObjectPattern > Property:matches([key.name=${pattern}], [key.value=${pattern}])`,
      message,
    },
  ];
}
/** no-restricted-globals entries for the network names. */
function networkGlobals(names) {
  return globalEntries(names, networkMessage);
}
/** The member selectors for the network names. */
function networkSelectors(names) {
  return globalMemberSelectors(names, networkMessage);
}

// The other places the extension's source could keep a value or pass it to
// every page: web storage, and a BroadcastChannel. The settings live only in
// chrome.storage, through the fixed functions of browser/chromeDeps.ts, which
// also opens the viewers' one channel. These apply to src/ only: tests build
// channels to test that wrapper.
const webStoreNames = ["localStorage", "sessionStorage", "indexedDB", "caches"];
const webStoreMessage =
  "Do not use web storage; the extension keeps settings only in chrome.storage through browser/chromeDeps.ts.";
const channelNames = ["BroadcastChannel"];
const channelMessage =
  "Only browser/chromeDeps.ts may open a BroadcastChannel; use its viewer channel.";

/** The no-restricted-globals entries of a src/ file: fetch, and the channel, as its exceptions allow. */
function sourceGlobals({ allowFetch = false, allowChannel = false } = {}) {
  return [
    ...networkGlobals(allowFetch ? workerNetworkNames : networkNames),
    ...globalEntries(webStoreNames, webStoreMessage),
    ...(allowChannel ? [] : globalEntries(channelNames, channelMessage)),
  ];
}

/** The member selectors of a src/ file for web storage and, unless allowed, the channel. */
function sourceStoreSelectors({ allowChannel = false } = {}) {
  return [
    ...globalMemberSelectors(webStoreNames, webStoreMessage),
    ...(allowChannel
      ? []
      : globalMemberSelectors(channelNames, channelMessage)),
  ];
}

const connectNativeSelectors = memberNameSelectors(
  ["connectNative"],
  "Do not use chrome.runtime.connectNative; send with sendNativeMessage in browser/chromeDeps.ts.",
);
// Only browser/chromeDeps.ts may call sendNativeMessage; its config below
// leaves this list out. Tests are not covered: they build fakes with the name.
const sendNativeSelectors = memberNameSelectors(
  ["sendNativeMessage"],
  "Only browser/chromeDeps.ts may call chrome.runtime.sendNativeMessage.",
);

// These APIs served the removed context-menu path (the menu, the result
// window, its session store and the toolbar badge). windows and action need no
// permission, so only this rule keeps them from coming back. Like the storage
// rule, it guards against accidental reuse, not deliberate evasion. The one
// exception is windows in browser/chromeDeps.ts, whose wrapper brings a viewer
// tab's window to the front; its config below leaves windows out of the list.
// session is forbidden only as chrome.session, the sessions API of that path:
// chrome.storage.session holds the job records.
const menuPathNames = ["contextMenus", "windows", "setBadgeText"];
const menuPathMessage =
  "Do not use chrome.contextMenus, chrome.session or chrome.action.setBadgeText, or chrome.windows outside browser/chromeDeps.ts; they served the removed context-menu path.";
const chromeObject =
  ':matches([object.name="chrome"], [object.property.name="chrome"])';
/** Selectors that forbid chrome.session, dotted, computed or destructured from chrome. */
const chromeSessionSelectors = [
  {
    selector: `MemberExpression${chromeObject}:matches([computed=false][property.name="session"], [computed=true][property.value="session"])`,
    message: menuPathMessage,
  },
  {
    selector: `MemberExpression${chromeObject}[computed=true][property.type="TemplateLiteral"][property.quasis.0.value.cooked="session"]`,
    message: menuPathMessage,
  },
  {
    selector: `VariableDeclarator:matches([init.name="chrome"], [init.property.name="chrome"]) > ObjectPattern > Property:matches([key.name="session"], [key.value="session"])`,
    message: menuPathMessage,
  },
];

// The branded types hide their brand symbol, so only the function named here
// may assert the type to create a value. Every other reference form (an alias
// import, a type alias, a nested reference, a namespace reference, a type
// predicate) is rejected too.
const brandMessage =
  "Only the type's creator may assert this branded type; build it through its constructor.";

/** Selectors that forbid asserting, aliasing or renaming any of the given brand names. */
function brandSelectors(names) {
  const name = `/^(${names.join("|")})$/`;
  return [
    {
      selector: `:matches(TSAsExpression, TSTypeAssertion) TSTypeReference[typeName.name=${name}]`,
      message: brandMessage,
    },
    {
      selector: `:matches(TSAsExpression, TSTypeAssertion) TSQualifiedName[right.name=${name}]`,
      message: brandMessage,
    },
    {
      selector: `TSTypePredicate TSTypeReference[typeName.name=${name}]`,
      message: brandMessage,
    },
    {
      selector: `TSTypePredicate TSQualifiedName[right.name=${name}]`,
      message: brandMessage,
    },
    // A type alias may hold a brand inside an object type member's type, as an
    // interface may: { readonly id: RequestId } names a member, not the brand.
    // Every other position (the brand itself, a union or intersection member,
    // a generic argument) is an alias of the brand and is rejected, and so is
    // a reference written directly under an indexed access or a conditional
    // type. The rule sees only the alias's own syntax: taking a member's type
    // back out through a second alias (W["id"] for an alias or interface W, or
    // a generic infer) is not caught, as it is not for an interface.
    {
      selector: `TSTypeAliasDeclaration TSTypeReference[typeName.name=${name}]:not(TSTypeLiteral > TSPropertySignature TSTypeReference)`,
      message: brandMessage,
    },
    {
      selector: `TSTypeAliasDeclaration TSQualifiedName[right.name=${name}]:not(TSTypeLiteral > TSPropertySignature TSQualifiedName)`,
      message: brandMessage,
    },
    {
      selector: `TSTypeAliasDeclaration :matches(TSIndexedAccessType, TSConditionalType) TSTypeReference[typeName.name=${name}]`,
      message: brandMessage,
    },
    {
      selector: `TSTypeAliasDeclaration :matches(TSIndexedAccessType, TSConditionalType) TSQualifiedName[right.name=${name}]`,
      message: brandMessage,
    },
    {
      selector: `ImportSpecifier[imported.name=${name}]:not([local.name=${name}])`,
      message: brandMessage,
    },
    {
      selector: `ExportSpecifier[local.name=${name}]:not([exported.name=${name}])`,
      message: brandMessage,
    },
  ];
}

const brandNames = [
  "CollectedInput",
  "PreparedRequest",
  "RequestId",
  "ArticleKey",
  "UsdAmount",
  "LinkUrl",
];

// Selectors shared by every file. The files-scoped configs below reuse this
// list through syntaxRules because a later config with files/ignores replaces
// the whole rule.
const commonSyntax = [
  {
    selector: `MemberExpression[computed=false][property.name=${htmlParsingPattern}]`,
    message: htmlParsingMessage,
  },
  {
    selector: `MemberExpression[computed=true][property.value=${htmlParsingPattern}]`,
    message: htmlParsingMessage,
  },
  {
    selector: `MemberExpression[computed=true][property.type="TemplateLiteral"][property.quasis.0.value.cooked=${htmlParsingPattern}]`,
    message: htmlParsingMessage,
  },
  {
    // Object keys: Object.assign(el, { innerHTML }) and destructuring.
    selector: `Property[key.name=${htmlParsingPattern}], Property[key.value=${htmlParsingPattern}]`,
    message: htmlParsingMessage,
  },
  {
    selector: `CallExpression[callee.property.name="setAttribute"][arguments.0.value=/^(srcdoc|on)/i]`,
    message: htmlParsingMessage,
  },
  {
    // document.write, window.document.write, el.ownerDocument.write,
    // dotted or computed.
    selector: `MemberExpression:matches([property.name=${documentWritePattern}], [property.value=${documentWritePattern}]):matches([object.name="document"], [object.property.name=/^(document|ownerDocument)$/])`,
    message: htmlParsingMessage,
  },
  {
    selector: 'Identifier[name="DOMParser"]',
    message: htmlParsingMessage,
  },
  {
    // check-dist.ts follows only static imports when it checks that every module
    // dist/ loads is in dist/, so import() is not allowed; dist/ is also checked
    // for it after the build.
    selector: "ImportExpression",
    message: "Use a static import; import() is not allowed in the extension.",
  },
  ...connectNativeSelectors,
  ...chromeSessionSelectors,
];

/**
 * The no-restricted-syntax selectors of a file. Every file gets the common
 * ones; a source file (src/ and scripts/, not tests) also gets the web
 * storage and channel ones. The options name the exceptions one file has:
 * browser/chromeDeps.ts may use storage, windows, sendNativeMessage and the
 * channel, and src/wasmWorker.ts may fetch.
 */
function syntaxRules({
  allowStorage = false,
  allowWindows = false,
  allowFetch = false,
  allowSendNative = false,
  allowChannel = false,
  source = true,
} = {}) {
  return [
    ...commonSyntax,
    ...(source ? sourceStoreSelectors({ allowChannel }) : []),
    ...(allowStorage ? [] : storageSelectors),
    ...networkSelectors(allowFetch ? workerNetworkNames : networkNames),
    ...memberNameSelectors(
      allowWindows
        ? menuPathNames.filter((name) => name !== "windows")
        : menuPathNames,
      menuPathMessage,
    ),
    ...(allowSendNative ? [] : sendNativeSelectors),
  ];
}

// The fixed chrome.storage functions of browser/chromeDeps.ts and the entry
// page scripts that may import each. An import elsewhere fails, so the API key
// is read only by the running viewer and written only by the options page;
// the service worker learns only whether the key item exists. A type-only
// import stays allowed, so a module can take a function as a dependency. Like
// the other rules, this one sees files, not roles: the running viewer and the
// displaying viewer are the same file.
const storageFunctionImporters = {
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
const storageEntryFiles = [
  ...new Set(Object.values(storageFunctionImporters).flat()),
];

/** The no-restricted-imports options that forbid every storage function file may not import. */
function storageImportRule(file) {
  const forbidden = Object.entries(storageFunctionImporters)
    .filter(([, files]) => !files.includes(file))
    .map(([name]) => name);
  return [
    "error",
    {
      patterns: [
        {
          regex: "(^|/)browser/chromeDeps\\.(ts|js)$",
          importNames: forbidden,
          allowTypeImports: true,
          message:
            "Only the entry page scripts listed in eslint.config.js may import this chrome.storage function.",
        },
      ],
    },
  ];
}

/** A config that forbids the listed brands (and the common selectors) in file. */
function creatorConfig(file, own) {
  const forbidden = brandNames.filter((name) => !own.includes(name));
  return {
    files: [file],
    rules: {
      "no-restricted-syntax": [
        "error",
        ...syntaxRules(),
        ...brandSelectors(forbidden),
      ],
    },
  };
}

export default defineConfig(
  { ignores: ["dist/", "dist-dev/", "node_modules/"] },
  js.configs.recommended,
  tseslint.configs.recommended,
  {
    languageOptions: {
      // no-implied-eval recognizes these only as declared globals.
      globals: {
        setTimeout: "readonly",
        setInterval: "readonly",
        window: "readonly",
        self: "readonly",
      },
    },
    rules: {
      // Code built from strings: the extension never evaluates text.
      "no-eval": "error",
      "no-implied-eval": "error",
      "no-new-func": "error",
      "no-restricted-globals": ["error", ...networkGlobals(networkNames)],
      "no-restricted-syntax": [
        "error",
        ...syntaxRules({ allowSendNative: true, source: false }),
      ],
    },
  },
  {
    // Source and scripts must consume every branded value through its type;
    // only the creator file below may assert it. Tests may build fixtures.
    files: ["src/**/*.ts", "scripts/**/*.ts"],
    rules: {
      "no-restricted-syntax": [
        "error",
        ...syntaxRules(),
        ...brandSelectors(brandNames),
      ],
      "no-restricted-globals": ["error", ...sourceGlobals()],
      "no-restricted-imports": storageImportRule(""),
    },
  },
  ...storageEntryFiles.map((file) => ({
    files: [file],
    rules: { "no-restricted-imports": storageImportRule(file) },
  })),
  {
    // The one file that uses chrome.storage, calls
    // chrome.runtime.sendNativeMessage and references chrome.windows.
    files: ["src/browser/chromeDeps.ts"],
    rules: {
      "no-restricted-syntax": [
        "error",
        ...syntaxRules({
          allowStorage: true,
          allowWindows: true,
          allowSendNative: true,
          allowChannel: true,
        }),
        ...brandSelectors(brandNames),
      ],
      "no-restricted-globals": [
        "error",
        ...sourceGlobals({ allowChannel: true }),
      ],
    },
  },
  {
    // The Worker's loader, the one file that may fetch: it loads the article
    // generation module, whose Go code sends to the API.
    files: ["src/wasmWorker.ts"],
    rules: {
      "no-restricted-globals": [
        "error",
        ...sourceGlobals({ allowFetch: true }),
      ],
      "no-restricted-syntax": [
        "error",
        ...syntaxRules({ allowFetch: true }),
        ...brandSelectors(brandNames),
      ],
    },
  },
  creatorConfig("src/core/collect.ts", ["CollectedInput"]),
  creatorConfig("src/core/prepare.ts", ["PreparedRequest"]),
  creatorConfig("src/core/response.ts", ["RequestId", "UsdAmount"]),
  // The extension-only path's cost text; response.ts keeps the server path's
  // until the server is removed.
  creatorConfig("src/core/wasmResult.ts", ["UsdAmount"]),
  creatorConfig("src/core/articleKey.ts", ["ArticleKey"]),
  creatorConfig("src/core/articleMarkdown.ts", ["LinkUrl"]),
);
