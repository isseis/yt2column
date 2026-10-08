import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { checkLockfile } from "../scripts/check-lockfile.ts";

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
  ];
  for (const { name, resolved } of rejected) {
    it(`rejects ${name}`, () => {
      const entry: Record<string, unknown> = { ...registryEntry, resolved };
      if (resolved === undefined) {
        delete entry.resolved;
      }
      const lock = lockfile({
        "": { name: "x" },
        "node_modules/good": registryEntry,
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
