// The options page: the shared token and the server port. The token is saved
// only when it matches the accepted form, and its value is never shown again
// (only whether one is set); nothing is trimmed or rewritten. The form is
// built here so the page's HTML stays an empty shell and the tests can run
// without reading options.html.

import { parsePort, parseToken } from "./core/settings.ts";
import type { SettingsStore } from "./core/settings.ts";
import type { Logger } from "./launch.ts";

export interface OptionsPageDeps {
  readonly store: SettingsStore;
  readonly log: Logger;
}

const tokenSetText = "A shared token is set.";
const tokenMissingText = "No shared token is set.";
const tokenInvalidText =
  "The stored shared token does not have the right form; paste the token again.";
const tokenRejectedText =
  "Enter the 64 lowercase hexadecimal characters of the shared token, with no surrounding spaces or newlines.";
const tokenSaveFailedText = "The token could not be saved.";
const tokenClearFailedText = "The token could not be cleared.";
const portDefaultText =
  "No port is stored; the default 8765 is used when sending.";
const portInvalidText =
  "The stored server port does not have the right form; sending is blocked until it is set again.";
const portRejectedText =
  "Enter the server port: a whole number from 1024 to 65535.";
const portSaveFailedText = "The port could not be saved.";
const settingsFailedText = "The settings could not be read. Reopen this page.";

const loadLabel = "yt2column load settings";
const saveTokenLabel = "yt2column save token";
const clearTokenLabel = "yt2column clear token";
const savePortLabel = "yt2column save port";

/** Builds the options form into container and wires its actions. Never throws. */
export async function runOptionsPage(
  container: HTMLElement,
  deps: OptionsPageDeps,
): Promise<void> {
  const doc = container.ownerDocument;

  const tokenLabel = doc.createElement("label");
  tokenLabel.textContent = "Shared token";
  const tokenInput = doc.createElement("input");
  tokenInput.type = "password";
  tokenInput.className = "options-token";
  tokenInput.setAttribute("autocomplete", "new-password");
  tokenInput.setAttribute("spellcheck", "false");
  const tokenSave = doc.createElement("button");
  tokenSave.type = "button";
  tokenSave.className = "options-token-save";
  tokenSave.textContent = "Save token";
  const tokenClear = doc.createElement("button");
  tokenClear.type = "button";
  tokenClear.className = "options-token-clear";
  tokenClear.textContent = "Clear token";
  const tokenStatus = doc.createElement("p");
  tokenStatus.className = "options-token-status";

  const portLabel = doc.createElement("label");
  portLabel.textContent = "Server port";
  const portInput = doc.createElement("input");
  portInput.type = "text";
  portInput.className = "options-port";
  portInput.setAttribute("inputmode", "numeric");
  const portSave = doc.createElement("button");
  portSave.type = "button";
  portSave.className = "options-port-save";
  portSave.textContent = "Save port";
  const portStatus = doc.createElement("p");
  portStatus.className = "options-port-status";

  container.replaceChildren(
    tokenLabel,
    tokenInput,
    tokenSave,
    tokenClear,
    tokenStatus,
    portLabel,
    portInput,
    portSave,
    portStatus,
  );

  /** Reads the store and shows the token state and the port state. */
  async function refresh(): Promise<void> {
    let stored: { readonly token: unknown; readonly port: unknown };
    try {
      stored = await deps.store.load();
    } catch (error) {
      deps.log.error(loadLabel, error);
      tokenStatus.textContent = settingsFailedText;
      portStatus.textContent = settingsFailedText;
      return;
    }
    if (stored.token === undefined) {
      tokenStatus.textContent = tokenMissingText;
    } else if (parseToken(stored.token) === undefined) {
      tokenStatus.textContent = tokenInvalidText;
    } else {
      tokenStatus.textContent = tokenSetText;
    }
    if (stored.port === undefined) {
      portInput.value = "";
      portStatus.textContent = portDefaultText;
    } else {
      const port = parsePort(stored.port);
      if (port === undefined) {
        portInput.value = "";
        portStatus.textContent = portInvalidText;
      } else {
        portInput.value = port;
        portStatus.textContent = "";
      }
    }
  }

  tokenSave.addEventListener("click", () => {
    void (async () => {
      const token = parseToken(tokenInput.value);
      if (token === undefined) {
        tokenStatus.textContent = tokenRejectedText;
        return;
      }
      try {
        await deps.store.saveToken(token);
      } catch (error) {
        deps.log.error(saveTokenLabel, error);
        tokenStatus.textContent = tokenSaveFailedText;
        return;
      }
      tokenInput.value = "";
      await refresh();
    })();
  });

  tokenClear.addEventListener("click", () => {
    void (async () => {
      try {
        await deps.store.clearToken();
      } catch (error) {
        deps.log.error(clearTokenLabel, error);
        tokenStatus.textContent = tokenClearFailedText;
        return;
      }
      await refresh();
    })();
  });

  portSave.addEventListener("click", () => {
    void (async () => {
      const port = parsePort(portInput.value);
      if (port === undefined) {
        portStatus.textContent = portRejectedText;
        return;
      }
      try {
        await deps.store.savePort(port);
      } catch (error) {
        deps.log.error(savePortLabel, error);
        portStatus.textContent = portSaveFailedText;
        return;
      }
      await refresh();
    })();
  });

  await refresh();
}
