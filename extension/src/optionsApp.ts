// The options page's path: it draws the page's fixed texts, shows whether an
// API key is set (from the item names only, never a stored value), shows the
// build's information, and runs a save or a delete through optionsFlow.ts on
// the buttons. Every outcome is shown as its fixed message in the status
// region, so it is announced without moving the focus. The API key is only
// read from its field by the flow, and never drawn, logged or kept here.

import { catalogs } from "./core/i18n/catalog.ts";
import { resolveLocale } from "./core/i18n/locale.ts";
import type { Locale } from "./core/i18n/locale.ts";
import { optionsView } from "./core/optionsView.ts";
import type { BuildInfo, SettingsField } from "./core/wasmResult.ts";
import {
  deleteSettings,
  loadBuildInfo,
  readApiKeySet,
  saveSettings,
} from "./optionsFlow.ts";
import type { OptionsFlowDeps, OptionsFlowResult } from "./optionsFlow.ts";
import { messageNodes } from "./ui/message.ts";
import { applyPageLocale } from "./ui/pageLocale.ts";

/** The options page's skeleton, found in options.html by id. */
export interface OptionsElements {
  readonly heading: HTMLElement;
  readonly keyState: HTMLElement;
  readonly apiKeyLabel: HTMLElement;
  readonly apiKey: HTMLInputElement;
  readonly apiKeyHint: HTMLElement;
  readonly workspaceIdLabel: HTMLElement;
  readonly workspaceId: HTMLInputElement;
  readonly workspaceIdHint: HTMLElement;
  readonly save: HTMLButtonElement;
  readonly remove: HTMLButtonElement;
  readonly status: HTMLElement;
  readonly buildHeading: HTMLElement;
  readonly buildInfo: HTMLElement;
}

/** What the options entry needs; options.ts builds it from the browser APIs. */
export interface OptionsStartDeps {
  /** The browser's UI language, undefined when it cannot be read. */
  uiLanguage(): string | undefined;
  readonly flow: Omit<OptionsFlowDeps, "form">;
  /** Calls listener each time the page becomes visible: visibilitychange. */
  onVisible(listener: () => void): void;
}

/**
 * The skeleton's elements, found in doc by id. Undefined unless every element
 * is present and of its kind, so a partial skeleton is never drawn into.
 */
export function findOptionsElements(
  doc: Document,
): OptionsElements | undefined {
  const win = doc.defaultView;
  if (win === null) {
    return undefined;
  }
  const element = (id: string): HTMLElement | undefined =>
    doc.getElementById(id) ?? undefined;
  const input = (id: string): HTMLInputElement | undefined => {
    const found = doc.getElementById(id);
    return found instanceof win.HTMLInputElement ? found : undefined;
  };
  const button = (id: string): HTMLButtonElement | undefined => {
    const found = doc.getElementById(id);
    return found instanceof win.HTMLButtonElement ? found : undefined;
  };
  const elements = {
    heading: element("heading"),
    keyState: element("key-state"),
    apiKeyLabel: element("api-key-label"),
    apiKey: input("api-key"),
    apiKeyHint: element("api-key-hint"),
    workspaceIdLabel: element("workspace-id-label"),
    workspaceId: input("workspace-id"),
    workspaceIdHint: element("workspace-id-hint"),
    save: button("save"),
    remove: button("delete"),
    status: element("status"),
    buildHeading: element("build-heading"),
    buildInfo: element("build-info"),
  };
  for (const value of Object.values(elements)) {
    if (value === undefined) {
      return undefined;
    }
  }
  return elements as OptionsElements;
}

/** The text that says whether an API key is set. */
function keyStateText(set: boolean | undefined, locale: Locale): string {
  const texts = catalogs[locale].optionsPage.keyState;
  switch (set) {
    case true:
      return texts.set;
    case false:
      return texts.notSet;
    case undefined:
      return texts.unknown;
  }
}

