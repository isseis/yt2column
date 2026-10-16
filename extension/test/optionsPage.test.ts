import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { JSDOM } from "jsdom";

import type { ServerPort, SharedToken } from "../src/core/settings.ts";
import type { SettingsStore } from "../src/core/settings.ts";
import { runOptionsPage } from "../src/optionsPage.ts";
import { identifiableToken } from "./helpers/settings.ts";
import { recordingLogger } from "./helpers/logger.ts";

const validToken = "a".repeat(64);

/** A fresh options container in its own document. */
function container(): HTMLElement {
  const dom = new JSDOM("<!DOCTYPE html><body><div id='root'></div></body>");
  const element = dom.window.document.getElementById("root");
  assert.ok(element !== null);
  return element;
}

/** The one element matching selector, asserting it exists. */
function query<T extends Element>(root: HTMLElement, selector: string): T {
  const element = root.querySelector(selector);
  assert.ok(element !== null, selector);
  return element as T;
}

/** Clicks a button and waits for the async handler to settle. */
async function click(button: HTMLButtonElement): Promise<void> {
  button.click();
  await new Promise((resolve) => setTimeout(resolve, 0));
}

interface FakeSettingsOptions {
  readonly token?: unknown;
  readonly port?: unknown;
  readonly loadFails?: unknown;
  readonly saveTokenFails?: unknown;
  readonly clearTokenFails?: unknown;
  readonly savePortFails?: unknown;
}

interface FakeSettings {
  readonly store: SettingsStore;
  readonly calls: {
    readonly saveToken: SharedToken[];
    readonly savePort: ServerPort[];
    readonly clearToken: number;
  };
}

/** A stateful SettingsStore that records calls and can fail on demand. */
function fakeSettings(options: FakeSettingsOptions = {}): FakeSettings {
  const stored = { token: options.token, port: options.port };
  const calls = {
    saveToken: [] as SharedToken[],
    savePort: [] as ServerPort[],
    clearToken: 0,
  };
  return {
    store: {
      async load() {
        if (options.loadFails !== undefined) {
          throw options.loadFails;
        }
        return { token: stored.token, port: stored.port };
      },
      async saveToken(token) {
        if (options.saveTokenFails !== undefined) {
          throw options.saveTokenFails;
        }
        calls.saveToken.push(token);
        stored.token = token;
      },
      async clearToken() {
        if (options.clearTokenFails !== undefined) {
          throw options.clearTokenFails;
        }
        calls.clearToken += 1;
        stored.token = undefined;
      },
      async savePort(port) {
        if (options.savePortFails !== undefined) {
          throw options.savePortFails;
        }
        calls.savePort.push(port);
        stored.port = port;
      },
    },
    calls,
  };
}

