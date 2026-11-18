// The service worker entry point. It registers the job coordinator's
// listeners -- runtime messages, runner Ports and closed tabs -- synchronously,
// as top-level statements, with dependencies built by synchronous calls
// alone: the event that woke a stopped worker reaches only the listeners its
// first run registered, so nothing may be awaited before the registrations.
// Every start of the worker also limits chrome.storage.local to the
// extension's own pages, since whether the setting outlives a restart is not
// documented.
import {
  createLogger,
  createQueryIdSource,
  createViewerTabs,
  listenForClosedTabs,
  listenForConnections,
  listenForMessages,
  localKeys,
  readSessionItems,
  removeSessionItems,
  schedule,
  setLocalAccessLevel,
  writeSessionItems,
} from "./browser/chromeDeps.ts";
import { createJobCoordinator } from "./jobCoordinator.ts";

const log = createLogger();
const coordinator = createJobCoordinator({
  identity: chrome.runtime,
  tabs: createViewerTabs(chrome.tabs),
  keys: localKeys,
  readSession: readSessionItems,
  writeSession: writeSessionItems,
  removeSession: removeSessionItems,
  newId: createQueryIdSource(),
  schedule,
  now: () => Date.now(),
  log,
});
listenForMessages(chrome.runtime, coordinator.onMessage);
listenForConnections(chrome.runtime, coordinator.onConnect);
listenForClosedTabs(chrome.tabs, coordinator.onTabRemoved);
void setLocalAccessLevel().then((set) => {
  if (!set.ok) {
    log.error("yt2column access level", { set: false });
  }
});
