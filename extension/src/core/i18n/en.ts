import type { Catalog } from "./catalog.ts";
import { formatCount } from "./locale.ts";

/** Formats a count and the English noun, singular at exactly one. */
function englishCount(count: number, noun: string): string {
  return `${formatCount(count, "en")} ${noun}${count === 1 ? "" : "s"}`;
}

// Button names a step quotes, kept in one place with the button's own name.
const generateButton = "Generate article";
const generateFromSelectionButton = "Generate from current selection";
const openExistingButton = "Open existing article";
const setupButton = "Set API key";
// Steps several messages share.
const pressAgainStep = `On the video's tab, open the popup and press "${generateButton}" again.`;
const fewerLinesStep =
  "Select fewer caption lines, then open the popup on the video's tab and press the button again.";
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
        "Launch YomuDoga again.",
      ],
    },
    "empty-selection": {
      text: "No text is selected.",
      steps: [
        'Open the transcript panel with "Show transcript".',
        "Select the caption lines.",
        "Launch YomuDoga again.",
      ],
    },
    "empty-title": { text: "The tab has no title.", steps: [] },
    "collection-failed": {
      text: "The selection could not be read from this tab (the page may have changed).",
      steps: [],
    },
    "settings-unprotected": {
      text: "The extension could not keep its saved settings from web pages, so it did not read this tab.",
      steps: [
        "Close this popup and open it again.",
        "If it happens again, reload the extension.",
      ],
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
  startRejected: {
    invalid_body: {
      text: "The selected text was not accepted as captions of this video, so no article was started.",
      steps: ["Open the transcript panel, select caption lines and try again."],
    },
    too_many_requests: {
      text: "Two articles are being generated already, so this one was not started.",
      steps: ["Wait until one of them ends, then press the button again."],
    },
    internal: {
      text: "The extension hit an internal error, so no article was started.",
      steps: [pressAgainStep, "If it happens again, reload the extension."],
    },
  },
  startUnknown: {
    text: "The result of the start did not arrive, so it is unknown whether the article was started.",
    steps: [
      "Check the viewer tab for the article's state.",
      "Pressing the button again with the same selection does not generate it twice while that article is being generated or is done. In any other case it starts a new article.",
    ],
  },
  inputUnavailable: {
    text: "The viewer could not take the selected captions, so no article was started and nothing was charged.",
    steps: [pressAgainStep],
  },
  setup: {
    api_key_missing: {
      text: "An Anthropic API key needs to be set to generate articles.",
      steps: [`Press "${setupButton}" and save your API key.`, pressAgainStep],
    },
    settings_invalid: {
      text: "The saved API key or workspace ID is not in a usable form.",
      steps: [
        `Press "${setupButton}" and save your API key and workspace ID again.`,
        pressAgainStep,
      ],
    },
  },
  keyRejected: {
    text: "Anthropic rejected the API key: it may have expired, been revoked or be wrong, or your organization may not allow calls from a browser.",
    steps: [
      `Press "${setupButton}" and save a new API key.`,
      `${pressAgainStep} With the same selection, the article is generated again under the same job.`,
    ],
  },
  failure: {
    internal: {
      text: "The extension hit an internal error while generating the article.",
      steps: [pressAgainStep],
    },
    transcript_too_long: {
      text: "The transcript is too long for the prompt.",
      steps: [fewerLinesStep],
    },
    llm_timeout: {
      text: "The LLM call timed out.",
      steps: [pressAgainStep],
    },
    llm_empty: {
      text: "The LLM returned an empty response.",
      steps: [pressAgainStep],
    },
    llm_truncated: {
      text: "The LLM's response was cut off before it ended.",
      steps: [pressAgainStep],
    },
    llm_failed: {
      text: "The LLM call failed.",
      steps: [
        `Wait a while, then on the video's tab, open the popup and press "${generateButton}" again.`,
        "If it keeps failing, check your usage limits and usage in the Claude Console.",
      ],
    },
    article_invalid: {
      text: "The generated article did not pass the completion check.",
      steps: [pressAgainStep],
    },
    article_too_large: {
      text: "The generated article is too large to keep.",
      steps: [fewerLinesStep],
    },
    run_timeout: {
      text: "The article was not finished within 50 minutes, so it was stopped.",
      steps: [pressAgainStep],
    },
  },
  stopped: {
    text: "The article stopped with its page: the viewer tab running it was closed or reloaded, or the extension lost contact with it.",
    steps: [pressAgainStep],
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
    interrupted: "Interrupted",
  },
  viewer: {
    preparing: { text: "Preparing the article...", steps: [] },
    loading: { text: "Loading the article's state...", steps: [] },
    invalidLink: {
      text: "This viewer link is not valid, so no article is shown.",
      steps: [pressAgainStep],
    },
    running: "Generating the article",
    lost: {
      text: "This article is no longer kept: the browser was closed, the extension was reloaded or updated, or the article was dropped to make room for newer ones.",
      steps: [
        "Open the video from the link above, select caption lines and generate the article again.",
      ],
    },
    unstored: {
      text: "The article is done, but the extension could not keep it: only this page holds it, and reloading or closing the page loses it.",
      steps: ["Save it or copy it now."],
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
  options: {
    saved: { text: "The API key is set.", steps: [] },
    saved_unverified: {
      text: "The API key is set, but it could not be checked with Anthropic.",
      steps: [
        "If an article later fails because Anthropic rejected the key, set a new key here.",
      ],
    },
    field_rejected: {
      text: "The marked field is not in an accepted form. Nothing was saved.",
      steps: [
        "Enter the API key as Anthropic shows it, without spaces.",
        "Leave the workspace ID empty, or enter it without spaces.",
      ],
    },
    key_rejected: {
      text: "Anthropic rejected this API key. Nothing was saved.",
      steps: [
        "Check that the key is current and has not been revoked, or create a new key in the Claude Console.",
        "If your organization does not allow calls from a browser, the key cannot be used here.",
      ],
    },
    check_unavailable: {
      text: "The settings could not be checked. Nothing was saved.",
      steps: ["Save again."],
    },
    write_uncertain: {
      text: "The settings may not have been saved.",
      steps: ["Check whether the page shows an API key as set.", "Save again."],
    },
    deleted: {
      text: "The API key and the workspace ID are deleted.",
      steps: [],
    },
    delete_uncertain: {
      text: "The settings may not have been deleted.",
      steps: [
        "Check whether the page shows an API key as set.",
        "Delete again.",
      ],
    },
  },
  viewerTitle: {
    name: { release: "YomuDoga", dev: "YomuDoga (development build)" },
    invalidLink: "Invalid link",
    untitled: "Untitled video",
  },
  openOptions: {
    link: "Settings",
    setup: setupButton,
  },
  buildMark: "Development build",
  optionsPage: {
    title: "YomuDoga settings",
    heading: "Settings",
    apiKey: "Anthropic API key",
    apiKeyHint:
      "The key is stored in this browser only and is not shown again. A screen reader may read out the characters you type.",
    workspaceId: "Workspace ID (optional)",
    workspaceIdHint:
      "Leave it empty to use the workspace the API key belongs to.",
    save: "Save",
    remove: "Delete",
    keyState: {
      set: "An API key is set.",
      notSet: "No API key is set.",
      unknown: "Whether an API key is set could not be read.",
    },
    buildInfo: {
      heading: "Build",
      commit: "Commit",
      goVersion: "Go version",
      llm: "LLM",
      modified: "(modified)",
      none: "None",
      unreadable: "The build information could not be read.",
    },
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
      text: "Starting the article...",
      steps: [closePopupStep],
    },
    viewerNotice: { text: closePopupStep, steps: [] },
    articleReady: { text: "The article is ready.", steps: [] },
    choice: {
      text: "The extension has an article of the same video.",
      steps: [
        `"${openExistingButton}" shows it. "${generateFromSelectionButton}" generates a new article from the captions you selected.`,
      ],
    },
    existingStates: {
      loading: "Loading the article's state...",
      running: "The article is being generated.",
      done: "The article is ready.",
    },
    choiceEnded: {
      text: "The article of the same video can no longer be used.",
      steps: [`Press "${generateButton}" to generate the article.`],
    },
  },
};
