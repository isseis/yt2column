// English messages for every way a send path ends: a launch that sends
// nothing, a server rejection, and a send outcome. Each switch ends in never,
// so adding a reason without a message fails the type check. A message for a
// request known not to have reached the server never asks the user to check
// Slack; every message for a result that is not known does.

import { rejectionMessage } from "./messages.ts";
import type { RejectionMessage } from "./messages.ts";
import type { BlockReason } from "./prepare.ts";
import type {
  NotSentCause,
  SendOutcome,
  ServerRejectReason,
  UnexpectedCheck,
} from "./response.ts";

/** Message for a launch that ends without a request. Collection reasons use rejectionMessage. */
export function blockMessage(reason: BlockReason): RejectionMessage {
  switch (reason) {
    case "not-watch-page":
    case "collection-failed":
    case "empty-selection":
    case "empty-title":
      return rejectionMessage(reason);
    case "body-too-large":
      return {
        text: "The selected text is too large (over 4 MiB).",
        steps: ["Select fewer caption lines and try again."],
      };
    default: {
      const exhaustive: never = reason;
      throw new Error(`Unhandled block reason: ${String(exhaustive)}`);
    }
  }
}

/** Message for a server rejection; distinct for every reason. */
export function serverRejectMessage(
  reason: ServerRejectReason,
): RejectionMessage {
  switch (reason) {
    case "too_many_requests":
      return {
        text: "The server is already processing another article.",
        steps: [
          "Check Slack for a notification from the previous send before sending again.",
          "Wait a while, then send again.",
        ],
      };
    case "body_too_large":
      return {
        text: "The server rejected the body as too large.",
        steps: [
          "Select fewer caption lines and try again.",
          "The server may also have posted a failure notice to Slack.",
        ],
      };
    case "invalid_body":
      return {
        text: "The server did not accept the request body (the extension may have a bug).",
        steps: ["The server may also have posted a failure notice to Slack."],
      };
    case "invalid_video_url":
      return {
        text: "The server did not accept the video URL.",
        steps: ["The server may also have posted a failure notice to Slack."],
      };
    case "invalid_transcript":
      return {
        text: "The selected text is not in the transcript panel's caption format.",
        steps: [
          "Open the transcript panel, select caption lines and try again.",
          "The server may also have posted a failure notice to Slack.",
        ],
      };
    case "internal_error":
      return {
        text: "The server hit an internal error.",
        steps: [
          "Check the server log.",
          "The server may also have posted a failure notice to Slack.",
        ],
      };
    default: {
      const exhaustive: never = reason;
      throw new Error(`Unhandled server reject reason: ${String(exhaustive)}`);
    }
  }
}

/** Message for a request known not to have reached the server; no Slack check. */
function notSentMessage(cause: NotSentCause): RejectionMessage {
  switch (cause) {
    case "server-unavailable":
      return {
        text: "Could not connect to the yt2column server, so nothing was sent.",
        steps: [
          "Start yt2column-server. A server that is stopping waits for its jobs, and a new one cannot start until they finish.",
          "If the server is running, check that it runs as your user and that its socket directory has the permissions the README describes.",
        ],
      };
    case "message-too-large":
      return {
        text: "The request is too large for the host, so nothing was sent.",
        steps: ["Select fewer caption lines and try again."],
      };
    default: {
      const exhaustive: never = cause;
      throw new Error(`Unhandled not-sent cause: ${String(exhaustive)}`);
    }
  }
}

/** The one message for every unexpected reply; the check is for the developer log only. */
function unexpectedMessage(check: UnexpectedCheck): RejectionMessage {
  switch (check) {
    case "extension-timeout":
    case "envelope-shape":
    case "envelope-version":
    case "host-protocol-error":
    case "body-over-limit":
    case "encoding":
    case "escape":
    case "json-shape":
    case "duplicate-member":
    case "member-set":
    case "request-id":
    case "status-pairing":
    case "classification-error":
      return {
        text: "Could not interpret the reply. The server may have received the request and be processing it.",
        steps: [
          "Check that the host, the server and the extension come from the same build; run yt2column-nmhost install again from the built binary.",
          "Check Slack for the article or a failure notice before sending again.",
        ],
      };
    default: {
      const exhaustive: never = check;
      throw new Error(`Unhandled unexpected check: ${String(exhaustive)}`);
    }
  }
}

/** Message for every send outcome; rejected delegates to serverRejectMessage. */
export function sendOutcomeMessage(outcome: SendOutcome): RejectionMessage {
  switch (outcome.kind) {
    case "accepted":
      return {
        text: "The server accepted the article. The result of generating and posting it will arrive in Slack.",
        steps: [],
      };
    case "rejected":
      return serverRejectMessage(outcome.reason);
    case "not-sent":
      return notSentMessage(outcome.cause);
    case "no-response":
      return {
        text: "The server did not respond in time. It may have received the request and be processing it.",
        steps: [
          "Check Slack for the article or a failure notice before sending again.",
          "If nothing arrives, check the server log around this time.",
        ],
      };
    case "host-failed":
      return {
        text: "The native messaging host could not be started, or it exited without replying.",
        steps: [
          "Most often the host is not installed or its binary was moved: run yt2column-nmhost install chrome (or brave) from the built binary, and again after moving it.",
          "The request may already have been sent: check Slack for the article or a failure notice before sending again.",
        ],
      };
    case "unexpected":
      return unexpectedMessage(outcome.check);
    default: {
      const exhaustive: never = outcome;
      throw new Error(`Unhandled send outcome: ${String(exhaustive)}`);
    }
  }
}
