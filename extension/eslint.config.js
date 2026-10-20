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

// The extension has no storage permission and keeps no settings. The type
// declarations still contain chrome.storage, so tsc would accept a reference
// the browser then fails at run time; this rule rejects it. The destructuring
// form matches any { storage } key by name, so the rule is intentionally
// name-based and would also reject an unrelated property named storage.
const storageSelectors = memberNameSelectors(
  ["storage"],
  "Do not use chrome.storage; the extension has no storage permission and keeps no settings.",
);

// The only connection the extension makes is one chrome.runtime.sendNativeMessage
// call in browser/chromeDeps.ts. These are the other ways to reach a network
// or a long-lived host connection. Like the storage rule, they guard against
// accidental reuse, not deliberate evasion.
const networkNames = ["fetch", "XMLHttpRequest", "WebSocket", "EventSource"];
const networkMessage =
  "Do not open connections; the extension only calls chrome.runtime.sendNativeMessage in browser/chromeDeps.ts.";
const networkGlobals = networkNames.map((name) => ({
  name,
  message: networkMessage,
}));
const networkPattern = `/^(${networkNames.join("|")})$/`;
const globalObject = "/^(window|self|globalThis)$/";
const networkSelectors = [
  {
    // window.fetch, self.fetch and globalThis.fetch, dotted or computed.
    selector: `MemberExpression:matches([property.name=${networkPattern}], [property.value=${networkPattern}])[object.name=${globalObject}]`,
    message: networkMessage,
  },
  {
    selector: `MemberExpression[computed=true][property.type="TemplateLiteral"][property.quasis.0.value.cooked=${networkPattern}][object.name=${globalObject}]`,
    message: networkMessage,
  },
  {
    // const { fetch } = window;
    selector: `VariableDeclarator[init.name=${globalObject}] > ObjectPattern > Property:matches([key.name=${networkPattern}], [key.value=${networkPattern}])`,
    message: networkMessage,
  },
];
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

// The extension sends only from the popup. These APIs served the removed
// context-menu path (the menu, the result window, its session store and the
// toolbar badge). windows and action need no permission, so only this rule
// keeps them from coming back. Like the storage rule, it guards against accidental
// reuse, not deliberate evasion.
const menuPathNames = ["contextMenus", "session", "windows", "setBadgeText"];
const menuPathMessage =
  "Do not use chrome.contextMenus, chrome.windows or chrome.action.setBadgeText; the extension sends only from the popup.";

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
    {
      selector: `TSTypeAliasDeclaration TSTypeReference[typeName.name=${name}]`,
      message: brandMessage,
    },
    {
      selector: `TSTypeAliasDeclaration TSQualifiedName[right.name=${name}]`,
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

const brandNames = ["CollectedInput", "PreparedRequest", "RequestId"];

// Selectors shared by every file. The files-scoped config below reuses this
// list because a later config with files/ignores replaces the whole rule.
const restrictedSyntax = [
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
  ...storageSelectors,
  ...networkSelectors,
  ...connectNativeSelectors,
  ...memberNameSelectors(menuPathNames, menuPathMessage),
];

/** A config that forbids the listed brands (and the common selectors) in file. */
function creatorConfig(file, own) {
  const forbidden = brandNames.filter((name) => !own.includes(name));
  return {
    files: [file],
    rules: {
      "no-restricted-syntax": [
        "error",
        ...restrictedSyntax,
        ...sendNativeSelectors,
        ...brandSelectors(forbidden),
      ],
    },
  };
}

export default defineConfig(
  { ignores: ["dist/", "node_modules/"] },
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
      "no-restricted-globals": ["error", ...networkGlobals],
      "no-restricted-syntax": ["error", ...restrictedSyntax],
    },
  },
  {
    // Source and scripts must consume every branded value through its type;
    // only the creator file below may assert it. Tests may build fixtures.
    files: ["src/**/*.ts", "scripts/**/*.ts"],
    rules: {
      "no-restricted-syntax": [
        "error",
        ...restrictedSyntax,
        ...sendNativeSelectors,
        ...brandSelectors(brandNames),
      ],
    },
  },
  {
    // The one file that calls chrome.runtime.sendNativeMessage.
    files: ["src/browser/chromeDeps.ts"],
    rules: {
      "no-restricted-syntax": [
        "error",
        ...restrictedSyntax,
        ...brandSelectors(brandNames),
      ],
    },
  },
  creatorConfig("src/core/collect.ts", ["CollectedInput"]),
  creatorConfig("src/core/prepare.ts", ["PreparedRequest"]),
  creatorConfig("src/core/response.ts", ["RequestId"]),
);
