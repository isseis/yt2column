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
const checksAgainStep = "The status is checked again automatically.";
// Button names a step quotes, kept in one place with the button's own name.
const generateButton = "Generate article";
const generateFromSelectionButton = "Generate from current selection";
const openExistingButton = "Open existing article";
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
      steps: [sendAgainStep],
    },
    existing: {
      text: "This viewer shows an existing article of the same video. It was not generated from your current selection.",
      steps: [
        `To generate from your current selection, open the popup on the video's tab and press "${generateButton}".`,
        `If the popup offers a choice, choose "${generateFromSelectionButton}".`,
      ],
    },
    videoLink: "Open the video on YouTube",
    elapsed: "Elapsed",
    duration: "Time taken",
    cost: "Estimated cost",
    costUnknown: "Unknown",
    save: "Save as .md",
    saved: "The download started.",
    copy: "Copy Markdown",
    copied: "Copied the Markdown.",
    copiedNeutralized:
      "Copied the Markdown. Invisible characters were inserted after @ so that they do not become mentions.",
    copyFailed: "Could not copy the Markdown.",
  },
  popup: {
    primary: {
      generate: generateButton,
      openViewer: "Open viewer",
      retry: "Try again",
      openExisting: openExistingButton,
    },
    secondary: generateFromSelectionButton,
    sending: {
      text: "Sending the request to the server...",
      steps: [closePopupStep],
    },
    viewerNotice: { text: closePopupStep, steps: [] },
    articleReady: { text: "The article is ready.", steps: [] },
    choice: {
      text: "This server has an article of the same video.",
      steps: [
        `"${openExistingButton}" shows it. "${generateFromSelectionButton}" generates a new article from the captions you selected.`,
      ],
    },
    existingStates: {
      loading: "Loading the article's status...",
      running: "The article is being generated.",
      done: "The article is ready.",
    },
    choiceEnded: {
      text: "The article of the same video can no longer be used.",
      steps: [`Press "${generateButton}" to generate the article.`],
    },
  },
};
