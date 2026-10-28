import assert from "node:assert/strict";
import { JSDOM } from "jsdom";

import type { Message } from "../../src/core/i18n/catalog.ts";
import { locales } from "../../src/core/i18n/locale.ts";
import type { Locale } from "../../src/core/i18n/locale.ts";
import type { PopupElements } from "../../src/ui/render.ts";

/** A fresh container element in its own document. */
export function container(): HTMLElement {
  const dom = new JSDOM("<!DOCTYPE html><body><div id='root'></div></body>");
  const element = dom.window.document.getElementById("root");
  assert.ok(element !== null);
  return element;
}

/** The one element with id in doc, asserting it exists. */
function byId<T extends HTMLElement = HTMLElement>(
  doc: Document,
  id: string,
): T {
  const element = doc.getElementById(id);
  assert.ok(element !== null, id);
  return element as T;
}

/**
 * The popup's skeleton in a fresh document: five containers, all hidden, the
 * primary button disabled, and the status region inside the action section
 * after the button. The markup is written here until the step that ships
 * popup.html makes this helper read that file; both must keep this shape.
 */
export function popupElements(): PopupElements {
  const dom = new JSDOM(`<!DOCTYPE html><html lang="en"><body>
    <section id="video" hidden></section>
    <section id="checks" hidden></section>
    <section id="rejection" aria-labelledby="rejection-text" hidden></section>
    <section id="action" hidden>
      <button id="primary" type="button" disabled></button>
      <div id="status" role="status"></div>
    </section>
    <details id="details" hidden></details>
  </body></html>`);
  const doc = dom.window.document;
  return {
    video: byId(doc, "video"),
    checks: byId(doc, "checks"),
    rejection: byId(doc, "rejection"),
    details: byId<HTMLDetailsElement>(doc, "details"),
    action: byId(doc, "action"),
    primary: byId<HTMLButtonElement>(doc, "primary"),
    status: byId(doc, "status"),
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

/** The value element of the summary field labelled label, if any. */
export function valueElement(
  root: HTMLElement,
  label: string,
): HTMLElement | undefined {
  for (const field of root.querySelectorAll(".summary-field")) {
    if (field.querySelector(".summary-label")?.textContent === label) {
      return field.querySelector<HTMLElement>(".summary-value") ?? undefined;
    }
  }
  return undefined;
}

/** The displayed value of the summary field labelled label, if any. */
export function valueOf(root: HTMLElement, label: string): string | undefined {
  return valueElement(root, label)?.textContent ?? undefined;
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
