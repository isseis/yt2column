// Queries to the viewers on their BroadcastChannel in the extension-only
// path: a viewer that was handed an existing job asks whether another viewer
// already shows the same job. A query posts one message with a fresh query ID
// and collects, for a fixed window, the answers that pass the message check
// and carry that ID; anything else on the channel is ignored. Every query
// opens its own endpoint and closes it when the window ends, so a late answer
// reaches no one.

import type { OpenViewerChannel, ViewerChannel } from "./browser/chromeDeps.ts";
import { parseJobChannelMessage } from "./core/jobMessages.ts";
import type { JobChannelMessage, PresentAnswer } from "./core/jobMessages.ts";
import { viewerQueryWindowMs } from "./core/poll.ts";

export interface JobViewerQueryDeps {
  readonly openChannel: OpenViewerChannel;
  /** A new query ID: 128 random bits as 32 lowercase hexadecimal characters. */
  readonly queryId: () => string;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  readonly schedule: (ms: number, callback: () => void) => () => void;
}

/**
 * Posts message on a new endpoint and passes each checked message that
 * arrives to onAnswer, until viewerQueryWindowMs passes; then closes the
 * endpoint. Resolves at the end of the window, at once when the channel
 * could not be opened or could not post. Never rejects.
 */
function ask(
  message: JobChannelMessage,
  deps: Omit<JobViewerQueryDeps, "queryId">,
  onAnswer: (answer: JobChannelMessage) => void,
): Promise<void> {
  const opened = deps.openChannel();
  if (!opened.ok) {
    return Promise.resolve();
  }
  const channel: ViewerChannel = opened.value;
  return new Promise<void>((resolve) => {
    let done = false;
    const finish = (): void => {
      if (done) {
        return;
      }
      done = true;
      cancel();
      stop();
      channel.close();
      resolve();
    };
    // Listening starts before the post, so no answer can come first.
    const stop = channel.listen((data) => {
      const answer = parseJobChannelMessage(data);
      if (!done && answer !== undefined) {
        onAnswer(answer);
      }
    });
    const cancel = deps.schedule(viewerQueryWindowMs, finish);
    if (!channel.post(message)) {
      finish();
    }
  });
}

/**
 * Asks the viewers whether they show jobId and collects, for
 * viewerQueryWindowMs, the present answers to this query. A channel that
 * cannot be opened or posted on yields no answers. Never rejects.
 */
export async function findJobViewers(
  jobId: string,
  deps: JobViewerQueryDeps,
): Promise<readonly PresentAnswer[]> {
  const queryId = deps.queryId();
  const present: PresentAnswer[] = [];
  await ask({ type: "find", queryId, jobId }, deps, (answer) => {
    if (answer.type === "present" && answer.queryId === queryId) {
      present.push(answer);
    }
  });
  return present;
}
