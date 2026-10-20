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
  it("declares exactly the three permissions the design requires", () => {
    // No storage: nothing is stored, so chrome.storage is never needed.
    assert.deepEqual(manifest["permissions"], [
      "activeTab",
      "scripting",
      "nativeMessaging",
    ]);
  });
});

describe("undeclared keys", () => {
  it("declares exactly the keys the design lists", () => {
    // No host_permissions (the host is not reached over the network) and no
    // options_ui (there is nothing to set).
    assert.deepEqual(Object.keys(manifest).sort(), [
      "action",
      "key",
      "manifest_version",
      "minimum_chrome_version",
      "name",
      "permissions",
      "version",
    ]);
  });
});

describe("fixed fields", () => {
  it("uses manifest version 3", () => {
    assert.equal(manifest["manifest_version"], 3);
  });

  it("requires Chrome 103", () => {
    assert.equal(manifest["minimum_chrome_version"], "103");
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

  it("rejects a value that is not a public key", () => {
    assert.throws(() => extensionIdFromManifestKey("AAAA"));
  });
});
