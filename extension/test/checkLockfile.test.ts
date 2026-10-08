import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, it } from "node:test";

import { checkLockfile } from "../scripts/check-lockfile.ts";
import { extensionDir } from "./helpers/paths.ts";

/** Builds a minimal version-3 lockfile wrapping the given packages. */
function lockfile(packages: Record<string, unknown>): unknown {
  return { name: "x", lockfileVersion: 3, requires: true, packages };
}

const registryEntry = {
  version: "1.0.0",
  resolved: "https://registry.npmjs.org/a/-/a-1.0.0.tgz",
  integrity: "sha512-AAAA",
  dev: true,
};

describe("checkLockfile", () => {
  it("accepts a lockfile whose packages all come from the registry", () => {
    const lock = lockfile({
      "": { name: "x", devDependencies: { a: "^1.0.0" } },
      "node_modules/a": registryEntry,
      "node_modules/a/node_modules/b": {
        ...registryEntry,
        resolved: "https://registry.npmjs.org/b/-/b-2.0.0.tgz",
      },
      "node_modules/@s/c": {
        ...registryEntry,
        resolved: "https://registry.npmjs.org/@s/c/-/c-3.0.0.tgz",
      },
    });
    assert.deepEqual(checkLockfile(lock), []);
  });

  const rejected: { name: string; resolved: unknown }[] = [
    { name: "another host", resolved: "https://example.com/a-1.0.0.tgz" },
    {
      name: "a host that extends the registry's",
      resolved: "https://registry.npmjs.org.example.com/a.tgz",
    },
    {
      name: "plain http",
      resolved: "http://registry.npmjs.org/a/-/a-1.0.0.tgz",
    },
    { name: "a git URL", resolved: "git+ssh://git@github.com/x/a.git#0123456" },
    { name: "a local file", resolved: "file:../a" },
    { name: "a missing resolved", resolved: undefined },
    { name: "a non-string resolved", resolved: 1 },
    {
      name: "another package's tarball",
      resolved: "https://registry.npmjs.org/evil/-/evil-1.0.0.tgz",
    },
    {
      name: "a package whose name extends the key's",
      resolved: "https://registry.npmjs.org/ab/-/ab-1.0.0.tgz",
    },
  ];
  for (const { name, resolved } of rejected) {
    it(`rejects ${name}`, () => {
      const entry: Record<string, unknown> = { ...registryEntry, resolved };
      if (resolved === undefined) {
        delete entry.resolved;
      }
      const lock = lockfile({
        "": { name: "x" },
        "node_modules/good": {
          ...registryEntry,
          resolved: "https://registry.npmjs.org/good/-/good-1.0.0.tgz",
        },
        "node_modules/a": entry,
      });
      const violations = checkLockfile(lock);
      assert.equal(violations.length, 1);
      assert.match(violations[0] ?? "", /^node_modules\/a: /);
    });
  }

  it("rejects a lockfile it cannot read as version 3", () => {
    assert.notDeepEqual(checkLockfile(null), []);
    assert.notDeepEqual(
      checkLockfile({ lockfileVersion: 2, packages: {} }),
      [],
    );
    assert.notDeepEqual(checkLockfile({ lockfileVersion: 3 }), []);
    assert.notDeepEqual(
      checkLockfile({ lockfileVersion: 3, packages: null }),
      [],
    );
  });
});

describe("check-lockfile command", () => {
  const script = path.join(extensionDir, "scripts", "check-lockfile.ts");
  const valid = JSON.stringify(
    lockfile({ "": { name: "x" }, "node_modules/a": registryEntry }),
  );

  // Runs the script in a directory holding the given files; returns its exit status.
  function runIn(files: Record<string, string>): number | null {
    const dir = mkdtempSync(path.join(tmpdir(), "check-lockfile-"));
    try {
      for (const [name, content] of Object.entries(files)) {
        writeFileSync(path.join(dir, name), content);
      }
      return spawnSync(process.execPath, [script], { cwd: dir }).status;
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  }

  it("exits 0 for a registry-only lockfile", () => {
    assert.equal(runIn({ "package-lock.json": valid }), 0);
  });

  it("exits 1 for a lockfile with a violation", () => {
    const bad = JSON.stringify(
      lockfile({
        "": { name: "x" },
        "node_modules/a": { ...registryEntry, resolved: "file:../a" },
      }),
    );
    assert.equal(runIn({ "package-lock.json": bad }), 1);
  });

  it("exits 1 when npm-shrinkwrap.json exists", () => {
    assert.equal(
      runIn({ "package-lock.json": valid, "npm-shrinkwrap.json": valid }),
      1,
    );
  });
});
