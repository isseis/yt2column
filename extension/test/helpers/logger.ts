import type { Logger } from "../../src/launch.ts";

export interface Recorded {
  readonly label: string;
  readonly data: unknown;
}

export interface RecordingLogger {
  readonly log: Logger;
  readonly infos: Recorded[];
  readonly errors: Recorded[];
}

/** A Logger that records every call instead of printing. */
export function recordingLogger(): RecordingLogger {
  const infos: Recorded[] = [];
  const errors: Recorded[] = [];
  return {
    log: {
      info: (label, data) => infos.push({ label, data }),
      error: (label, data) => errors.push({ label, data }),
    },
    infos,
    errors,
  };
}
