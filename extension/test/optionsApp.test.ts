import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { catalogs } from "../src/core/i18n/catalog.ts";
import { findOptionsElements, startOptions } from "../src/optionsApp.ts";
import { startWasmWorker } from "../src/wasmWorkerClient.ts";
import { fakeLocal, type FakeLocal } from "./helpers/chrome.ts";
import { fakeClock, settle, type FakeClock } from "./helpers/clock.ts";
import { optionsDocument } from "./helpers/dom.ts";
import { recordingLogger } from "./helpers/logger.ts";
import { fakeWorkers, type FakeWorkers } from "./helpers/worker.ts";

const secret = "sk-ant-SECRET-VALUE-3f1c";

interface Page {
  readonly doc: Document;
  readonly local: FakeLocal;
  readonly workers: FakeWorkers;
  readonly clock: FakeClock;
  readonly elements: NonNullable<ReturnType<typeof findOptionsElements>>;
  /** Makes the page visible again, which fires its listeners. */
  show(): void;
  /** The end of startOptions, which waits for the build information. */
  readonly done: Promise<void>;
}

/** Opens the options page with an API key set or not; the build's Worker answers buildInfo. */
async function open(
  options: {
    readonly uiLanguage?: string;
    readonly keySet?: boolean;
    readonly buildInfo?: string;
  } = {},
): Promise<Page> {
  const doc = optionsDocument();
  const local = fakeLocal();
  if (options.keySet === true) {
    local.items.set("anthropic.apiKey", "sk-ant-stored");
    local.items.set("anthropic.workspaceId", "");
  }
  const workers = fakeWorkers();
  const clock = fakeClock();
  const visible: (() => void)[] = [];
  const done = startOptions(doc, {
    uiLanguage: () => options.uiLanguage ?? "en-US",
    flow: {
      startWorker: () => startWasmWorker(workers.create, clock.schedule),
      schedule: clock.schedule,
      write: local.write,
      remove: local.remove,
      keys: local.keys,
    },
    onVisible: (listener) => visible.push(listener),
  });
  await settle();
  const build = workers.created[0];
  assert.ok(build !== undefined, "no Worker for the build information");
  build.send({ kind: "ready" });
  build.send({
    callId: 1,
    kind: "reply",
    value:
      options.buildInfo ??
      JSON.stringify({
        goVersion: "go1.26.5",
        vcsRevision: "0123456789abcdef",
        vcsModified: true,
        mainPath: "github.com/isseis/yt2column/cmd/yt2column-wasm",
        llm: { kind: "claude" },
      }),
  });
  await done;
  const elements = findOptionsElements(doc);
  assert.ok(elements !== undefined);
  return {
    doc,
    local,
    workers,
    clock,
    elements,
    show: () => {
      for (const listener of visible) {
        listener();
      }
    },
    done,
  };
}

/** Presses save and lets the check's Worker answer with result. */
async function save(page: Page, result: string): Promise<void> {
  page.elements.save.click();
  await settle();
  const worker = page.workers.created.at(-1);
  assert.ok(worker !== undefined);
  worker.send({ kind: "ready" });
  worker.send({ callId: 1, kind: "reply", value: result });
  await settle();
}

function statusText(page: Page): string {
  return page.elements.status.textContent ?? "";
}

