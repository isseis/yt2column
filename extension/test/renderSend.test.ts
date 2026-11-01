import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { locales } from "../src/core/i18n/locale.ts";
import type { Locale } from "../src/core/i18n/locale.ts";
import type { SendOutcome } from "../src/core/articlesResponse.ts";
import type { RequestId } from "../src/core/response.ts";
import { sendOutcomeMessage } from "../src/core/sendMessages.ts";
import { renderSendOutcome } from "../src/ui/renderSend.ts";
import { container, expectLocaleMessage, query } from "./helpers/dom.ts";

const requestId = "0123456789abcdef" as RequestId;
const localeList = Object.keys(locales) as Locale[];
const malicious = "<img src=x onerror=alert(1)>";

describe("renderSendOutcome", () => {
  const accepted: SendOutcome = { kind: "accepted", requestId };
  const rejected: SendOutcome = {
    kind: "rejected",
    reason: "invalid_transcript",
    requestId,
  };

  for (const locale of localeList) {
    const withRequestId = [accepted, rejected];
    for (const outcome of withRequestId) {
      it(`shows the ${locale} ${outcome.kind} message and its request id`, () => {
        const root = container();
        renderSendOutcome(root, outcome, locale);
        expectLocaleMessage(root, locale, (l) =>
          sendOutcomeMessage(outcome, l),
        );
        assert.equal(query(root, ".send-request-id").textContent, requestId);
      });
    }

    const withoutRequestId: SendOutcome[] = [
      { kind: "not-sent", cause: "server-unavailable" },
      { kind: "no-response" },
      { kind: "host-failed" },
    ];
    for (const outcome of withoutRequestId) {
      it(`shows the ${locale} ${outcome.kind} message and no request id`, () => {
        const root = container();
        renderSendOutcome(root, outcome, locale);
        expectLocaleMessage(root, locale, (l) =>
          sendOutcomeMessage(outcome, l),
        );
        assert.equal(root.querySelector(".send-request-id"), null);
      });
    }

    it(`shows the ${locale} unexpected message but not the check`, () => {
      const root = container();
      const outcome: SendOutcome = { kind: "unexpected", check: "json-shape" };
      renderSendOutcome(root, outcome, locale);
      expectLocaleMessage(root, locale, (l) => sendOutcomeMessage(outcome, l));
      assert.equal((root.textContent ?? "").includes("json-shape"), false);
    });

    it(`displays a request id with markup as text, not elements, in ${locale}`, () => {
      // A RequestId is validated before it reaches here; the cast checks the
      // rendering alone.
      const root = container();
      renderSendOutcome(
        root,
        { kind: "accepted", requestId: malicious as RequestId },
        locale,
      );
      assert.equal(root.querySelector("img"), null);
      assert.equal(query(root, ".send-request-id").textContent, malicious);
    });

    it(`replaces the previous content on each call in ${locale}`, () => {
      const root = container();
      renderSendOutcome(root, accepted, locale);
      renderSendOutcome(root, { kind: "no-response" }, locale);
      assert.equal(root.querySelectorAll(".send-request-id").length, 0);
      expectLocaleMessage(root, locale, (l) =>
        sendOutcomeMessage({ kind: "no-response" }, l),
      );
    });
  }
});
