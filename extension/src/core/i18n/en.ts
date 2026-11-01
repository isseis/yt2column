import type { Catalog } from "./catalog.ts";
import { formatCount } from "./locale.ts";

/** Formats a count and the English noun, singular at exactly one. */
function englishCount(count: number, noun: string): string {
  return `${formatCount(count, "en")} ${noun}${count === 1 ? "" : "s"}`;
}

// Steps several job API messages share.
const sameVersionStep =
  "Update the extension, the host and the server to the same build; run yt2column-nmhost install again from the built binary.";
const startServerStep =
  "Start yt2column-server. A server that is stopping waits for its jobs, and a new one cannot start until they finish.";
const serverPermissionsStep =
  "If the server is running, check that it runs as your user and that its socket directory has the permissions README.en.md describes.";
const installHostStep =
  "Most often the host is not installed or its binary was moved: run yt2column-nmhost install chrome (or brave) from the built binary, and again after moving it.";
// The server returns the existing article of the same input while it is
// generating or done; any other case generates again, so this is not promised.
const sendSameInputStep =
  "You can send the same input again from the popup. While the server is generating or holds that article, it returns it instead of generating it again; a changed input, a failed article or a restarted server generates it again.";
const sendAgainStep =
  "Send the same input again from the popup to generate the article again.";
const repeatedPostStep =
  "If an earlier copy of this article was posted, posting the new one repeats it in Slack.";
const checksAgainStep = "The status is checked again automatically.";
const setWebhookStep = "Set SLACK_WEBHOOK_URL and restart yt2column-server.";
const publishUnknownStep = "The posting status is checked again automatically.";
const publishAgainStep =
  "Pressing the button again posts nothing twice only while the server holds the article as posting or posted; after a failed post, posting again may repeat the one message whose delivery was not confirmed.";
