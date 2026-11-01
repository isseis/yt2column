import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

import { JSDOM } from "jsdom";

import type { Message } from "../../src/core/i18n/catalog.ts";
import { locales } from "../../src/core/i18n/locale.ts";
import type { Locale } from "../../src/core/i18n/locale.ts";
import type { PopupElements } from "../../src/ui/render.ts";
import type { ViewerElements } from "../../src/ui/renderViewer.ts";
import { extensionDir } from "./paths.ts";

/** A fresh container element in its own document. */
export function container(): HTMLElement {
  const dom = new JSDOM("<!DOCTYPE html><body><div id='root'></div></body>");
  const element = dom.window.document.getElementById("root");
  assert.ok(element !== null);
  return element;
}

/** The one element with id in doc, asserting it exists. */
function byId(doc: Document, id: string): HTMLElement {
  const element = doc.getElementById(id);
  assert.ok(element !== null, id);
  return element;
}

/** A fresh document parsed from a shipped page under static/. */
function staticDocument(file: string): Document {
  const html = readFileSync(path.join(extensionDir, "static", file), "utf8");
  return new JSDOM(html).window.document;
}

/** A fresh document parsed from the shipped static/popup.html. */
export function popupDocument(): Document {
  return staticDocument("popup.html");
}

/** A fresh document parsed from the shipped static/viewer.html. */
export function viewerDocument(): Document {
  return staticDocument("viewer.html");
}

/**
 * The popup's skeleton elements in doc, by default a fresh copy of the shipped
 * popup.html: five containers, all hidden, the primary button disabled, and
 * the status region inside the action section after the button.
 */
export function popupElements(doc = popupDocument()): PopupElements {
  const win = doc.defaultView;
  assert.ok(win !== null);
  const primary = byId(doc, "primary");
  assert.ok(primary instanceof win.HTMLButtonElement, "primary");
  const details = byId(doc, "details");
  assert.ok(details instanceof win.HTMLDetailsElement, "details");
  return {
    video: byId(doc, "video"),
    checks: byId(doc, "checks"),
    rejection: byId(doc, "rejection"),
    details,
    action: byId(doc, "action"),
    primary,
    status: byId(doc, "status"),
  };
}

/** The viewer's skeleton elements in doc, by default a fresh copy of the shipped viewer.html. */
export function viewerElements(doc = viewerDocument()): ViewerElements {
  const win = doc.defaultView;
  assert.ok(win !== null);
  const button = (id: string): HTMLButtonElement => {
    const element = byId(doc, id);
    assert.ok(element instanceof win.HTMLButtonElement, id);
    return element;
  };
  const videoLink = byId(doc, "video-link");
  assert.ok(videoLink instanceof win.HTMLAnchorElement, "video-link");
  return {
    heading: byId(doc, "heading"),
    channel: byId(doc, "channel"),
    videoLink,
    state: byId(doc, "state"),
    stateStatus: byId(doc, "state-status"),
    article: byId(doc, "article"),
    actions: byId(doc, "actions"),
    publish: button("publish"),
    publishRow: byId(doc, "publish-row"),
    save: button("save"),
    copy: button("copy"),
    actionStatus: byId(doc, "action-status"),
  };
}

/** The one element matching selector in root, asserting it exists. */
export function query<T extends Element>(
  root: ParentNode,
  selector: string,
): T {
  const element = root.querySelector(selector);
  assert.ok(element !== null, selector);
  return element as T;
}

/** Asserts the container's text shows the message and each of its steps. */
export function expectMessage(root: HTMLElement, message: Message): void {
  const text = root.textContent ?? "";
  assert.ok(text.includes(message.text), message.text);
  for (const step of message.steps) {
    assert.ok(text.includes(step), step);
  }
}

/** The locales other than locale. */
export function otherLocales(locale: Locale): Locale[] {
  return (Object.keys(locales) as Locale[]).filter((other) => other !== locale);
}

/**
 * Asserts root shows none of the given strings. Each is first checked to
 * differ from every string in shown -- a string the shown text contains would
 * be displayed anyway, so its absence could not be asserted.
 */
export function expectAbsent(
  root: HTMLElement,
  absent: readonly string[],
  shown: readonly string[],
): void {
  const text = root.textContent ?? "";
  for (const string of absent) {
    assert.ok(
      !shown.some((displayed) => displayed.includes(string)),
      `precondition: ${string} is part of the shown text`,
    );
    assert.ok(!text.includes(string), string);
  }
}

/**
 * Asserts root shows the locale's message, from messageFor, and no sentence
 * of the same message in any other locale.
 */
export function expectLocaleMessage(
  root: HTMLElement,
  locale: Locale,
  messageFor: (locale: Locale) => Message,
): void {
  const message = messageFor(locale);
  expectMessage(root, message);
  const shown = [message.text, ...message.steps];
  for (const other of otherLocales(locale)) {
    const otherMessage = messageFor(other);
    expectAbsent(root, [otherMessage.text, ...otherMessage.steps], shown);
  }
}
