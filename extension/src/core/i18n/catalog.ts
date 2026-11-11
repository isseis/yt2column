import type { PreflightReason } from "../prepare.ts";
import type { NotSentCause } from "../exchange.ts";
import type {
  FailureCode,
  ServerRejectReason,
  StepName,
  StepState,
} from "../response.ts";
import type { ExistingState } from "../popupView.ts";
import type { CheckName, CheckState, RejectionReason } from "../types.ts";
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

  // The job API's sections. A generation start whose result is not known is
  // resolved by sending the same input again: the server returns the article
  // of that input while it is generating or done instead of starting it again.

  /** Server rejection reasons of the two operations; one message per reason, whichever operation returned it. */
  readonly jobReject: Readonly<Record<ServerRejectReason, Message>>;
  /** The exchange results of a generation start that are not a server response. */
  readonly startExchange: ExchangeMessages;
  /** The exchange results of a status query that are not a server response. */
  readonly statusExchange: ExchangeMessages;
  /** A generation start accepted, and a wait that ended without its result. */
  readonly startResult: {
    readonly accepted: Message;
    readonly noResult: Message;
  };
  /** Failure codes; every code has a distinct message. */
  readonly failure: Readonly<Record<FailureCode, Message>>;
  /** The generation steps' names. */
  readonly stepNames: Readonly<Record<StepName, string>>;
  /** The word that tells a step's state; given to assistive technology. */
  readonly stepStates: Readonly<Record<StepState, string>>;
  /** The viewer page's own texts. */
  readonly viewer: {
    readonly sending: Message;
    readonly loading: Message;
    readonly invalidLink: Message;
    readonly running: string;
    readonly lost: Message;
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
     * The server has an article of the same video. It never says that no
     * other one exists or that this one will stay.
     */
    readonly choice: Message;
    /** The existing article's state while the choice is shown. */
    readonly existingStates: Readonly<Record<ExistingState, string>>;
    /** The existing article can no longer be used; the button generates again. */
    readonly choiceEnded: Message;
  };
}

/** The messages of the exchange results that are not a server response, for one operation. */
export interface ExchangeMessages {
  readonly notSent: Readonly<Record<NotSentCause, Message>>;
  readonly noResponse: Message;
  readonly hostFailed: Message;
  /** The one message for every unexpected reply; the check is for the developer log only. */
  readonly unexpected: Message;
}

/** The catalog of every locale. */
export const catalogs: Readonly<Record<Locale, Catalog>> = { en, ja };