const closePopupStep =
  "You can close this popup: the result appears in the viewer tab.";

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
  jobReject: {
    too_many_requests: {
      text: "The server is busy with other articles, so it did not start this one.",
      steps: ["Wait a while, then send again from the popup."],
    },
    body_too_large: {
      text: "The server rejected the request as too large.",
      steps: ["Select fewer caption lines and try again."],
    },
    invalid_body: {
      text: "The server did not accept the request.",
      steps: [
        sameVersionStep,
        "If they already come from the same build, the extension may have a bug.",
      ],
    },
    invalid_video_url: {
      text: "The server did not accept the video URL.",
      steps: [],
    },
    invalid_transcript: {
      text: "The selected text is not in the transcript panel's caption format.",
      steps: ["Open the transcript panel, select caption lines and try again."],
    },
    invalid_key: {
      text: "The server did not accept the article key.",
      steps: [sameVersionStep, "Send the article again from the popup."],
    },
    webhook_not_configured: {
      text: "The server cannot post: it has no Slack Webhook configured.",
      steps: [setWebhookStep],
    },
    article_not_ready: {
      text: "The article is not finished yet, so it cannot be posted.",
      steps: ["Post it once the article is shown."],
    },
    article_not_found: {
      text: "The server no longer has this article, so it cannot be posted.",
      steps: [sendAgainStep, repeatedPostStep],
    },
    article_not_postable: {
      text: "The server refused to post this article.",
      steps: [],
    },
    internal_error: {
      text: "The server hit an internal error.",
      steps: ["Check the server log."],
    },
  },
  startExchange: {
    notSent: {
      "server-unavailable": {
        text: "Could not connect to the yt2column server, so nothing was sent.",
        steps: [startServerStep, serverPermissionsStep],
      },
      "message-too-large": {
        text: "The request is too large for the host, so nothing was sent.",
        steps: ["Select fewer caption lines and try again."],
      },
    },
    noResponse: {
      text: "The server did not respond in time. It may have received the request and be generating the article.",
      steps: [sendSameInputStep],
    },
    hostFailed: {
      text: "The native messaging host could not be started, or it exited without replying. The request may have been sent.",
      steps: [installHostStep, sendSameInputStep],
    },
    unexpected: {
      text: "Could not interpret the reply. The server may have received the request and be generating the article.",
      steps: [sameVersionStep, sendSameInputStep],
    },
  },
  statusExchange: {
    notSent: {
      "server-unavailable": {
        text: "Could not connect to the yt2column server to read the article's status.",
        steps: [startServerStep, checksAgainStep],
      },
      "message-too-large": {
        text: "The host refused the status request as too large, so status checks stopped.",
        steps: [sameVersionStep, "Send the article again from the popup."],
      },
    },
    noResponse: {
      text: "The server did not answer the status request in time.",
      steps: [checksAgainStep],
    },
    hostFailed: {
      text: "The native messaging host could not be started to read the status, or it exited without replying.",
      steps: [installHostStep, checksAgainStep],
    },
    unexpected: {
      text: "Could not interpret the status reply, so status checks stopped.",
      steps: [sameVersionStep, "Send the article again from the popup."],
    },
  },
  publishExchange: {
    notSent: {
      "server-unavailable": {
        text: "Could not connect to the yt2column server, so the article was not posted.",
        steps: [startServerStep, "Then post again."],
      },
      "message-too-large": {
        text: "The host refused the publish request as too large, so the article was not posted.",
        steps: [sameVersionStep],
      },
    },
    noResponse: {
      text: "The server did not answer the publish request in time. It may be posting the article.",
      steps: [publishUnknownStep, publishAgainStep],
    },
    hostFailed: {
      text: "The native messaging host could not be started, or it exited without replying. The publish request may have been sent.",
      steps: [installHostStep, publishUnknownStep, publishAgainStep],
    },
    unexpected: {
      text: "Could not interpret the reply to the publish request. The server may be posting the article.",
      steps: [sameVersionStep, publishUnknownStep, publishAgainStep],
    },
  },
  startResult: {
    accepted: {
      text: "The server accepted the article. Its progress appears in the viewer tab.",
      steps: [],
    },
    noResult: {
      text: "The result of the send did not arrive, so it is unknown whether the server started the article.",
      steps: [sendSameInputStep],
    },
  },
  failure: {
    internal: {
      text: "The server failed while generating the article.",
      steps: ["Check the server log.", sendAgainStep],
    },
    transcript_too_long: {
      text: "The transcript is too long for the prompt.",
      steps: ["Select fewer caption lines and send again from the popup."],
    },
    llm_timeout: {
      text: "The LLM call timed out.",
      steps: [sendAgainStep],
    },
    llm_empty: {
      text: "The LLM returned an empty response.",
      steps: [sendAgainStep],
    },
    llm_truncated: {
      text: "The LLM's response was cut off before it ended.",
      steps: [sendAgainStep, "If it happens again, check the server log."],
    },
    llm_failed: {
      text: "The LLM call failed.",
      steps: ["Check the server log for the provider's error.", sendAgainStep],
    },
    article_invalid: {
      text: "The generated article did not pass the server's completion check.",
      steps: [sendAgainStep],
    },
    article_too_large: {
      text: "The generated article is too large to show.",
      steps: ["Select fewer caption lines and send again from the popup."],
    },
  },
  stepNames: {
    draft: "Draft",
    shorten: "Shorten",
    review: "Review",
  },
  stepStates: {
    pending: "Not started",
    running: "In progress",
    done: "Done",
    skipped: "Skipped",
    failed: "Failed",
  },
  publishStates: {
    not_posted: { text: "Not posted to Slack yet.", steps: [] },
    posting: { text: "Posting to Slack...", steps: [] },
    posted: { text: "Posted to Slack.", steps: [] },
    failed: {
      text: "The last post to Slack stopped before it finished.",
      steps: [
        "Posting again resumes from the first message that did not arrive; the one message whose delivery was not confirmed may arrive twice.",
      ],
    },
    unpostable: { text: "This article cannot be posted to Slack.", steps: [] },
  },
  publishReasons: {
    mention: {
      text: "The article contains a mention, which the server does not post.",
      steps: [],
    },
    other: {
      text: "The server's check before posting refused the article.",
      steps: [],
    },
    webhook: {
      text: "Slack did not confirm that it received a message.",
      steps: [],
    },
    internal: {
      text: "The posting job ended abnormally.",
      steps: ["Check the server log."],
    },
  },
  viewer: {
    sending: { text: "Sending the request to the server...", steps: [] },
    loading: { text: "Loading the article's status...", steps: [] },
    invalidLink: {
      text: "This viewer link is not valid, so no article is shown.",
      steps: ["Send the article again from the popup."],
    },
    running: "Generating the article",
    lost: {
      text: "The server no longer has this article: it restarted, or dropped the article from its cache.",
      steps: [sendAgainStep, repeatedPostStep],
    },
    webhookNotConfigured: {
      text: "The server has no Slack Webhook configured.",
      steps: [setWebhookStep],
    },
    videoLink: "Open the video on YouTube",
    elapsed: "Elapsed",
    duration: "Time taken",
    cost: "Estimated cost",
    costUnknown: "Unknown",
    publish: "Post to Slack",
    save: "Save as .md",
    saved: "The download started.",
    copy: "Copy Markdown",
    copied: "Copied the Markdown.",
    copyFailed: "Could not copy the Markdown.",
  },
  popup: {
    primary: {
      generate: "Generate article",
      openViewer: "Open viewer",
      retry: "Try again",
    },
    sending: {
      text: "Sending the request to the server...",
      steps: [closePopupStep],
    },
    viewerNotice: { text: closePopupStep, steps: [] },
    articleReady: { text: "The article is ready.", steps: [] },
  },
};
