import assert from "node:assert/strict";
import { createPublicKey } from "node:crypto";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { extensionIdFromManifestKey } from "./helpers/extensionId.ts";
import { extensionDir } from "./helpers/paths.ts";
import { markdownSection, readRepoFile } from "./helpers/repoFile.ts";

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

  it("are listed under the requested permissions in security.md 8.1", () => {
    const body = markdownSection(
      readRepoFile("docs/dev/security.md"),
      "### 8.1. ブラウザ拡張（#110）",
    );
    assert.notEqual(body, "", "security.md has no 8.1 extension section");
    const bullet = body
      .split("\n")
      .find((line) => line.startsWith("- **要求する権限**"));
    assert.ok(bullet !== undefined, "security.md 8.1 has no 要求する権限");
    // Only the first sentence lists what is requested; the next one names
    // what is not. A full stop inside a parenthesised reason does not end it.
    let depth = 0;
    let end = bullet.length;
    for (let i = 0; i < bullet.length; i += 1) {
      const c = bullet[i];
      if (c === "（") depth += 1;
      else if (c === "）") depth = Math.max(0, depth - 1);
      else if (c === "。" && depth === 0) {
        end = i;
        break;
      }
    }
    const requested = bullet.slice(0, end);
    const permissions = manifest["permissions"];
    assert.ok(Array.isArray(permissions));
    for (const permission of permissions) {
      assert.ok(typeof permission === "string");
      assert.ok(
        requested.includes(`\`${permission}\``),
        `security.md 8.1 does not list the permission ${permission}`,
      );
    }
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
