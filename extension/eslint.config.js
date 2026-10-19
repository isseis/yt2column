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

// chrome.storage.sync would sync the shared token off the device; the
// extension stores it in chrome.storage.local only.
const syncPattern = "/^sync$/";
const syncMessage =
  "Do not use chrome.storage.sync; store settings in chrome.storage.local.";

// The extension sends only from the popup. These APIs served the removed
// context-menu path (the menu, the result window, its session store and the
// toolbar badge). storage.session works under the storage permission the
// extension keeps, and windows and action need no permission, so only this rule
// keeps them from coming back. Like the sync rule, it guards against accidental
// reuse, not deliberate evasion.
const menuPathPattern = "/^(contextMenus|session|windows|setBadgeText)$/";
const menuPathMessage =
  "Do not use chrome.contextMenus, chrome.storage.session, chrome.windows or chrome.action.setBadgeText; the extension sends only from the popup.";

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

const brandNames = [
  "CollectedInput",
  "SharedToken",
  "ServerPort",
  "PreparedRequest",
  "RequestId",
];

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
  {
    // chrome.storage.sync in the same four reference forms. The destructuring
    // form matches any { sync } key by name, so the rule is intentionally
    // name-based and would also reject an unrelated property named sync.
    selector: `MemberExpression[computed=false][property.name=${syncPattern}]`,
    message: syncMessage,
  },
  {
    selector: `MemberExpression[computed=true][property.value=${syncPattern}]`,
    message: syncMessage,
  },
  {
    selector: `MemberExpression[computed=true][property.type="TemplateLiteral"][property.quasis.0.value.cooked=${syncPattern}]`,
    message: syncMessage,
  },
  {
    selector: `Property[key.name=${syncPattern}], Property[key.value=${syncPattern}]`,
    message: syncMessage,
  },
  {
    // The removed menu-path APIs in the same four reference forms, by name like
    // the sync rule above. Type positions (chrome.windows.Window) are type-only
    // references and are not matched.
    selector: `MemberExpression[computed=false][property.name=${menuPathPattern}]`,
    message: menuPathMessage,
  },
  {
    selector: `MemberExpression[computed=true][property.value=${menuPathPattern}]`,
    message: menuPathMessage,
  },
  {
    selector: `MemberExpression[computed=true][property.type="TemplateLiteral"][property.quasis.0.value.cooked=${menuPathPattern}]`,
    message: menuPathMessage,
  },
  {
    selector: `Property[key.name=${menuPathPattern}], Property[key.value=${menuPathPattern}]`,
    message: menuPathMessage,
  },
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
        ...brandSelectors(brandNames),
      ],
    },
  },
  creatorConfig("src/core/collect.ts", ["CollectedInput"]),
  creatorConfig("src/core/settings.ts", ["SharedToken", "ServerPort"]),
  creatorConfig("src/core/prepare.ts", ["PreparedRequest"]),
  creatorConfig("src/core/response.ts", ["RequestId"]),
);
