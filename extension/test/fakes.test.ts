import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type {
  OpenViewerChannel,
  ViewerChannel,
} from "../src/browser/chromeDeps.ts";
import type { ViewerChannelMessage } from "../src/core/extensionMessages.ts";
import { fakeChannels } from "./helpers/chrome.ts";
import { fakeClock, settle } from "./helpers/clock.ts";

// The ordering tests of the service worker and the viewer rely on these two
// fakes behaving as the browser does, so their rules are pinned here.

const message: ViewerChannelMessage = {
  type: "took-over",
  queryId: "0123456789abcdef0123456789abcdef",
  tabId: 7,
};

/** An open endpoint and the data it has received. */
function endpoint(open: OpenViewerChannel): {
  channel: ViewerChannel;
  received: unknown[];
} {
  const opened = open();
  assert.ok(opened.ok);
  const received: unknown[] = [];
  opened.value.listen((data) => received.push(data));
  return { channel: opened.value, received };
}

describe("fake viewers' channel", () => {
  it("delivers to every other endpoint as a task, never to the poster", async () => {
    const { open } = fakeChannels();
    const poster = endpoint(open);
    const first = endpoint(open);
    const second = endpoint(open);
    assert.equal(poster.channel.post(message), true);
    assert.deepEqual(first.received, [], "delivered during post");
    await settle();
    assert.deepEqual(first.received, [message]);
    assert.deepEqual(second.received, [message]);
    assert.deepEqual(poster.received, []);
  });

  it("delivers a copy, not the posted object", async () => {
    const { open } = fakeChannels();
    const poster = endpoint(open);
    const receiver = endpoint(open);
    poster.channel.post(message);
    await settle();
    assert.notEqual(receiver.received[0], message);
  });

  it("delivers nothing to an endpoint closed before delivery", async () => {
    const { open } = fakeChannels();
    const poster = endpoint(open);
    const receiver = endpoint(open);
    poster.channel.post(message);
    receiver.channel.close();
    await settle();
    assert.deepEqual(receiver.received, []);
  });

  it("posts nothing from a closed endpoint", async () => {
    const { open, posted } = fakeChannels();
    const poster = endpoint(open);
    const receiver = endpoint(open);
    poster.channel.close();
    assert.equal(poster.channel.post(message), false);
    await settle();
    assert.deepEqual(receiver.received, []);
    assert.deepEqual(posted, []);
  });

  it("stops calling a listener that was removed", async () => {
    const { open } = fakeChannels();
    const poster = endpoint(open);
    const opened = open();
    assert.ok(opened.ok);
    const received: unknown[] = [];
    const stop = opened.value.listen((data) => received.push(data));
    stop();
    poster.channel.post(message);
    await settle();
    assert.deepEqual(received, []);
  });
});

describe("fake clock", () => {
  it("runs due callbacks in due order, then scheduling order, at their due time", () => {
    const clock = fakeClock(1_000);
    const ran: string[] = [];
    clock.schedule(30, () => ran.push(`c@${clock.now()}`));
    clock.schedule(10, () => ran.push(`a@${clock.now()}`));
    clock.schedule(10, () => ran.push(`b@${clock.now()}`));
    clock.advance(30);
    assert.deepEqual(ran, ["a@1010", "b@1010", "c@1030"]);
    assert.equal(clock.now(), 1_030);
  });

  it("runs nothing before its due time and keeps it pending", () => {
    const clock = fakeClock();
    const ran: number[] = [];
    clock.schedule(1_000, () => ran.push(clock.now()));
    clock.advance(999);
    assert.deepEqual(ran, []);
    assert.equal(clock.pending(), 1);
    clock.advance(1);
    assert.deepEqual(ran, [1_000]);
    assert.equal(clock.pending(), 0);
  });

  it("runs a callback scheduled on the way when it falls due within the advance", () => {
    const clock = fakeClock();
    const ran: number[] = [];
    clock.schedule(10, () => {
      ran.push(clock.now());
      clock.schedule(10, () => ran.push(clock.now()));
      clock.schedule(100, () => ran.push(clock.now()));
    });
    clock.advance(50);
    assert.deepEqual(ran, [10, 20]);
    assert.equal(clock.pending(), 1);
  });

  it("does not run a cancelled callback", () => {
    const clock = fakeClock();
    const ran: number[] = [];
    const cancel = clock.schedule(10, () => ran.push(1));
    cancel();
    clock.advance(10);
    assert.deepEqual(ran, []);
  });
});
