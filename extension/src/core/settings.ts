// Settings judgement: the shared token and the server port. Every value that
// reaches a request is checked here; nothing is trimmed, lowercased or
// renumbered, so a stored value that does not have the accepted shape is
// refused rather than repaired.

declare const tokenBrand: unique symbol;
declare const portBrand: unique symbol;

/** A string that matched ^[0-9a-f]{64}$ exactly. Only parseToken creates it. */
export type SharedToken = string & { readonly [tokenBrand]: true };

/** A decimal string of 1024..65535 with no leading zero. Only parsePort creates it. */
export type ServerPort = string & { readonly [portBrand]: true };

/** The port used when none is stored; the same as the server's default. */
export const defaultPort: ServerPort = "8765" as ServerPort;

// ASCII only: a full-width digit or letter must not match.
const tokenPattern = /^[0-9a-f]{64}$/;
const portPattern = /^[1-9][0-9]*$/;
const minPort = 1024;
const maxPort = 65535;

/** Returns the value unchanged when it has the accepted form; otherwise undefined. */
export function parseToken(value: unknown): SharedToken | undefined {
  if (typeof value !== "string" || !tokenPattern.test(value)) {
    return undefined;
  }
  return value as SharedToken;
}

/** Returns the value unchanged when it has the accepted form; otherwise undefined. */
export function parsePort(value: unknown): ServerPort | undefined {
  if (typeof value !== "string" || !portPattern.test(value)) {
    return undefined;
  }
  const port = Number(value);
  if (port < minPort || port > maxPort) {
    return undefined;
  }
  return value as ServerPort;
}

/** The raw values read from storage; undefined means the key is absent. */
export interface StoredSettings {
  readonly token: unknown;
  readonly port: unknown;
}

export type SettingsBlockReason =
  "token-missing" | "token-invalid" | "port-invalid";

export type SettingsCheck =
  | {
      readonly kind: "ready";
      readonly token: SharedToken;
      readonly port: ServerPort;
    }
  | { readonly kind: "blocked"; readonly reason: SettingsBlockReason };

/** Checks the token, then the port, in the required order. Never repairs a value. */
export function checkSettings(stored: StoredSettings): SettingsCheck {
  if (stored.token === undefined) {
    return { kind: "blocked", reason: "token-missing" };
  }
  const token = parseToken(stored.token);
  if (token === undefined) {
    return { kind: "blocked", reason: "token-invalid" };
  }
  if (stored.port === undefined) {
    return { kind: "ready", token, port: defaultPort };
  }
  const port = parsePort(stored.port);
  if (port === undefined) {
    return { kind: "blocked", reason: "port-invalid" };
  }
  return { kind: "ready", token, port };
}

/** Reads the stored settings. Rejects when the storage cannot be read. */
export interface SettingsReader {
  load(): Promise<StoredSettings>;
}

/** The options page's view of the storage: reading plus the three writes. */
export interface SettingsStore extends SettingsReader {
  saveToken(token: SharedToken): Promise<void>;
  clearToken(): Promise<void>;
  savePort(port: ServerPort): Promise<void>;
}
