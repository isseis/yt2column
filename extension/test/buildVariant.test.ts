import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

import {
  buildKindSource,
  devManifest,
  parseBuildKind,
} from "../scripts/build-variant.ts";
import { extensionDir } from "./helpers/paths.ts";

describe("buildKindSource", () => {
  it("is the text of static/buildKind.js for release, which the release build copies", () => {
    assert.equal(
      readFileSync(path.join(extensionDir, "static", "buildKind.js"), "utf8"),
      buildKindSource("release"),
    );
  });
});

describe("parseBuildKind", () => {
  for (const kind of ["release", "dev"]) {
    it(`accepts ${kind}`, () => {
      assert.equal(parseBuildKind(kind), kind);
    });
  }
  for (const word of ["", "Release", "development", " dev"]) {
    it(`rejects ${JSON.stringify(word)}`, () => {
      assert.throws(() => parseBuildKind(word));
    });
  }
});

describe("devManifest", () => {
  const release = {
    manifest_version: 3,
    name: "__MSG_extName__",
    key: "a-key",
    permissions: ["activeTab"],
    action: { default_popup: "popup.html" },
  };

  it("drops the key and names the development build, keeping every other member", () => {
    assert.deepEqual(JSON.parse(devManifest(JSON.stringify(release))), {
      manifest_version: 3,
      name: "__MSG_extNameDev__",
      permissions: ["activeTab"],
      action: { default_popup: "popup.html" },
    });
  });

  it("transforms the shipped manifest, whose name both catalogs resolve", () => {
    const text = readFileSync(
      path.join(extensionDir, "static", "manifest.json"),
      "utf8",
    );
    const manifest = JSON.parse(devManifest(text)) as Record<string, unknown>;
    assert.equal("key" in manifest, false);
    for (const locale of ["en", "ja"]) {
      const catalog = JSON.parse(
        readFileSync(
          path.join(
            extensionDir,
            "static",
            "_locales",
            locale,
            "messages.json",
          ),
          "utf8",
        ),
      ) as Record<string, { message: string }>;
      const name = catalog["extNameDev"]?.message ?? "";
      // The development name is told apart from the release name and starts
      // with the core product name of its own language.
      assert.notEqual(name, catalog["extName"]?.message, locale);
      assert.ok(
        name.startsWith(locale === "ja" ? "ヨムドウガ" : "YomuDoga"),
        locale,
      );
    }
  });

  const rejected: { name: string; manifest: unknown }[] = [
    { name: "no key", manifest: { ...release, key: undefined } },
    { name: "another name", manifest: { ...release, name: "yt2column" } },
    { name: "an array", manifest: [release] },
  ];
  for (const { name, manifest } of rejected) {
    it(`rejects a manifest with ${name}`, () => {
      assert.throws(() => devManifest(JSON.stringify(manifest)));
    });
  }
});
