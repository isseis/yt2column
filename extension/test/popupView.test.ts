import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { CollectedInput } from "../src/core/collect.ts";
import { acceptMetadata } from "../src/core/metadata.ts";
import type { MetadataResult } from "../src/core/metadata.ts";
import {
  buildCollectedView,
  metadataDisplayLimit,
} from "../src/core/popupView.ts";
import type { CollectedView } from "../src/core/popupView.ts";
import { maxBodyBytes, prepareSend } from "../src/core/prepare.ts";

const watchUrl = "https://www.youtube.com/watch?v=abc";

/** A collected input; tests may assert the branded type to build fixtures. */
function collectedInput(
  selection = "selection",
  url = watchUrl,
  title = "a tab title",
): CollectedInput {
  return { selection, url, title } as CollectedInput;
}

/** The accepted metadata for a page with these values. */
function accepted(
  values: { title?: string; channelName?: string; description?: string },
  descriptionCollapsed = false,
): MetadataResult {
  return acceptMetadata({ ...values, descriptionCollapsed });
}

/** The view for an input and metadata, with the size check run over them. */
function viewFor(
  input: CollectedInput,
  metadata: MetadataResult = accepted({}),
): CollectedView {
  return buildCollectedView(
    input,
    metadata,
    prepareSend(input, metadata.values),
  );
}

/** The view's selection row for one selection. */
function previews(selection: string): CollectedView["selection"] {
  return viewFor(collectedInput(selection)).selection;
}

