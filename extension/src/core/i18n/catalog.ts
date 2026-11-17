import type { BuildKind } from "../../buildKind.d.ts";
import type { PreflightReason } from "../prepare.ts";
import type { StartRejection } from "../jobMessages.ts";
import type { ArticleFailure, SetupFailure } from "../jobView.ts";
import type { OptionsOutcome } from "../optionsView.ts";
import type { ExistingState } from "../popupView.ts";
import type { CheckName, CheckState, RejectionReason } from "../types.ts";
import type { StepName, StepState } from "../wasmResult.ts";
import { en } from "./en.ts";
import { ja } from "./ja.ts";
import type { Locale } from "./locale.ts";

/** The shape of a message: a sentence and optional numbered steps. */
export interface Message {
  /** One-sentence statement of the situation. */
  readonly text: string;
  /** Steps that let the user succeed next time; empty when none apply. */
  readonly steps: readonly string[];
}

/**
 * Every string the popup and the viewer display, for one locale. Each section keyed by a
 * reason or field type is a Record over that type, so a catalog missing a
 * message -- in any locale, or after a value is added to the type -- fails
 * the type check.
 */
export interface Catalog {
  /** Collection rejection reasons. */
  readonly rejection: Readonly<Record<RejectionReason, Message>>;
  /** Blocked sends: the body size check. */
  readonly preflight: Readonly<Record<PreflightReason, Message>>;
  /**
   * The prompt shown when the video description is collapsed. A fixed text:
   * it never holds page text such as the description itself.
   */
  readonly expandPrompt: Message;
  /** The check rows' names. */
  readonly checkNames: Readonly<Record<CheckName, string>>;
  /** The word that tells a check row's state; given to assistive technology only. */
  readonly checkStates: Readonly<Record<CheckState, string>>;
  /** The selection row's counts; formats the numbers in its own locale. */
  readonly selectionSize: (characters: number, lines: number) => string;
  /** The description row when the full description is sent. */
  readonly descriptionFull: (characters: number) => string;
  /** The description row when no description is sent; no steps. */
  readonly descriptionNotSent: Message;
  /** The details' toggle and its value names. */
  readonly details: {
    readonly summary: string;
    readonly url: string;
    readonly tabTitle: string;
  };

  // The states of a generation start and of a job. A failure that ends
  // before the LLM API is called never mentions a cost.

  /** A start the service worker or the viewer refused; api_key_missing is setup's. */
  readonly startRejected: Readonly<
    Record<Exclude<StartRejection, "api_key_missing">, Message>
  >;
  /**
   * The reservation's result never arrived. It says that pressing the button
   * again does not generate twice only while the same input's article is
   * generating or done, and promises nothing for any other case.
   */
  readonly startUnknown: Message;
  /** The viewer could not take the input: nothing was charged, and the button is pressed again. */
  readonly inputUnavailable: Message;
  /** A setting is needed: no API key is set, or the stored settings cannot be used. */
  readonly setup: Readonly<Record<SetupFailure, Message>>;
  /** Anthropic rejected the API key while generating, and how to generate again. */
  readonly keyRejected: Message;
  /** Failure codes of a failed article; every code has a distinct message. */
  readonly failure: Readonly<Record<ArticleFailure, Message>>;
  /** The run stopped with its page. */
  readonly stopped: Message;
  /** The generation steps' names. */
  readonly stepNames: Readonly<Record<StepName, string>>;
  /** The word that tells a step's state; given to assistive technology. */
  readonly stepStates: Readonly<Record<StepState, string>>;
  /** The viewer page's own texts. */
  readonly viewer: {
    /** A start not reserved yet: the input is taken and checked. */
    readonly preparing: Message;
    /** A job ID whose record is being read. */
    readonly loading: Message;
    readonly invalidLink: Message;
    readonly running: string;
    /** No record holds the job: the browser closed, the extension was reloaded or updated, or it was dropped. */
    readonly lost: Message;
    /** Done, but only this page holds the article: reloading or closing it loses the article. */
    readonly unstored: Message;
    /**
     * An existing article of the same video: not generated from the current
     * selection, and the steps that generate from it.
     */
    readonly existing: Message;
    readonly videoLink: string;
    readonly elapsed: string;
    readonly duration: string;
    readonly cost: string;
    readonly costUnknown: string;
    readonly save: string;
    readonly saved: string;
    readonly copy: string;
    readonly copied: string;
    /** Copied, with an invisible character after an '@' to keep a mention from starting. */
    readonly copiedNeutralized: string;
    readonly copyFailed: string;
  };
  /** The settings page's outcome of a save or a delete; never the user's values. */
  readonly options: Readonly<Record<OptionsOutcome, Message>>;
  /** The viewer page's title: the name it ends with, and the parts that stand for a state. */
  readonly viewerTitle: {
    /** The extension's name for each build kind; the development build's carries its mark. */
    readonly name: Readonly<Record<BuildKind, string>>;
    /** The title's part when the viewer's link is not one it accepts. */
    readonly invalidLink: string;
    /** The title's part when no video title or heading is known. */
    readonly untitled: string;
  };
  /** The operations that open the options page: the popup's permanent one, and the one beside a missing or rejected setting. */
  readonly openOptions: {
    readonly link: string;
    readonly setup: string;
  };
  /** The visible mark of a development build. */
  readonly buildMark: string;
  /** The options page's own texts. */
  readonly optionsPage: {
    readonly title: string;
    readonly heading: string;
    readonly apiKey: string;
    /** The API key field's description; it warns that a screen reader may read the typed characters. */
    readonly apiKeyHint: string;
    readonly workspaceId: string;
    readonly workspaceIdHint: string;
    readonly save: string;
    readonly remove: string;
    /** Whether an API key is set; unknown when the item names could not be read. */
    readonly keyState: {
      readonly set: string;
      readonly notSet: string;
      readonly unknown: string;
    };
    /** The build's information, a hint for an investigation. */
    readonly buildInfo: {
      readonly heading: string;
      readonly commit: string;
      readonly goVersion: string;
      readonly llm: string;
      readonly modified: string;
      readonly none: string;
      readonly unreadable: string;
    };
  };
  /** The popup's job states. */
  readonly popup: {
    /** The primary button's names: before the send, and in the job states. */
    readonly primary: {
      readonly generate: string;
      readonly openViewer: string;
      readonly retry: string;
      /** The choice's primary button: shows the existing article. */
      readonly openExisting: string;
    };
    /** The choice's secondary button: generates from the current selection. */
    readonly secondary: string;
    readonly sending: Message;
    /** The popup may be closed: the result appears in the viewer tab. */
    readonly viewerNotice: Message;
    readonly articleReady: Message;
    /**
     * The extension has an article of the same video. It never says that no
     * other one exists or that this one will stay.
     */
    readonly choice: Message;
    /** The existing article's state while the choice is shown. */
    readonly existingStates: Readonly<Record<ExistingState, string>>;
    /** The existing article can no longer be used; the button generates again. */
    readonly choiceEnded: Message;
  };
}

/** The catalog of every locale. */
export const catalogs: Readonly<Record<Locale, Catalog>> = { en, ja };
