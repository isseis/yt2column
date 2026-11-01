import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { ArticleKey } from "../src/core/articleKey.ts";
import type { ViewerChannelMessage } from "../src/core/extensionMessages.ts";
import { viewerQueryWindowMs } from "../src/core/poll.ts";
import { askViewers, findViewers } from "../src/viewerQuery.ts";
import type { ViewerQueryDeps } from "../src/viewerQuery.ts";
import { fakeChannels } from "./helpers/chrome.ts";
import { fakeClock, settle } from "./helpers/clock.ts";
import type { FakeClock } from "./helpers/clock.ts";

const key = "key-1" as ArticleKey;
const queryId = "0123456789abcdef0123456789abcdef";
const otherQueryId = "fedcba9876543210fedcba9876543210";

/** A query against a fake channel and clock, and a viewer endpoint that sees its find. */
function setup(): {
  deps: ViewerQueryDeps;
  clock: FakeClock;
  viewer: { post(message: unknown): void; received: unknown[] };
} {
  const channels = fakeChannels();
  const clock = fakeClock();
  const opened = channels.open();
  assert.ok(opened.ok);
  const received: unknown[] = [];
  opened.value.listen((data) => received.push(data));
  return {
    deps: {
      openChannel: channels.open,
      queryId: () => queryId,
      schedule: clock.schedule,
    },
    clock,
    viewer: {
      // The channel's type admits only checked messages; a viewer of another
      // version could post anything, so the test posts unchecked values.
      post: (message) => opened.value.post(message as ViewerChannelMessage),
      received,
    },
  };
}

const present = (tabId: number, id = queryId): ViewerChannelMessage => ({
  type: "present",
  queryId: id,
  tabId,
  windowId: 1,
  sentAt: 10,
});

describe("findViewers", () => {
  it("posts one find for the key and collects the answers to it", async () => {
    const { deps, clock, viewer } = setup();
    const result = findViewers(key, deps);
    await settle();
    assert.deepEqual(viewer.received, [{ type: "find", queryId, key }]);
    viewer.post(present(5));
    viewer.post({ type: "waiting", queryId, tabId: 6, sentAt: 20 });
    await settle();
    clock.advance(viewerQueryWindowMs);
    assert.deepEqual(await result, {
      queryId,
      present: [present(5)],
      waiting: [{ type: "waiting", queryId, tabId: 6, sentAt: 20 }],
    });
  });

  it("counts an answer just before the window ends and ignores one at its end", async () => {
    const { deps, clock, viewer } = setup();
    const result = findViewers(key, deps);
    await settle();
    clock.advance(viewerQueryWindowMs - 1);
    viewer.post(present(5));
    await settle();
    clock.advance(1);
    viewer.post(present(6));
    await settle();
    assert.deepEqual((await result).present, [present(5)]);
  });

  it("ignores answers to another query and messages that fail the check", async () => {
    const { deps, clock, viewer } = setup();
    const result = findViewers(key, deps);
    await settle();
    viewer.post(present(5, otherQueryId));
    viewer.post({
      type: "waiting",
      queryId: otherQueryId,
      tabId: 6,
      sentAt: 1,
    });
    viewer.post({ ...present(6), extra: true });
    viewer.post({
      type: "present",
      queryId,
      tabId: -1,
      windowId: 1,
      sentAt: 1,
    });
    viewer.post({ type: "took-over", queryId, tabId: 7 });
    viewer.post({ type: "find", queryId, key });
    await settle();
    clock.advance(viewerQueryWindowMs);
    assert.deepEqual(await result, { queryId, present: [], waiting: [] });
  });

  it("resolves with no answers when no viewer answers", async () => {
    const { deps, clock } = setup();
    const result = findViewers(key, deps);
    clock.advance(viewerQueryWindowMs);
    assert.deepEqual(await result, { queryId, present: [], waiting: [] });
    assert.equal(clock.pending(), 0);
  });

  it("resolves at once with no answers when the channel cannot be opened", async () => {
    const channels = fakeChannels();
    channels.failOpen();
    const clock = fakeClock();
    const result = await findViewers(key, {
      openChannel: channels.open,
      queryId: () => queryId,
      schedule: clock.schedule,
    });
    assert.deepEqual(result, { queryId, present: [], waiting: [] });
    assert.equal(clock.pending(), 0);
  });
});

describe("askViewers", () => {
  const takeOver: ViewerChannelMessage = {
    type: "take-over",
    queryId,
    tabId: 5,
    sentAt: 10,
  };

  it("stops at the first answer onAnswer accepts, and hears nothing after", async () => {
    const { deps, clock, viewer } = setup();
    const seen: string[] = [];
    const asked = askViewers(takeOver, deps, (answer) => {
      seen.push(answer.type);
      return answer.type === "took-over";
    });
    await settle();
    viewer.post(present(5));
    viewer.post({ type: "took-over", queryId, tabId: 5 });
    viewer.post(present(6));
    await settle();
    assert.equal(await asked, true);
    assert.deepEqual(seen, ["present", "took-over"]);
    assert.equal(clock.pending(), 0, "the window timer is left running");
  });

  it("resolves false at once when the channel cannot post", async () => {
    const clock = fakeClock();
    let closed = false;
    const asked = await askViewers(
      takeOver,
      {
        openChannel: () => ({
          ok: true,
          value: {
            post: () => false,
            listen: () => () => {},
            close: () => {
              closed = true;
            },
          },
        }),
        schedule: clock.schedule,
      },
      () => true,
    );
    assert.equal(asked, false);
    assert.equal(closed, true);
    assert.equal(clock.pending(), 0);
  });
});
