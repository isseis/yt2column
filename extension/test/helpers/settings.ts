import type {
  SettingsReader,
  StoredSettings,
} from "../../src/core/settings.ts";

/** A 64-character lowercase hex token, distinctive enough to search for in output. */
export const identifiableToken = "cafebabe".repeat(8);

/** A second accepted token, distinct from identifiableToken. */
export const otherToken = "deadbeef".repeat(8);

export interface SettingsReaderRecorder {
  readonly reader: SettingsReader;
  loadCalls(): number;
}

/** A SettingsReader whose load is respond and whose call count is recorded. */
export function recordingSettings(
  respond: () => Promise<StoredSettings>,
): SettingsReaderRecorder {
  let calls = 0;
  return {
    reader: {
      async load() {
        calls += 1;
        return respond();
      },
    },
    loadCalls: () => calls,
  };
}

/** A SettingsReader that resolves to stored on every load. */
export function fixedSettings(stored: StoredSettings): SettingsReaderRecorder {
  return recordingSettings(async () => stored);
}

/** A SettingsReader whose load rejects with cause. */
export function failingSettings(cause: unknown): SettingsReaderRecorder {
  return recordingSettings(async () => {
    throw cause;
  });
}
