import assert from "node:assert/strict";
import { createPublicKey } from "node:crypto";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { extensionIdFromManifestKey } from "./helpers/extensionId.ts";
import { extensionDir } from "./helpers/paths.ts";

/** The extension ID fixed for this extension and recorded in the design. */
const recordedExtensionId = "clfmbbcdpnjcefbdihdoahomaifbabkk";

const manifest = JSON.parse(
  readFileSync(path.join(extensionDir, "static", "manifest.json"), "utf8"),
) as Record<string, unknown>;

describe("permissions", () => {
  it("declares exactly the four permissions the design requires", () => {
    assert.deepEqual(manifest["permissions"], [
      "activeTab",
      "contextMenus",
      "scripting",
      "storage",
    ]);
  });
});

describe("undeclared keys", () => {
  const undeclared = [
    "host_permissions",
    "optional_permissions",
    "optional_host_permissions",
    "content_scripts",
    "content_security_policy",
    "web_accessible_resources",
    "externally_connectable",
    "options_ui",
    "commands",
  ];
  for (const key of undeclared) {
    it(`does not declare ${key}`, () => {
      assert.equal(Object.hasOwn(manifest, key), false);
    });
  }
});

describe("fixed fields", () => {
  it("uses manifest version 3", () => {
    assert.equal(manifest["manifest_version"], 3);
  });

  it("requires Chrome 102 for storage.session", () => {
    assert.equal(manifest["minimum_chrome_version"], "102");
  });

  it("loads the service worker as a module", () => {
    assert.deepEqual(manifest["background"], {
      service_worker: "background.js",
      type: "module",
    });
  });

  it("opens popup.html from the toolbar icon", () => {
    assert.deepEqual(manifest["action"], { default_popup: "popup.html" });
  });

  it("names the extension and gives it a version", () => {
    assert.equal(manifest["name"], "yt2column");
    assert.equal(manifest["version"], "0.1.0");
  });
});

describe("key", () => {
  const key = manifest["key"];

  it("is an RSA 2048-bit SubjectPublicKeyInfo", () => {
    assert.ok(typeof key === "string");
    const publicKey = createPublicKey({
      key: Buffer.from(key, "base64"),
      format: "der",
      type: "spki",
    });
    assert.equal(publicKey.asymmetricKeyType, "rsa");
    assert.equal(publicKey.asymmetricKeyDetails?.modulusLength, 2048);
  });

  it("derives the recorded extension ID", () => {
    assert.ok(typeof key === "string");
    assert.equal(extensionIdFromManifestKey(key), recordedExtensionId);
  });
});
