import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { hasExactKeys } from "../src/core/exchange.ts";

describe("hasExactKeys", () => {
  it("accepts a record with exactly the keys, in any order", () => {
    assert.equal(hasExactKeys({ b: 1, a: 2 }, ["a", "b"]), true);
  });

  const rejected: {
    name: string;
    record: Record<string, unknown>;
    keys: string[];
  }[] = [
    { name: "an extra key", record: { a: 1, b: 2, c: 3 }, keys: ["a", "b"] },
    { name: "a missing key", record: { a: 1 }, keys: ["a", "b"] },
    {
      name: "a different key of the same count",
      record: { a: 1, c: 2 },
      keys: ["a", "b"],
    },
    {
      // The inherited b is not an own key, so the record has one key, not two;
      // the own c keeps the count equal, so only the own-key check rejects it.
      name: "an inherited key",
      record: Object.assign(Object.create({ b: 1 }) as object, { c: 2 }),
      keys: ["b"],
    },
  ];
  for (const { name, record, keys } of rejected) {
    it(`rejects ${name}`, () => {
      assert.equal(hasExactKeys(record, keys), false);
    });
  }
});
