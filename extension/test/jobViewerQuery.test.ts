import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { findJobViewers, viewerQueryWindowMs } from "../src/jobViewerQuery.ts";
import type { JobViewerQueryDeps } from "../src/jobViewerQuery.ts";
import { fakeChannels } from "./helpers/chrome.ts";
import { fakeClock, settle } from "./helpers/clock.ts";

const jobId = "fedcba9876543210fedcba9876543210";
const queryId = "0123456789abcdef0123456789abcdef";
const otherQueryId = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";

function setup() {
  const channels = fakeChannels();
  const clock = fakeClock();
  const viewer = channels.open();
  assert.ok(viewer.ok);
  const received: unknown[] = [];
  viewer.value.listen((data) => received.push(data));
  const deps: JobViewerQueryDeps = {
    openChannel: channels.open,
    queryId: () => queryId,
    schedule: clock.schedule,
  };
  return { channels, clock, viewer: viewer.value, received, deps };
}

const present = (tabId: number, id = queryId) => ({
  type: "present",
  queryId: id,
  tabId,
  windowId: 1,
});

describe("findJobViewers", () => {
  it("posts the find for the job and collects the matching answers of the window", async () => {
    const { clock, viewer, received, deps } = setup();
    const found = findJobViewers(jobId, deps);
    await settle();
    assert.deepEqual(received, [{ type: "find", queryId, jobId }]);
    viewer.post(present(7) as never);
    viewer.post(present(8, otherQueryId) as never);
    viewer.post({ type: "present", queryId, tabId: "x" } as never);
    await settle();
    clock.advance(viewerQueryWindowMs);
    assert.deepEqual(await found, [present(7)]);
  });

  it("ignores an answer that arrives after the window", async () => {
    const { clock, viewer, deps } = setup();
    const found = findJobViewers(jobId, deps);
    await settle();
    clock.advance(viewerQueryWindowMs);
    const answers = await found;
    viewer.post(present(7) as never);
    await settle();
    assert.deepEqual(answers, []);
  });

  it("yields no answers when the channel cannot be opened", async () => {
    const { channels, deps } = setup();
    channels.failOpen();
    assert.deepEqual(await findJobViewers(jobId, deps), []);
  });
});
