import assert from "node:assert/strict";
import { JSDOM } from "jsdom";

/** A fresh container element in its own document. */
export function container(): HTMLElement {
  const dom = new JSDOM("<!DOCTYPE html><body><div id='root'></div></body>");
  const element = dom.window.document.getElementById("root");
  assert.ok(element !== null);
  return element;
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
