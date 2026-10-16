// English messages for every way a send path ends: a launch that sends
// nothing, a server rejection, and a send outcome. Each switch ends in never,
// so adding a reason without a message fails the type check.

import { rejectionMessage } from "./messages.ts";
import type { RejectionMessage } from "./messages.ts";
import type { BlockReason } from "./prepare.ts";
import type { SendOutcome, ServerRejectReason } from "./response.ts";

/** Message for a launch that ends without a request. Collection reasons use rejectionMessage. */
export function blockMessage(reason: BlockReason): RejectionMessage {
  switch (reason) {
    case "not-watch-page":
    case "collection-failed":
    case "empty-selection":
    case "empty-title":
      return rejectionMessage(reason);
    case "settings-unreadable":
      return {
        text: "The extension could not read its settings.",
        steps: [
          "Launch yt2column again.",
          "If it keeps failing, open the options page and save the settings again.",
        ],
      };
    case "token-missing":
      return {
        text: "No shared token is set.",
        steps: [
          "Right-click the toolbar icon and open Options.",
          "Paste the token the server printed at startup (or the token file's contents) and save it.",
          "Launch yt2column again.",
        ],
      };
    case "token-invalid":
      return {
        text: "The stored shared token does not have the right form.",
        steps: [
          "Right-click the toolbar icon and open Options.",
          "Paste the token the server printed at startup (or the token file's contents) and save it.",
          "Launch yt2column again.",
        ],
      };
    case "port-invalid":
      return {
        text: "The stored server port is not valid.",
        steps: [
          "Open the options page.",
          "Enter the server port YT2COLUMN_SERVER_PORT uses (8765 if unchanged) and save it.",
        ],
      };
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
    case "unauthorized":
      return {
        text: "The shared token does not match the server.",
        steps: [
          "Check the token the server printed at startup.",
          "Paste it into the options page and save it.",
        ],
      };
    case "forbidden_origin":
      return {
        text: "The server did not accept this extension.",
        steps: [
          "The loaded extension's ID may differ from the one the server allows.",
        ],
      };
    case "forbidden_host":
      return {
        text: "The server did not accept the request host.",
        steps: [
          "Check the port in the options page; another process may be answering.",
        ],
      };
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
    case "not_found":
      return {
        text: "No article endpoint was found at the server address.",
        steps: [
          "Check the port in the options page; the server may not be running.",
        ],
      };
    case "method_not_allowed":
      return {
        text: "The server did not accept POST at the article endpoint.",
        steps: [
          "Check the port in the options page; another process may be using it.",
        ],
      };
    case "unsupported_media_type":
      return {
        text: "The server did not accept the request content type.",
        steps: [
          "Check the port in the options page; another process may be using it.",
        ],
      };
    default: {
      const exhaustive: never = reason;
      throw new Error(`Unhandled server reject reason: ${String(exhaustive)}`);
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
    case "unknown":
      return {
        text: "Could not get a response from the server. It may not be running, or it may have received and be processing the request.",
        steps: [
          "Check Slack for the article or a failure notice.",
          "If nothing arrived, make sure the server is running, then send again.",
          "If it keeps failing, check the server log around this time.",
        ],
      };
    case "unexpected":
      return {
        text: "Could not interpret the server's response.",
        steps: [
          "Check that the server version matches the extension, and that the port in the options page is correct.",
          "Check Slack before sending again.",
        ],
      };
    case "redirected":
      return {
        text: "The server responded with a redirect.",
        steps: [
          "Check that the port in the options page is correct.",
          "Check Slack before sending again.",
        ],
      };
    default: {
      const exhaustive: never = outcome;
      throw new Error(`Unhandled send outcome: ${String(exhaustive)}`);
    }
  }
}

/** Message shown when a result window cannot read what to display. */
export const unreadableResultMessage: RejectionMessage = {
  text: "This result cannot be displayed. If you sent an article, check Slack before sending again.",
  steps: [],
};
