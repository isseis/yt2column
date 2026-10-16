import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  checkSettings,
  defaultPort,
  parsePort,
  parseToken,
} from "../src/core/settings.ts";

const validToken = "a".repeat(64);

describe("parseToken", () => {
  const accepted: { name: string; value: string }[] = [
    { name: "a repeating hex pattern", value: "0123456789abcdef".repeat(4) },
    { name: "the all-a token", value: validToken },
  ];
  for (const { name, value } of accepted) {
    it(`accepts ${name} unchanged`, () => {
      assert.equal(parseToken(value), value);
    });
  }

  const rejected: { name: string; value: unknown }[] = [
    { name: "an empty string", value: "" },
    { name: "63 characters", value: "a".repeat(63) },
    { name: "65 characters", value: "a".repeat(65) },
    { name: "an uppercase letter", value: "A".repeat(64) },
    { name: "a g", value: `g${"a".repeat(63)}` },
    { name: "a leading space", value: ` ${validToken}` },
    { name: "a trailing newline", value: `${validToken}\n` },
    { name: "a trailing space", value: `${validToken} ` },
    { name: "a leading newline", value: `\n${validToken}` },
    { name: "full-width characters", value: "\uFF41".repeat(64) },
    { name: "a full-width digit", value: `\uFF10${"a".repeat(63)}` },
    { name: "a number", value: 1 },
    { name: "null", value: null },
    { name: "an object", value: {} },
  ];
  for (const { name, value } of rejected) {
    it(`rejects ${name}`, () => {
      assert.equal(parseToken(value), undefined);
    });
  }
});

describe("parsePort", () => {
  const accepted = ["1024", "8765", "65535"];
  for (const port of accepted) {
    it(`accepts ${port} unchanged`, () => {
      assert.equal(parsePort(port), port);
    });
  }

  const rejected: { name: string; value: unknown }[] = [
    { name: "an empty string", value: "" },
    { name: "zero", value: "0" },
    { name: "80", value: "80" },
    { name: "1023", value: "1023" },
    { name: "65536", value: "65536" },
    { name: "a leading zero", value: "08765" },
    { name: "a leading plus", value: "+8765" },
    { name: "a leading space", value: " 8765" },
    { name: "a trailing space", value: "8765 " },
    { name: "a hex prefix", value: "0x2235" },
    { name: "a decimal point", value: "8765.0" },
    { name: "full-width digits", value: "\uFF18\uFF17\uFF16\uFF15" },
    { name: "a number", value: 8765 },
  ];
  for (const { name, value } of rejected) {
    it(`rejects ${name}`, () => {
      assert.equal(parsePort(value), undefined);
    });
  }
});

describe("checkSettings", () => {
  it("uses the default port when none is stored", () => {
    const check = checkSettings({ token: validToken, port: undefined });
    assert.deepEqual(check, {
      kind: "ready",
      token: validToken,
      port: defaultPort,
    });
  });

  it("reports a missing token before anything else", () => {
    assert.deepEqual(checkSettings({ token: undefined, port: undefined }), {
      kind: "blocked",
      reason: "token-missing",
    });
  });

  it("reports an invalid token", () => {
    assert.deepEqual(checkSettings({ token: "x", port: undefined }), {
      kind: "blocked",
      reason: "token-invalid",
    });
  });

  it("reports an invalid stored port without replacing it with the default", () => {
    assert.deepEqual(checkSettings({ token: validToken, port: "80" }), {
      kind: "blocked",
      reason: "port-invalid",
    });
  });

  it("accepts a valid token and port", () => {
    assert.deepEqual(checkSettings({ token: validToken, port: "1024" }), {
      kind: "ready",
      token: validToken,
      port: "1024",
    });
  });
});
