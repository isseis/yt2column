// @ts-check
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
      "no-restricted-syntax": [
        "error",
        {
          selector: `MemberExpression[computed=false][property.name=${htmlParsingPattern}]`,
          message: htmlParsingMessage,
        },
        {
          selector: `MemberExpression[computed=true][property.value=${htmlParsingPattern}]`,
          message: htmlParsingMessage,
        },
        {
          selector: `MemberExpression[property.name=${documentWritePattern}][object.name="document"]`,
          message: htmlParsingMessage,
        },
        {
          selector: `MemberExpression[property.name=${documentWritePattern}][object.property.name="ownerDocument"]`,
          message: htmlParsingMessage,
        },
        {
          selector: 'Identifier[name="DOMParser"]',
          message: htmlParsingMessage,
        },
        {
          // A Manifest V3 service worker rejects import(); dist/ is also
          // checked for it after the build.
          selector: "ImportExpression",
          message:
            "Use a static import; import() is not allowed in the extension.",
        },
      ],
    },
  },
);
