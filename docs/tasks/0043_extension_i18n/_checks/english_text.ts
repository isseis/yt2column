// Checks that the extension's displayed English text is the same at the
// merge-base with origin/main and at HEAD. Run from the repository root,
// after `git fetch origin main` and `make ext-install`:
//
//   node docs/tasks/0043_extension_i18n/_checks/english_text.ts
//
// Both versions of extension/src and extension/test/helpers are taken from
// their commits (never from the working tree) into temporary directories that
// share the working tree's extension/node_modules. In each, the same driver
// renders every rejection reason, the metadata fields and the expand prompt,
// every send outcome and block reason, and the send button with its sending
// notice, and prints one line per displayed element (label, value, message
// paragraph, step). The driver calls the functions in HEAD's form -- the
// locale "en" as the last argument and locale: "en" in the send dependencies;
// the merge-base functions ignore the extra argument and property. The counts
// stay under 1000 so that the intended digit grouping is not part of the
// comparison. Exits 0 when the two outputs are identical, 1 when they differ
// (printing the differing lines), and 2 when the check cannot run.

import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const driver = `
import { rejectionReasons } from "./src/core/types.ts";
import { preflightReasons } from "./src/core/prepare.ts";
import { serverRejectStatus } from "./src/core/response.ts";
import { renderMetadata, renderSummary } from "./src/ui/render.ts";
import { renderBlockReason, renderSendOutcome } from "./src/ui/renderSend.ts";
import { runPopupSend } from "./src/launch.ts";
import { container } from "./test/helpers/dom.ts";
import { deferredNative, responseReply, testSendDeps } from "./test/helpers/native.ts";

const lines = [];
function dump(title, root) {
  lines.push("== " + title);
  for (const element of root.querySelectorAll("*")) {
    if (element.children.length > 0) continue;
    const classes = element.className ? "." + element.className.split(/\\s+/).join(".") : "";
    lines.push(element.tagName.toLowerCase() + classes + ": " + element.textContent);
  }
}

const requestId = "0123456789abcdef";
for (const reason of Object.keys(rejectionReasons)) {
  const root = container();
  renderSummary(root, { kind: "rejected", reason }, "en");
  dump("renderSummary rejected " + reason, root);
}
{
  const root = container();
  renderSummary(root, {
    kind: "collected", title: "A video", titleTruncated: false,
    url: "https://www.youtube.com/watch?v=abc", urlTruncated: false,
    characterCount: 123, lineCount: 4, preview: "00:00 hello", previewTruncated: false,
  }, "en");
  dump("renderSummary collected", root);
}
{
  const root = container();
  renderMetadata(root, {
    values: { title: "A title", channelName: "A channel", description: "A description" },
    showExpandPrompt: true, omissions: [],
  }, "en");
  dump("renderMetadata", root);
}
const unexpectedChecks = [
  "extension-timeout", "envelope-shape", "envelope-version", "host-protocol-error",
  "body-over-limit", "encoding", "escape", "json-shape", "duplicate-member",
  "member-set", "request-id", "status-pairing", "classification-error",
];
const outcomes = [
  { kind: "accepted", requestId },
  ...Object.keys(serverRejectStatus).map((reason) => ({ kind: "rejected", reason, requestId })),
  { kind: "not-sent", cause: "server-unavailable" },
  { kind: "not-sent", cause: "message-too-large" },
  { kind: "no-response" },
  { kind: "host-failed" },
  ...unexpectedChecks.map((check) => ({ kind: "unexpected", check })),
];
for (const outcome of outcomes) {
  const root = container();
  renderSendOutcome(root, outcome, "en");
  dump("renderSendOutcome " + JSON.stringify(outcome), root);
}
for (const reason of [...Object.keys(rejectionReasons), ...Object.keys(preflightReasons)]) {
  const root = container();
  renderBlockReason(root, reason, "en");
  dump("renderBlockReason " + reason, root);
}
{
  const root = container();
  const native = deferredNative();
  const silent = { info: () => {}, error: () => {} };
  await runPopupSend(root, {
    outcome: { kind: "collected", input: { selection: "00:00 hello", url: "https://www.youtube.com/watch?v=abc", title: "A video" } },
    metadata: { values: {}, showExpandPrompt: false, omissions: [] },
  }, { send: testSendDeps(native.sendNative), now: () => 1000, log: silent, locale: "en" });
  dump("runPopupSend before click", root);
  root.querySelector(".send-button").click();
  await new Promise((resolve) => setTimeout(resolve, 0));
  dump("runPopupSend sending", root);
  native.resolve(responseReply(202, JSON.stringify({ status: "accepted", requestId })));
  await new Promise((resolve) => setTimeout(resolve, 0));
}
process.stdout.write(lines.join("\\n") + "\\n");
`;

function git(...args: string[]): string {
  return execFileSync("git", args, { encoding: "utf8" }).trim();
}

function fail(message: string): never {
  process.stderr.write(`english_text: ${message}\n`);
  process.exit(2);
}

/** Extracts rev's extension sources into a temporary directory and runs the driver there. */
function render(rev: string, nodeModules: string): string {
  const dir = mkdtempSync(path.join(tmpdir(), "english-text-"));
  try {
    const archive = execFileSync("git", [
      "archive",
      rev,
      "extension/src",
      "extension/test/helpers",
      "extension/package.json",
    ]);
    execFileSync("tar", ["-x", "-C", dir], { input: archive });
    const extension = path.join(dir, "extension");
    symlinkSync(nodeModules, path.join(extension, "node_modules"), "dir");
    const script = path.join(extension, "english-text-driver.ts");
    writeFileSync(script, driver);
    const result = spawnSync(process.execPath, [script], {
      cwd: extension,
      encoding: "utf8",
    });
    if (result.status !== 0) {
      fail(`the driver failed at ${rev}:\n${result.stderr}`);
    }
    return result.stdout;
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

const root = git("rev-parse", "--show-toplevel");
process.chdir(root);
const head = git("rev-parse", "HEAD");
const base = git("merge-base", "origin/main", "HEAD");
const lockfile = "extension/package-lock.json";
if (git("rev-parse", `${base}:${lockfile}`) !== git("rev-parse", `${head}:${lockfile}`)) {
  fail(`${lockfile} differs between ${base} and ${head}`);
}
const nodeModules = path.join(root, "extension", "node_modules");

const before = render(base, nodeModules).split("\n");
const after = render(head, nodeModules).split("\n");
process.stdout.write(`merge-base ${base}\nhead       ${head}\n`);
const differing: string[] = [];
for (let i = 0; i < Math.max(before.length, after.length); i++) {
  if (before[i] !== after[i]) {
    differing.push(`line ${i + 1}\n  merge-base: ${before[i] ?? "(none)"}\n  head:       ${after[i] ?? "(none)"}`);
  }
}
if (differing.length > 0) {
  process.stdout.write(`FAIL: the displayed English text differs\n${differing.join("\n")}\n`);
  process.exit(1);
}
const elements = after.filter((line) => line !== "" && !line.startsWith("== ")).length;
process.stdout.write(`PASS: ${elements} displayed elements identical\n`);
