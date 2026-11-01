// The service worker entry point. It registers the start coordinator as the
// runtime message listener synchronously, as the first and only top-level
// statement, with dependencies built by synchronous calls alone: the event
// that woke a stopped worker reaches only the listeners its first run
// registered, so nothing may be awaited before the registration.
import {
  createLogger,
  createQueryIdSource,
  createSendDeps,
  createViewerChannelOpener,
  createViewerTabs,
  listenForMessages,
} from "./browser/chromeDeps.ts";
import { createStartCoordinator } from "./startCoordinator.ts";

listenForMessages(
  chrome.runtime,
  createStartCoordinator({
    identity: chrome.runtime,
    tabs: createViewerTabs(chrome.tabs),
    openChannel: createViewerChannelOpener(),
    queryId: createQueryIdSource(),
    send: createSendDeps(chrome.runtime),
    now: () => Date.now(),
    log: createLogger(),
  }),
);
