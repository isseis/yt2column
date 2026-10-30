import type { Catalog } from "./catalog.ts";
import { formatCount } from "./locale.ts";

/** Formats a count and the English noun, singular at exactly one. */
function englishCount(count: number, noun: string): string {
  return `${formatCount(count, "en")} ${noun}${count === 1 ? "" : "s"}`;
}

/** The English catalog. */
export const en: Catalog = {
  rejection: {
    "not-watch-page": {
      text: "This is not a YouTube watch page.",
      steps: [
        "Open a video page such as https://www.youtube.com/watch?v=...",
        "Open the transcript panel and select caption lines.",
        "Launch yt2column again.",
      ],
    },
    "empty-selection": {
      text: "No text is selected.",
      steps: [
        'Open the transcript panel with "Show transcript".',
        "Select the caption lines.",
        "Launch yt2column again.",
      ],
    },
    "empty-title": { text: "The tab has no title.", steps: [] },
    "collection-failed": {
      text: "The selection could not be read from this tab (the page may have changed).",
      steps: [],
    },
  },
  preflight: {
    "body-too-large": {
      text: "The selected text is too large (over 4 MiB).",
      steps: ["Select fewer caption lines and try again."],
    },
  },
  serverReject: {
    too_many_requests: {
      text: "The server is already processing another article.",
      steps: [
        "Check Slack for a notification from the previous send before sending again.",
        "Wait a while, then send again.",
      ],
    },
    body_too_large: {
      text: "The server rejected the body as too large.",
      steps: [
        "Select fewer caption lines and try again.",
        "The server may also have posted a failure notice to Slack.",
      ],
    },
    invalid_body: {
      text: "The server did not accept the request body (the extension may have a bug).",
      steps: ["The server may also have posted a failure notice to Slack."],
    },
    invalid_video_url: {
      text: "The server did not accept the video URL.",
      steps: ["The server may also have posted a failure notice to Slack."],
    },
    invalid_transcript: {
      text: "The selected text is not in the transcript panel's caption format.",
      steps: [
        "Open the transcript panel, select caption lines and try again.",
        "The server may also have posted a failure notice to Slack.",
      ],
    },
    internal_error: {
      text: "The server hit an internal error.",
      steps: [
        "Check the server log.",
        "The server may also have posted a failure notice to Slack.",
      ],
    },
  },
  notSent: {
    "server-unavailable": {
      text: "Could not connect to the yt2column server, so nothing was sent.",
      steps: [
        "Start yt2column-server. A server that is stopping waits for its jobs, and a new one cannot start until they finish.",
        "If the server is running, check that it runs as your user and that its socket directory has the permissions README.en.md describes.",
      ],
    },
    "message-too-large": {
      text: "The request is too large for the host, so nothing was sent.",
      steps: ["Select fewer caption lines and try again."],
    },
  },
  accepted: {
    text: "The server accepted the article. The result of generating and posting it will arrive in Slack.",
    steps: [],
  },
  noResponse: {
    text: "The server did not respond in time. It may have received the request and be processing it.",
    steps: [
      "Check Slack for the article or a failure notice before sending again.",
      "If nothing arrives, check the server log around this time.",
    ],
  },
  hostFailed: {
    text: "The native messaging host could not be started, or it exited without replying.",
    steps: [
      "Most often the host is not installed or its binary was moved: run yt2column-nmhost install chrome (or brave) from the built binary, and again after moving it.",
      "The request may already have been sent: check Slack for the article or a failure notice before sending again.",
    ],
  },
  unexpected: {
    text: "Could not interpret the reply. The server may have received the request and be processing it.",
    steps: [
      "Check that the host, the server and the extension come from the same build; run yt2column-nmhost install again from the built binary.",
      "Check Slack for the article or a failure notice before sending again.",
    ],
  },
  sending: {
    text: "Sending the article to the server...",
    steps: [
      "Do not close this window until the result appears.",
      "If you close it, check Slack before sending again.",
    ],
  },
  expandPrompt: {
    text: "Collapsed, so only the beginning will be sent.",
    steps: ['Click "...more" below the video.', "Reopen this popup."],
  },
  checkNames: {
    selection: "Selected captions",
    description: "Description",
  },
  checkStates: {
    ok: "OK",
    warning: "Warning",
  },
  selectionSize: (characters, lines) =>
    `${englishCount(characters, "character")} \u00b7 ${englishCount(lines, "line")}`,
  descriptionFull: (characters) =>
    `Full text \u00b7 ${englishCount(characters, "character")}`,
  descriptionNotSent: {
    text: "Not available, so it will not be sent.",
    steps: [],
  },
  details: {
    summary: "Details",
    url: "URL",
    tabTitle: "Tab title",
  },
  primaryButton: "Generate article",
};
