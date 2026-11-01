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

/** Every entry name directly under dir, sorted, whatever its kind. */
function entryNames(dir: string): string[] {
  return readdirSync(dir).sort();
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

/** Every __MSG_<name>__ name in the manifest's strings, lower-cased. */
function manifestMessageNames(): Set<string> {
  const names = new Set<string>();
  const visit = (value: unknown): void => {
    if (typeof value === "string") {
      for (const [text] of value.matchAll(/__MSG_(.+?)__/g)) {
        names.add(
          text.slice("__MSG_".length, text.length - "__".length).toLowerCase(),
        );
      }
    } else if (typeof value === "object" && value !== null) {
      for (const child of Object.values(value)) {
        visit(child);
      }
    }
  };
  visit(manifest);
  return names;
}

/** Chrome accepts only ASCII letters and digits, `_` and `@`, and reserves @@. */
function assertValidMessageName(name: string): void {
  assert.match(name, /^[A-Za-z0-9_@]+$/, name);
  assert.ok(!name.startsWith("@@"), name);
}

/** True when text contains Japanese script: hiragana, katakana or kanji. */
function hasJapaneseScript(text: string): boolean {
  return /[\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Han}]/u.test(text);
}

/** The message string of one catalog entry. */
function messageOf(
  catalog: Record<string, unknown>,
  name: string,
  locale: string,
): string {
  const label = `${locale}/${name}`;
  const entry = catalog[name];
  assert.ok(typeof entry === "object" && entry !== null, label);
  const message = (entry as Record<string, unknown>)["message"];
  assert.ok(typeof message === "string", label);
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
      "background",
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

  it("requires Chrome 110", () => {
    // From Chrome 110, extension API calls and events extend the service
    // worker's idle timer, which the start's 25-second exchange relies on.
    assert.equal(manifest["minimum_chrome_version"], "110");
  });

  it("runs background.js as a module service worker", () => {
    assert.deepEqual(manifest["background"], {
      service_worker: "background.js",
      type: "module",
    });
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
    // The design requires exactly these two locale directories, each holding
    // only its messages.json.
    const locales = localeDirs();
    assert.deepEqual(locales, ["en", "ja"]);
    for (const locale of locales) {
      assert.deepEqual(entryNames(path.join(localesDir, locale)), [
        "messages.json",
      ]);
    }
  });

  it("gives every catalog entry a message string and no unexpected fields, without placeholders", () => {
    const byLocale = catalogs();
    // Precondition: the walk reached the catalogs.
    assert.ok(byLocale.size > 0, "found no catalogs");
    for (const [locale, catalog] of byLocale) {
      assert.notEqual(Object.keys(catalog).length, 0, locale);
      for (const [name, entry] of Object.entries(catalog)) {
        const label = `${locale}/${name}`;
        assertValidMessageName(name);
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

  it("resolves every manifest placeholder in both catalogs under Chrome's name rule", () => {
    const names = manifestMessageNames();
    // Precondition: the scan reached the manifest's catalog references.
    assert.ok(names.size > 0, "the manifest refers to catalog messages");
    for (const name of names) {
      assertValidMessageName(name);
    }
    const byLocale = catalogs();
    assert.ok(byLocale.size > 0, "found no catalogs");
    for (const [locale, catalog] of byLocale) {
      const keys = Object.keys(catalog).map((key) => key.toLowerCase());
      for (const name of names) {
        assert.ok(keys.includes(name), `${locale}: ${name}`);
      }
    }
  });

  it("names the extension the same way and describes it in each language", () => {
    const byLocale = catalogs();
    // Precondition: the walk reached the catalogs.
    assert.ok(byLocale.size > 0, "found no catalogs");
    const descriptions = new Set<string>();
    for (const [locale, catalog] of byLocale) {
      assert.equal(messageOf(catalog, "extName", locale), "yt2column", locale);
      const description = messageOf(catalog, "extDescription", locale);
      assert.notEqual(description.trim(), "", locale);
      // Chrome rejects a manifest description longer than 132 characters.
      assert.ok(description.length <= 132, locale);
      // The description is written in its own language: Japanese has
      // Japanese script, English has none.
      assert.equal(hasJapaneseScript(description), locale === "ja", locale);
      descriptions.add(description.trim());
    }
    // Each language gets its own description, not the English one copied.
    assert.equal(descriptions.size, byLocale.size);
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
