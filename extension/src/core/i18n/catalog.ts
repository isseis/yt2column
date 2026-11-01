import type { PreflightReason } from "../prepare.ts";
import type { ArticlesRejectReason } from "../articlesResponse.ts";
import type { NotSentCause } from "../exchange.ts";
import type {
  FailureCode,
  PublishReason,
  PublishState,
  ServerRejectReason,
  StepName,
  StepState,
} from "../response.ts";
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
  /** Server rejection reasons. */
  readonly serverReject: Readonly<Record<ArticlesRejectReason, Message>>;
  /** Causes of a request known not to have reached the server; no Slack check. */
  readonly notSent: Readonly<Record<NotSentCause, Message>>;
  // The other send outcomes. Every result that is not known (noResponse,
  // hostFailed, unexpected) asks to check Slack before sending again.
  readonly accepted: Message;
  readonly noResponse: Message;
  readonly hostFailed: Message;
  /** The one message for every unexpected reply; the check is for the developer log only. */
  readonly unexpected: Message;
  /** The notice shown while a send is in flight. */
  readonly sending: Message;
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
  /** The primary button's name. */
  readonly primaryButton: string;

  // The job API's sections. No message of a generation start's result, nor
  // the sending notices, mentions Slack: the server returns the article of
  // the same input while it is generating or done, so a result that is not
  // known is resolved by sending again, not by checking Slack.

  /** Server rejection reasons of the three operations; one message per reason, whichever operation returned it. */
  readonly jobReject: Readonly<Record<ServerRejectReason, Message>>;
  /** The exchange results of a generation start that are not a server response. */
  readonly startExchange: ExchangeMessages;
  /** The exchange results of a status query that are not a server response. */
  readonly statusExchange: ExchangeMessages;
  /** The exchange results of a publish operation that are not a server response. */
  readonly publishExchange: ExchangeMessages;
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
  /** Publish states, shown beside the publish button. */
  readonly publishStates: Readonly<Record<PublishState, Message>>;
  /** Publish reasons of a failed or unpostable article. */
  readonly publishReasons: Readonly<Record<PublishReason, Message>>;
  /** The viewer page's own texts. */
  readonly viewer: {
    readonly sending: Message;
    readonly loading: Message;
    readonly invalidLink: Message;
    readonly running: string;
    readonly lost: Message;
    readonly webhookNotConfigured: Message;
    readonly videoLink: string;
    readonly elapsed: string;
    readonly duration: string;
    readonly cost: string;
    readonly costUnknown: string;
    readonly publish: string;
    readonly save: string;
    readonly saved: string;
    readonly copy: string;
    readonly copied: string;
    readonly copyFailed: string;
  };
  /** The popup's job states. */
  readonly popup: {
    /** The primary button's names: before the send, and in the job states. */
    readonly primary: {
      readonly generate: string;
      readonly openViewer: string;
      readonly retry: string;
    };
    readonly sending: Message;
    /** The popup may be closed: the result appears in the viewer tab. */
    readonly viewerNotice: Message;
    readonly articleReady: Message;
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