describe("runOptionsPage", () => {
  it("builds the token and port fields with the expected attributes", async () => {
    const { store } = fakeSettings();
    const root = container();
    await runOptionsPage(root, { store, log: recordingLogger().log });

    const token = query<HTMLInputElement>(root, ".options-token");
    assert.equal(token.type, "password");
    assert.equal(token.getAttribute("autocomplete"), "new-password");
    assert.equal(token.getAttribute("spellcheck"), "false");
    const port = query<HTMLInputElement>(root, ".options-port");
    assert.equal(port.type, "text");
    assert.equal(port.getAttribute("inputmode"), "numeric");
  });

  it("distinguishes the set, missing and invalid stored-token states", async () => {
    const texts: string[] = [];
    for (const token of [undefined, validToken, "not-a-token"]) {
      const root = container();
      await runOptionsPage(root, {
        store: fakeSettings({ token }).store,
        log: recordingLogger().log,
      });
      const status = query(root, ".options-token-status").textContent ?? "";
      assert.notEqual(status, "");
      texts.push(status);
    }
    assert.equal(new Set(texts).size, 3);
  });

  it("saves an accepted token, clears the field and shows it is set", async () => {
    const settings = fakeSettings();
    const root = container();
    await runOptionsPage(root, {
      store: settings.store,
      log: recordingLogger().log,
    });
    const tokenInput = query<HTMLInputElement>(root, ".options-token");
    tokenInput.value = validToken;
    await click(query<HTMLButtonElement>(root, ".options-token-save"));

    assert.deepEqual(settings.calls.saveToken, [validToken]);
    assert.equal(tokenInput.value, "");
    assert.notEqual(query(root, ".options-token-status").textContent, "");
  });

  const rejectedTokens: { name: string; value: string }[] = [
    { name: "an empty string", value: "" },
    { name: "63 characters", value: "a".repeat(63) },
    { name: "65 characters", value: "a".repeat(65) },
    { name: "an uppercase letter", value: "A".repeat(64) },
    { name: "a g", value: `g${"a".repeat(63)}` },
    { name: "a leading space", value: ` ${validToken}` },
    { name: "full-width characters", value: "\uFF41".repeat(64) },
  ];
  for (const { name, value } of rejectedTokens) {
    it(`rejects a token with ${name} without saving or clearing the field`, async () => {
      const settings = fakeSettings();
      const root = container();
      await runOptionsPage(root, {
        store: settings.store,
        log: recordingLogger().log,
      });
      const status = query(root, ".options-token-status");
      const before = status.textContent;
      const tokenInput = query<HTMLInputElement>(root, ".options-token");
      tokenInput.value = value;
      await click(query<HTMLButtonElement>(root, ".options-token-save"));

      assert.equal(settings.calls.saveToken.length, 0);
      assert.equal(tokenInput.value, value);
      // A rejection replaces the state text with a reason.
      assert.notEqual(status.textContent, before);
      assert.notEqual(status.textContent, "");
    });
  }

  it("clears the token and leaves the port untouched", async () => {
    const settings = fakeSettings({ token: validToken, port: "1024" });
    const root = container();
    await runOptionsPage(root, {
      store: settings.store,
      log: recordingLogger().log,
    });
    await click(query<HTMLButtonElement>(root, ".options-token-clear"));

    assert.equal(settings.calls.clearToken, 1);
    assert.notEqual(query(root, ".options-token-status").textContent, "");
    assert.deepEqual(settings.calls.savePort, []);
    assert.equal(query<HTMLInputElement>(root, ".options-port").value, "1024");
  });

  it("shows a stored port in the field", async () => {
    const { store } = fakeSettings({ port: "1024" });
    const root = container();
    await runOptionsPage(root, { store, log: recordingLogger().log });
    assert.equal(query<HTMLInputElement>(root, ".options-port").value, "1024");
  });

  it("shows the default note and an empty field when no port is stored", async () => {
    const { store } = fakeSettings();
    const root = container();
    await runOptionsPage(root, { store, log: recordingLogger().log });
    assert.equal(query<HTMLInputElement>(root, ".options-port").value, "");
    assert.notEqual(query(root, ".options-port-status").textContent, "");
  });

  it("shows the invalid note, not the value, when the stored port is invalid", async () => {
    const { store } = fakeSettings({ port: "80" });
    const root = container();
    await runOptionsPage(root, { store, log: recordingLogger().log });
    assert.equal(query<HTMLInputElement>(root, ".options-port").value, "");
    const status = query(root, ".options-port-status").textContent ?? "";
    assert.notEqual(status, "");
    assert.equal(status.includes("80"), false);
  });

  it("saves an accepted port", async () => {
    const settings = fakeSettings();
    const root = container();
    await runOptionsPage(root, {
      store: settings.store,
      log: recordingLogger().log,
    });
    const portInput = query<HTMLInputElement>(root, ".options-port");
    portInput.value = "65535";
    await click(query<HTMLButtonElement>(root, ".options-port-save"));
    assert.deepEqual(settings.calls.savePort, ["65535"]);
  });

  const rejectedPorts = [
    "",
    "0",
    "80",
    "1023",
    "65536",
    "08765",
    "+8765",
    " 8765",
    "8765 ",
    "0x2235",
    "8765.0",
  ];
  for (const value of rejectedPorts) {
    it(`rejects the port ${JSON.stringify(value)} without saving`, async () => {
      const settings = fakeSettings();
      const root = container();
      await runOptionsPage(root, {
        store: settings.store,
        log: recordingLogger().log,
      });
      const status = query(root, ".options-port-status");
      const before = status.textContent;
      query<HTMLInputElement>(root, ".options-port").value = value;
      await click(query<HTMLButtonElement>(root, ".options-port-save"));
      assert.equal(settings.calls.savePort.length, 0);
      // A rejection replaces the state text with a reason.
      assert.notEqual(status.textContent, before);
      assert.notEqual(status.textContent, "");
    });
  }

  it("does not discard an unsaved port entry when the token is saved", async () => {
    const settings = fakeSettings();
    const root = container();
    await runOptionsPage(root, {
      store: settings.store,
      log: recordingLogger().log,
    });
    query<HTMLInputElement>(root, ".options-port").value = "9999";
    query<HTMLInputElement>(root, ".options-token").value = validToken;
    await click(query<HTMLButtonElement>(root, ".options-token-save"));
    assert.equal(query<HTMLInputElement>(root, ".options-port").value, "9999");
  });

  it("reports a load failure on both sections and logs it", async () => {
    const recorder = recordingLogger();
    const { store } = fakeSettings({ loadFails: new Error("boom") });
    const root = container();
    await runOptionsPage(root, { store, log: recorder.log });
    assert.equal(recorder.errors.length, 1);
    assert.equal(recorder.infos.length, 0);
    assert.notEqual(query(root, ".options-token-status").textContent, "");
    assert.notEqual(query(root, ".options-port-status").textContent, "");
  });

  const saveFailures: {
    name: string;
    options: FakeSettingsOptions;
    selector: string;
  }[] = [
    {
      name: "a token save",
      options: { token: validToken, saveTokenFails: new Error("boom") },
      selector: ".options-token-save",
    },
    {
      name: "a token clear",
      options: { token: validToken, clearTokenFails: new Error("boom") },
      selector: ".options-token-clear",
    },
    {
      name: "a port save",
      options: { port: "1024", savePortFails: new Error("boom") },
      selector: ".options-port-save",
    },
  ];
  for (const { name, options, selector } of saveFailures) {
    it(`reports ${name} failure and logs it`, async () => {
      const recorder = recordingLogger();
      const settings = fakeSettings(options);
      const root = container();
      await runOptionsPage(root, { store: settings.store, log: recorder.log });
      if (selector === ".options-token-save") {
        query<HTMLInputElement>(root, ".options-token").value = validToken;
      }
      if (selector === ".options-port-save") {
        query<HTMLInputElement>(root, ".options-port").value = "65535";
      }
      await click(query<HTMLButtonElement>(root, selector));

      assert.equal(recorder.errors.length, 1);
      assert.equal(recorder.infos.length, 0);
    });
  }

  it("never puts the token value in the log, the display or a field", async () => {
    const recorder = recordingLogger();
    const settings = fakeSettings({ token: identifiableToken });
    const root = container();
    await runOptionsPage(root, { store: settings.store, log: recorder.log });
    const tokenInput = query<HTMLInputElement>(root, ".options-token");
    tokenInput.value = identifiableToken;
    await click(query<HTMLButtonElement>(root, ".options-token-save"));

    const serialized = JSON.stringify({
      infos: recorder.infos,
      errors: recorder.errors,
      text: root.textContent,
      tokenField: tokenInput.value,
      portField: query<HTMLInputElement>(root, ".options-port").value,
    });
    // The whole value and even its first or last few characters must not appear.
    assert.equal(serialized.includes(identifiableToken), false);
    assert.equal(serialized.includes(identifiableToken.slice(0, 4)), false);
    assert.equal(serialized.includes(identifiableToken.slice(-4)), false);
  });
});
