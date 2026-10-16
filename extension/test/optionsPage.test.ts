import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { ServerPort, SharedToken } from "../src/core/settings.ts";
import type { SettingsStore } from "../src/core/settings.ts";
import { runOptionsPage } from "../src/optionsPage.ts";
import { container } from "./helpers/dom.ts";
import { recordingLogger } from "./helpers/logger.ts";
import { identifiableToken } from "./helpers/settings.ts";

const validToken = "a".repeat(64);

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
    const texts = new Map<string, string>();
    for (const [state, token] of [
      ["missing", undefined],
      ["set", validToken],
      ["invalid", "not-a-token"],
    ] as const) {
      const root = container();
      await runOptionsPage(root, {
        store: fakeSettings({ token }).store,
        log: recordingLogger().log,
      });
      const text = query(root, ".options-token-status").textContent ?? "";
      assert.notEqual(text, "");
      texts.set(state, text);
    }
    assert.notEqual(texts.get("set"), texts.get("missing"));
    assert.notEqual(texts.get("invalid"), texts.get("missing"));
    assert.notEqual(texts.get("invalid"), texts.get("set"));
    // The set state never shows the stored value.
    assert.equal(texts.get("set")?.includes(validToken), false);
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

  // Two representatives: the wrong length and the wrong characters. The full
  // AC-02 input table is parseToken's contract, covered by settings.test.ts.
  const rejectedTokens: { name: string; value: string }[] = [
    { name: "63 characters", value: "a".repeat(63) },
    { name: "an uppercase letter", value: "A".repeat(64) },
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

  // Two representatives: the wrong range and the wrong characters. The full
  // AC-03 input table is parsePort's contract, covered by settings.test.ts.
  const rejectedPorts = ["80", "08765"];
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

  it("does not log the token when a save fails", async () => {
    const recorder = recordingLogger();
    const settings = fakeSettings({ saveTokenFails: new Error("boom") });
    const root = container();
    await runOptionsPage(root, { store: settings.store, log: recorder.log });
    query<HTMLInputElement>(root, ".options-token").value = identifiableToken;
    await click(query<HTMLButtonElement>(root, ".options-token-save"));

    assert.equal(recorder.errors.length, 1);
    const logged = JSON.stringify({
      infos: recorder.infos,
      errors: recorder.errors,
      text: root.textContent,
    });
    assert.equal(logged.includes(identifiableToken), false);
  });
});