describe("startOptions", () => {
  it("draws the page's texts, its title and language in the display language", async () => {
    const page = await open({ uiLanguage: "ja-JP" });
    const texts = catalogs.ja.optionsPage;
    assert.equal(page.doc.documentElement.lang, "ja");
    assert.equal(page.doc.title, texts.title);
    assert.equal(page.elements.heading.textContent, texts.heading);
    assert.equal(page.elements.apiKeyLabel.textContent, texts.apiKey);
    assert.equal(page.elements.apiKeyHint.textContent, texts.apiKeyHint);
    assert.equal(page.elements.workspaceIdLabel.textContent, texts.workspaceId);
    assert.equal(page.elements.save.textContent, texts.save);
    assert.equal(page.elements.remove.textContent, texts.remove);
  });

  for (const [keySet, text] of [
    [false, catalogs.en.optionsPage.keyState.notSet],
    [true, catalogs.en.optionsPage.keyState.set],
  ] as const) {
    it(`says whether an API key is set from the item names alone (${String(keySet)})`, async () => {
      const page = await open({ keySet });
      assert.equal(page.elements.keyState.textContent, text);
      // Only the names were read: the stored value is never asked for.
      assert.deepEqual(page.local.calls, ["keys"]);
      assert.equal(page.doc.body.textContent?.includes("sk-ant-stored"), false);
    });
  }

  it("shows the build's commit, Go version and LLM", async () => {
    const page = await open();
    assert.deepEqual(
      Array.from(
        page.elements.buildInfo.children,
        (child) => child.textContent,
      ),
      [
        "Commit",
        "0123456789abcdef (modified)",
        "Go version",
        "go1.26.5",
        "LLM",
        "claude",
      ],
    );
  });

  it("shows a development build's fake LLM with its scenario", async () => {
    const page = await open({
      buildInfo: JSON.stringify({
        goVersion: "go1.26.5",
        vcsRevision: "",
        vcsModified: false,
        mainPath: "github.com/isseis/yt2column/cmd/yt2column-wasm-dev",
        llm: {
          kind: "fake",
          scenario: {
            article: "normal",
            failure: "none",
            failAt: "draft",
            delaySeconds: 2,
            settingsCheck: "ok",
          },
        },
      }),
    });
    const text = page.elements.buildInfo.textContent ?? "";
    assert.ok(text.includes("None"), text);
    assert.ok(
      text.includes(
        "fake (article=normal, failure=none, failAt=draft, delay=2s, settingsCheck=ok)",
      ),
      text,
    );
  });

  it("says the build information could not be read when the Worker's answer is not one", async () => {
    const page = await open({ buildInfo: "{}" });
    assert.equal(
      page.elements.buildInfo.textContent,
      catalogs.en.optionsPage.buildInfo.unreadable,
    );
  });

  it("saves a checked key, empties the fields, and says it is set, in the status region", async () => {
    const page = await open();
    page.elements.apiKey.value = secret;
    page.elements.workspaceId.value = "";
    await save(page, JSON.stringify({ status: "ok" }));
    assert.equal(page.local.items.get("anthropic.apiKey"), secret);
    assert.equal(page.elements.apiKey.value, "");
    assert.ok(statusText(page).includes(catalogs.en.options.saved.text));
    assert.ok(page.elements.status.classList.contains("options-result-plain"));
    assert.equal(
      page.elements.keyState.textContent,
      catalogs.en.optionsPage.keyState.set,
    );
    assert.equal(page.doc.body.textContent?.includes(secret), false);
  });

  it("marks the field the check did not accept, keeps the values and writes nothing", async () => {
    const page = await open();
    page.elements.apiKey.value = " bad ";
    page.elements.workspaceId.value = "ws";
    await save(
      page,
      JSON.stringify({ status: "rejected", fields: ["anthropicWorkspaceId"] }),
    );
    assert.equal(
      page.elements.workspaceId.getAttribute("aria-invalid"),
      "true",
    );
    assert.equal(page.elements.apiKey.hasAttribute("aria-invalid"), false);
    assert.equal(page.elements.apiKey.value, " bad ");
    assert.equal(page.local.calls.includes("write"), false);
    assert.ok(
      statusText(page).includes(catalogs.en.options.field_rejected.text),
    );
    assert.ok(
      page.elements.status.classList.contains("options-result-failure"),
    );
    // A later save clears the marks and the failure's look.
    await save(page, JSON.stringify({ status: "ok" }));
    assert.equal(page.elements.workspaceId.hasAttribute("aria-invalid"), false);
    assert.equal(
      page.elements.status.classList.contains("options-result-failure"),
      false,
    );
  });

  it("keeps the fields read-only and both buttons disabled while the check runs", async () => {
    const page = await open();
    page.elements.save.click();
    await settle();
    for (const input of [page.elements.apiKey, page.elements.workspaceId]) {
      assert.equal(input.readOnly, true);
    }
    for (const button of [page.elements.save, page.elements.remove]) {
      assert.equal(button.disabled, true);
    }
    const worker = page.workers.created.at(-1);
    assert.ok(worker !== undefined);
    worker.send({ kind: "ready" });
    worker.send({
      callId: 1,
      kind: "reply",
      value: JSON.stringify({ status: "key_rejected" }),
    });
    await settle();
    assert.equal(page.elements.apiKey.readOnly, false);
    assert.equal(page.elements.save.disabled, false);
  });

  it("deletes the settings and says so", async () => {
    const page = await open({ keySet: true });
    page.elements.remove.click();
    await settle();
    assert.equal(page.local.items.has("anthropic.apiKey"), false);
    assert.ok(statusText(page).includes(catalogs.en.options.deleted.text));
    assert.equal(
      page.elements.keyState.textContent,
      catalogs.en.optionsPage.keyState.notSet,
    );
  });

  it("reads again whether a key is set when the page comes back to the front", async () => {
    const page = await open();
    page.local.items.set("anthropic.apiKey", "sk-ant-other-tab");
    page.show();
    await settle();
    assert.equal(
      page.elements.keyState.textContent,
      catalogs.en.optionsPage.keyState.set,
    );
  });

  it("says whether a key is set is unknown when the names cannot be read", async () => {
    const page = await open();
    page.local.fail("keys");
    page.show();
    await settle();
    assert.equal(
      page.elements.keyState.textContent,
      catalogs.en.optionsPage.keyState.unknown,
    );
  });

  it("does nothing on a skeleton without one of its elements", async () => {
    const doc = optionsDocument();
    doc.getElementById("save")?.remove();
    const log = recordingLogger();
    await startOptions(doc, {
      uiLanguage: () => "en-US",
      flow: {
        startWorker: () => {
          throw new Error("no Worker may start");
        },
        schedule: () => () => {},
        write: async () => ({ ok: true, value: undefined }),
        remove: async () => ({ ok: true, value: undefined }),
        keys: async () => {
          log.log.info("keys", undefined);
          return { ok: true, value: { apiKeySet: false } };
        },
      },
      onVisible: () => {},
    });
    assert.deepEqual(log.infos, []);
    assert.equal(doc.getElementById("heading")?.textContent, "");
  });
});