describe("buildCollectedView", () => {
  describe("video section", () => {
    it("uses the video title and channel name when they are sent", () => {
      const view = viewFor(
        collectedInput(),
        accepted({ title: "Video", channelName: "Channel" }),
      );
      assert.deepEqual(view.video, {
        heading: { value: "Video", truncated: false },
        channelName: { value: "Channel", truncated: false },
      });
    });

    it("uses the tab title for the heading when the video title is not sent", () => {
      const view = viewFor(
        collectedInput("selection", watchUrl, "Tab title"),
        accepted({}),
      );
      assert.deepEqual(view.video.heading, {
        value: "Tab title",
        truncated: false,
      });
    });

    it("carries no channel name when it is not sent", () => {
      const view = viewFor(collectedInput(), accepted({ title: "Video" }));
      assert.equal("channelName" in view.video, false);
    });

    it("truncates a long video title and channel name to the display limit", () => {
      const long = "t".repeat(metadataDisplayLimit + 1);
      const view = viewFor(
        collectedInput(),
        accepted({ title: long, channelName: long }),
      );
      const shown = {
        value: "t".repeat(metadataDisplayLimit),
        truncated: true,
      };
      assert.deepEqual(view.video.heading, shown);
      assert.deepEqual(view.video.channelName, shown);
    });

    it("keeps a tab title up to its own longer limit", () => {
      const tabTitle = "t".repeat(metadataDisplayLimit + 1);
      const view = viewFor(collectedInput("selection", watchUrl, tabTitle));
      assert.deepEqual(view.video.heading, {
        value: tabTitle,
        truncated: false,
      });
    });
  });

  describe("selection row", () => {
    it("counts the whole selection, not the previews", () => {
      // 60 lines of 10 characters: either 100-code-point preview holds only
      // part of it, so a preview-based count fails this.
      const selection = Array.from({ length: 60 }, () => "0123456789").join(
        "\n",
      );
      const view = viewFor(collectedInput(selection));
      assert.equal(view.selection.characterCount, 60 * 10 + 59);
      assert.equal(view.selection.lineCount, 60);
      assert.equal(view.selection.status.kind, "sendable");
    });

    it("carries the request that prepareSend accepted", () => {
      // Identity, not equality: a request rebuilt from the same inputs would
      // not be the one whose size passed the check.
      const input = collectedInput();
      const prepared = prepareSend(input, {});
      assert.ok(prepared.kind === "ready");
      const view = buildCollectedView(input, accepted({}), prepared);
      assert.ok(view.selection.status.kind === "sendable");
      assert.equal(view.selection.status.request, prepared.request);
    });

    it("marks a body over the limit as a warning with its reason", () => {
      // The selection alone takes the body over the limit.
      const input = collectedInput("a".repeat(maxBodyBytes));
      const metadata = accepted({});
      const prepared = prepareSend(input, metadata.values);
      assert.ok(prepared.kind === "blocked");
      const view = buildCollectedView(input, metadata, prepared);
      assert.deepEqual(view.selection.status, {
        kind: "blocked",
        reason: "body-too-large",
      });
    });
  });

  describe("previews", () => {
    it("shows the whole selection and no tail at exactly 100 code points", () => {
      const selection = "x".repeat(100);
      const check = previews(selection);
      assert.deepEqual(check.head, { value: selection, truncated: false });
      assert.equal("tail" in check, false);
    });

    it("cuts one code point over the limit to first and last 100 code points", () => {
      const selection = "x".repeat(101);
      const check = previews(selection);
      assert.deepEqual(check.head, { value: "x".repeat(100), truncated: true });
      assert.deepEqual(check.tail, { value: "x".repeat(100), truncated: true });
    });

    it("shows each line break as one space", () => {
      const check = previews("a\r\nb\rc\nd");
      assert.deepEqual(check.head, { value: "a b c d", truncated: false });
    });

    it("cuts the head before showing its line breaks as spaces", () => {
      // Replacing first would fit one more b in the 100 shown code points.
      const check = previews(`${"a".repeat(50)}\r\n${"b".repeat(60)}`);
      assert.deepEqual(check.head, {
        value: `${"a".repeat(50)} ${"b".repeat(48)}`,
        truncated: true,
      });
    });

    it("cuts the tail before showing its line breaks as spaces", () => {
      const check = previews(`${"b".repeat(60)}\r\n${"a".repeat(50)}`);
      assert.deepEqual(check.tail, {
        value: `${"b".repeat(48)} ${"a".repeat(50)}`,
        truncated: true,
      });
    });

    it("turns each half of a \\r\\n cut at the boundary into one space", () => {
      // The cut leaves only the \r of a \r\n pair at the head's end, and only
      // the \n at the tail's start; either half must still become one space.
      const headView = previews(`${"a".repeat(99)}\r\nb`);
      assert.deepEqual(headView.head, {
        value: `${"a".repeat(99)} `,
        truncated: true,
      });
      const tailView = previews(`${"x".repeat(10)}\r\n${"a".repeat(99)}`);
      assert.deepEqual(tailView.tail, {
        value: ` ${"a".repeat(99)}`,
        truncated: true,
      });
    });

    it("does not split a surrogate pair at either boundary", () => {
      const pair = "\u{1F600}";
      const headView = previews(`${"a".repeat(99)}${pair}${"b".repeat(10)}`);
      assert.deepEqual(headView.head, {
        value: `${"a".repeat(99)}${pair}`,
        truncated: true,
      });
      const tailView = previews(`${"b".repeat(10)}${pair}${"a".repeat(99)}`);
      assert.deepEqual(tailView.tail, {
        value: `${pair}${"a".repeat(99)}`,
        truncated: true,
      });
    });
  });

  describe("description row", () => {
    it("marks the full description OK with its code point count", () => {
      const view = viewFor(
        collectedInput(),
        accepted({ description: "a\u{1F600}b" }),
      );
      assert.deepEqual(view.description.status, {
        kind: "full",
        characterCount: 3,
      });
    });

    it("marks a collapsed description as a warning without a count", () => {
      const view = viewFor(
        collectedInput(),
        accepted({ description: "a description" }, true),
      );
      assert.deepEqual(view.description.status, {
        kind: "collapsed",
      });
    });

    it("marks a description that is not sent as a warning", () => {
      const view = viewFor(
        collectedInput(),
        accepted({ description: " \u3000" }),
      );
      assert.deepEqual(view.description.status, {
        kind: "not-sent",
      });
    });
  });

  describe("details", () => {
    it("carries the URL and the tab title", () => {
      const view = viewFor(collectedInput("selection", watchUrl, "Tab title"));
      assert.deepEqual(view.details, {
        url: { value: watchUrl, truncated: false },
        tabTitle: { value: "Tab title", truncated: false },
      });
    });

    it("keeps both values at 1000 code points and truncates one over", () => {
      const long = "t".repeat(1000);
      const atLimit = viewFor(collectedInput("selection", long, long));
      assert.deepEqual(atLimit.details, {
        url: { value: long, truncated: false },
        tabTitle: { value: long, truncated: false },
      });
      const over = viewFor(collectedInput("selection", `${long}t`, `${long}t`));
      assert.deepEqual(over.details, {
        url: { value: long, truncated: true },
        tabTitle: { value: long, truncated: true },
      });
    });
  });
});
