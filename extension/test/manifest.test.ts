import assert from "node:assert/strict";
import { createPublicKey } from "node:crypto";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import { extensionIdFromManifestKey } from "./helpers/extensionId.ts";
import { extensionDir } from "./helpers/paths.ts";

/** The extension ID fixed for this extension and recorded in the design. */
const recordedExtensionId = "clfmbbcdpnjcefbdihdoahomaifbabkk";

const manifest = JSON.parse(
  readFileSync(path.join(extensionDir, "static", "manifest.json"), "utf8"),
) as Record<string, unknown>;

const localesDir = path.join(extensionDir, "static", "_locales");

/** The locale directories under static/_locales, sorted. */
function localeDirs(): string[] {
  return readdirSync(localesDir, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort();
}

/** The regular files directly under dir, sorted. */
function fileNames(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true })
    .filter((entry) => entry.isFile())
    .map((entry) => entry.name)
    .sort();
}

/** Every catalog, by locale directory name. */
function catalogs(): Map<string, Record<string, unknown>> {
  const byLocale = new Map<string, Record<string, unknown>>();
  for (const locale of localeDirs()) {
    const file = path.join(localesDir, locale, "messages.json");
    byLocale.set(
      locale,
      JSON.parse(readFileSync(file, "utf8")) as Record<string, unknown>,
    );
  }
  return byLocale;
}

/** The catalog names the manifest refers to, lower-cased, in field order. */
function manifestMessageNames(): string[] {
  const names: string[] = [];
  for (const field of ["name", "description"]) {
    const value = manifest[field];
    assert.ok(typeof value === "string", field);
    const match = /^__MSG_(.+)__$/.exec(value);
    assert.ok(match !== null, `${field}: ${value}`);
    const name = match[1];
    assert.ok(name !== undefined, field);
    names.push(name.toLowerCase());
  }
  return names;
}

/** The message string of one catalog entry. */
function messageOf(catalog: Record<string, unknown>, name: string): string {
  const entry = catalog[name];
  assert.ok(typeof entry === "object" && entry !== null, name);
  const message = (entry as Record<string, unknown>)["message"];
  assert.ok(typeof message === "string", name);
  return message;
}

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
      "default_locale",
      "description",
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

  it("names and describes the extension through catalog messages", () => {
    assert.equal(manifest["name"], "__MSG_extName__");
    assert.equal(manifest["description"], "__MSG_extDescription__");
  });

  it("gives the extension a version", () => {
    assert.equal(manifest["version"], "0.1.0");
  });
});

describe("locales", () => {
  it("makes en the default locale", () => {
    assert.equal(manifest["default_locale"], "en");
  });

  it("holds exactly the en and ja catalogs, each with only messages.json", () => {
    // Chrome rejects the whole extension when a catalog is missing for
    // default_locale or a locale directory holds another file.
    assert.deepEqual(localeDirs(), ["en", "ja"]);
    for (const locale of localeDirs()) {
      assert.deepEqual(fileNames(path.join(localesDir, locale)), [
        "messages.json",
      ]);
    }
  });

  it("gives every catalog entry a message string without placeholders", () => {
    const byLocale = catalogs();
    assert.deepEqual([...byLocale.keys()].sort(), ["en", "ja"]);
    for (const [locale, catalog] of byLocale) {
      for (const [name, entry] of Object.entries(catalog)) {
        const label = `${locale}/${name}`;
        assert.ok(typeof entry === "object" && entry !== null, label);
        const fields = entry as Record<string, unknown>;
        assert.deepEqual(
          Object.keys(fields).filter(
            (key) => key !== "message" && key !== "description",
          ),
          [],
          label,
        );
        const message = fields["message"];
        assert.ok(typeof message === "string", label);
        // A $ starts a Chrome placeholder; the catalogs use plain strings.
        assert.ok(!message.includes("$"), label);
        if ("description" in fields) {
          assert.ok(typeof fields["description"] === "string", label);
        }
      }
    }
  });

  it("resolves every manifest placeholder in both catalogs", () => {
    const names = manifestMessageNames();
    // Precondition: the manifest refers to its name and description this way.
    assert.equal(names.length, 2);
    for (const name of names) {
      // Chrome accepts only these characters in a message name.
      assert.match(name, /^[a-z0-9_@]+$/, name);
    }
    for (const [locale, catalog] of catalogs()) {
      const keys = Object.keys(catalog).map((key) => key.toLowerCase());
      for (const name of names) {
        assert.ok(keys.includes(name), `${locale}: ${name}`);
      }
    }
  });

  it("names the extension the same way and describes it in each language", () => {
    const byLocale = catalogs();
    assert.deepEqual([...byLocale.keys()].sort(), ["en", "ja"]);
    const descriptions = new Set<string>();
    for (const [locale, catalog] of byLocale) {
      assert.equal(messageOf(catalog, "extName"), "yt2column", locale);
      const description = messageOf(catalog, "extDescription");
      assert.notEqual(description.trim(), "", locale);
      // Chrome rejects a manifest description longer than 132 characters.
      assert.ok(description.length <= 132, locale);
      descriptions.add(description);
    }
    // Each language gets its own description, not the English one copied.
    assert.equal(descriptions.size, 2);
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
