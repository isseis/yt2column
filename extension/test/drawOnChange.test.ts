import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { drawOnKeyChange } from "../src/ui/drawOnChange.ts";
import { container } from "./helpers/dom.ts";

/** A draw that replaces the container's children with count new spans, counting its calls. */
function counter(target: HTMLElement, count = 1) {
  const calls = { count: 0 };
  const draw = () => {
    calls.count++;
    const doc = target.ownerDocument;
    target.replaceChildren();
    for (let i = 0; i < count; i++) {
      target.append(doc.createElement("span"));
    }
  };
  return { calls, draw };
}

describe("drawOnKeyChange", () => {
  it("draws the first time and not again for the same key and nodes", () => {
    const target = container();
    const { calls, draw } = counter(target);
    assert.equal(drawOnKeyChange(target, "a", draw), true);
    assert.equal(drawOnKeyChange(target, "a", draw), false);
    assert.equal(calls.count, 1);
  });

  it("draws again for another key", () => {
    const target = container();
    const { calls, draw } = counter(target);
    drawOnKeyChange(target, "a", draw);
    assert.equal(drawOnKeyChange(target, "b", draw), true);
    assert.equal(calls.count, 2);
  });

  // Each row changes the children drawn before by another means than draw,
  // keeping the key; the container must be drawn again.
  const changes: readonly {
    name: string;
    change: (target: HTMLElement) => void;
  }[] = [
    {
      // As many nodes as before, so only the node-by-node comparison sees it.
      name: "replaced by the same number of other nodes",
      change: (target) => {
        target.replaceChildren(
          target.ownerDocument.createElement("span"),
          target.ownerDocument.createElement("span"),
        );
      },
    },
    {
      // The drawn nodes stay first, so only the count sees it.
      name: "followed by an appended node",
      change: (target) => {
        target.append(target.ownerDocument.createElement("span"));
      },
    },
    {
      // The viewer hides the article this way and may show the same one again.
      name: "removed",
      change: (target) => {
        target.replaceChildren();
      },
    },
  ];
  for (const { name, change } of changes) {
    it(`draws again with the same key when the drawn nodes were ${name}`, () => {
      const target = container();
      const { calls, draw } = counter(target, 2);
      drawOnKeyChange(target, "a", draw);
      change(target);
      assert.equal(drawOnKeyChange(target, "a", draw), true);
      assert.equal(calls.count, 2);
    });
  }

  it("draws again after a draw that threw", () => {
    const target = container();
    assert.throws(() =>
      drawOnKeyChange(target, "a", () => {
        throw new Error("draw failed");
      }),
    );
    const { calls, draw } = counter(target);
    assert.equal(drawOnKeyChange(target, "a", draw), true);
    assert.equal(calls.count, 1);
  });
});