/** The build's information as name and value pairs, or the unreadable message. */
function drawBuildInfo(
  container: HTMLElement,
  info: BuildInfo | undefined,
  locale: Locale,
): void {
  const doc = container.ownerDocument;
  const texts = catalogs[locale].optionsPage.buildInfo;
  if (info === undefined) {
    const dd = doc.createElement("dd");
    dd.textContent = texts.unreadable;
    container.replaceChildren(dd);
    return;
  }
  const commit =
    info.vcsRevision === ""
      ? texts.none
      : `${info.vcsRevision}${info.vcsModified ? ` ${texts.modified}` : ""}`;
  let llm: string;
  switch (info.llm?.kind) {
    case undefined:
      llm = texts.none;
      break;
    case "claude":
      llm = "claude";
      break;
    case "fake": {
      const scenario = info.llm.scenario;
      llm = `fake (article=${scenario.article}, failure=${scenario.failure}, failAt=${scenario.failAt}, delay=${String(scenario.delaySeconds)}s, settingsCheck=${scenario.settingsCheck})`;
      break;
    }
  }
  const pairs: readonly (readonly [string, string])[] = [
    [texts.commit, commit],
    [texts.goVersion, info.goVersion],
    [texts.llm, llm],
  ];
  container.replaceChildren(
    ...pairs.flatMap(([name, value]) => {
      const term = doc.createElement("dt");
      term.textContent = name;
      const definition = doc.createElement("dd");
      definition.textContent = value;
      return [term, definition];
    }),
  );
}

/** Marks the fields the check did not accept, and clears every other mark. */
function markFields(
  elements: OptionsElements,
  rejected: readonly SettingsField[],
): void {
  const mark = (input: HTMLInputElement, field: SettingsField): void => {
    if (rejected.includes(field)) {
      input.setAttribute("aria-invalid", "true");
    } else {
      input.removeAttribute("aria-invalid");
    }
  };
  mark(elements.apiKey, "anthropicApiKey");
  mark(elements.workspaceId, "anthropicWorkspaceId");
}

/**
 * Options entry. Decides the locale and sets the page language and title,
 * finds the skeleton (with any element missing it does nothing), draws the
 * fixed texts, then shows whether an API key is set and the build's
 * information. A save or a delete keeps both fields and both buttons
 * disabled until it ends, then shows its message, clears the fields only
 * after a write, marks the fields a check did not accept, and reads again
 * whether an API key is set. Coming back to the page reads that again too.
 * Never throws.
 */
export async function startOptions(
  doc: Document,
  deps: OptionsStartDeps,
): Promise<void> {
  const locale = resolveLocale(deps.uiLanguage());
  applyPageLocale(doc, locale);
  const texts = catalogs[locale].optionsPage;
  doc.title = texts.title;
  const found = findOptionsElements(doc);
  if (found === undefined) {
    return;
  }
  const elements: OptionsElements = found;
  elements.heading.textContent = texts.heading;
  elements.apiKeyLabel.textContent = texts.apiKey;
  elements.apiKeyHint.textContent = texts.apiKeyHint;
  elements.workspaceIdLabel.textContent = texts.workspaceId;
  elements.workspaceIdHint.textContent = texts.workspaceIdHint;
  elements.save.textContent = texts.save;
  elements.remove.textContent = texts.remove;
  elements.buildHeading.textContent = texts.buildInfo.heading;

  const flow: OptionsFlowDeps = {
    ...deps.flow,
    form: {
      read: () => ({
        apiKey: elements.apiKey.value,
        workspaceId: elements.workspaceId.value,
      }),
      setBusy: (busy) => {
        elements.apiKey.readOnly = busy;
        elements.workspaceId.readOnly = busy;
        elements.save.disabled = busy;
        elements.remove.disabled = busy;
      },
    },
  };

  const showKeyState = (set: boolean | undefined): void => {
    elements.keyState.textContent = keyStateText(set, locale);
  };

  const showResult = (done: OptionsFlowResult): void => {
    const view = optionsView(done.result, locale);
    elements.status.className = `options-result options-result-${view.tone}`;
    elements.status.replaceChildren(...messageNodes(doc, view.message));
    markFields(elements, view.rejectedFields);
    if (view.clearFields) {
      elements.apiKey.value = "";
      elements.workspaceId.value = "";
    }
    showKeyState(done.apiKeySet);
  };

  elements.save.addEventListener("click", () => {
    void saveSettings(flow).then(showResult);
  });
  elements.remove.addEventListener("click", () => {
    void deleteSettings(flow).then(showResult);
  });
  deps.onVisible(() => {
    void readApiKeySet(flow).then(showKeyState);
  });

  showKeyState(await readApiKeySet(flow));
  drawBuildInfo(elements.buildInfo, await loadBuildInfo(flow), locale);
}
