// What the options page shows after a save or a delete. Each outcome has one
// fixed message (never an error's or the browser's text, never a value the
// user typed), and decides whether the two fields are cleared and which
// fields are marked as not accepted. The page reads again whether an API key
// is set after every save and delete, whatever the outcome, since a write
// that failed may still have been applied.

import { catalogs } from "./i18n/catalog.ts";
import type { Message } from "./i18n/catalog.ts";
import type { Locale } from "./i18n/locale.ts";
import type { SettingsField } from "./wasmResult.ts";

/** The outcome of a save or a delete on the options page. */
export type OptionsOutcome =
  | "saved"
  | "saved_unverified"
  | "field_rejected"
  | "key_rejected"
  | "check_unavailable"
  | "write_uncertain"
  | "deleted"
  | "delete_uncertain";

/** An outcome, with the fields the check did not accept for field_rejected. */
export type OptionsResult =
  | { readonly outcome: Exclude<OptionsOutcome, "field_rejected"> }
  | {
      readonly outcome: "field_rejected";
      readonly fields: readonly SettingsField[];
    };

/**
 * How an outcome's message looks: plain for a save or delete that was done,
 * warning for one that was done but is uncertain or unchecked, and failure
 * for one that saved nothing. The message's words tell the same, so the look
 * only repeats them.
 */
export type OptionsTone = "plain" | "warning" | "failure";

/** How the options page shows an outcome. */
export interface OptionsView {
  /** The message's look. */
  readonly tone: OptionsTone;
  /** The fixed message, announced in the page's status region. */
  readonly message: Message;
  /** Whether both fields are emptied: only after the settings were written. */
  readonly clearFields: boolean;
  /** The fields to mark as not accepted; empty for every other outcome. */
  readonly rejectedFields: readonly SettingsField[];
}

/** How the options page shows result in locale. */
export function optionsView(
  result: OptionsResult,
  locale: Locale,
): OptionsView {
  const message = catalogs[locale].options[result.outcome];
  switch (result.outcome) {
    case "saved":
      return { tone: "plain", message, clearFields: true, rejectedFields: [] };
    case "saved_unverified":
      return {
        tone: "warning",
        message,
        clearFields: true,
        rejectedFields: [],
      };
    case "field_rejected":
      return {
        tone: "failure",
        message,
        clearFields: false,
        rejectedFields: result.fields,
      };
    case "key_rejected":
    case "check_unavailable":
      return {
        tone: "failure",
        message,
        clearFields: false,
        rejectedFields: [],
      };
    case "write_uncertain":
    case "delete_uncertain":
      return {
        tone: "warning",
        message,
        clearFields: false,
        rejectedFields: [],
      };
    case "deleted":
      return { tone: "plain", message, clearFields: false, rejectedFields: [] };
  }
}
