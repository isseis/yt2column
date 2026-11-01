// Queries to the viewers on their BroadcastChannel, shared by the service
// worker and the popup. A query posts one message with a fresh query ID and
// collects, for a fixed window, the answers that pass the message check and
// carry that ID; anything else on the channel is ignored. Every query opens
// its own endpoint and closes it when the window ends, so a late answer
// reaches no one.

import type { OpenViewerChannel, ViewerChannel } from "./browser/chromeDeps.ts";
import type { ArticleKey } from "./core/articleKey.ts";
import { parseViewerChannelMessage } from "./core/extensionMessages.ts";
import type {
  PresentAnswer,
  ViewerChannelMessage,
  WaitingAnswer,
} from "./core/extensionMessages.ts";
import { viewerQueryWindowMs } from "./core/poll.ts";

export interface ViewerQueryDeps {
  readonly openChannel: OpenViewerChannel;
  /** A new query ID: 128 random bits as 32 lowercase hexadecimal characters. */
  readonly queryId: () => string;
  /** Runs callback once after ms milliseconds; returns a function that cancels it. */
  readonly schedule: (ms: number, callback: () => void) => () => void;
}

/**
 * Posts message on a new endpoint and passes each checked message that
 * arrives to onAnswer, until onAnswer returns true or viewerQueryWindowMs
 * passes; then closes the endpoint. Resolves whether the message was posted:
 * false when the channel could not be opened or could not post. Never
 * rejects.
 */
export function askViewers(
  message: ViewerChannelMessage,
  deps: Omit<ViewerQueryDeps, "queryId">,
  onAnswer: (answer: ViewerChannelMessage) => boolean,
): Promise<boolean> {
  const opened = deps.openChannel();
  if (!opened.ok) {
    return Promise.resolve(false);
  }
  const channel: ViewerChannel = opened.value;
  return new Promise((resolve) => {
    let done = false;
    const finish = (posted: boolean): void => {
      if (done) {
        return;
      }
      done = true;
      cancel();
      stop();
      channel.close();
      resolve(posted);
    };
    // Listening starts before the post, so no answer can come first.
    const stop = channel.listen((data) => {
      const answer = parseViewerChannelMessage(data);
      if (!done && answer !== undefined && onAnswer(answer)) {
        finish(true);
      }
    });
    const cancel = deps.schedule(viewerQueryWindowMs, () => finish(true));
    if (!channel.post(message)) {
      finish(false);
    }
  });
}

/** The viewers that answered one query for a key. */
export interface ViewerAnswers {
  /** The query's ID; a take-over for one of these viewers carries it. */
  readonly queryId: string;
  /** The viewers showing the key. */
  readonly present: readonly PresentAnswer[];
  /** The viewers still waiting for a start result. */
  readonly waiting: readonly WaitingAnswer[];
}

/**
 * Asks the viewers whether they show key and collects for
 * viewerQueryWindowMs the present and waiting answers to this query. A
 * channel that cannot be opened or posted on yields no answers. Never
 * rejects.
 */
export async function findViewers(
  key: ArticleKey,
  deps: ViewerQueryDeps,
): Promise<ViewerAnswers> {
  const queryId = deps.queryId();
  const present: PresentAnswer[] = [];
  const waiting: WaitingAnswer[] = [];
  await askViewers({ type: "find", queryId, key }, deps, (answer) => {
    if (answer.type === "present" && answer.queryId === queryId) {
      present.push(answer);
    } else if (answer.type === "waiting" && answer.queryId === queryId) {
      waiting.push(answer);
    }
    return false;
  });
  return { queryId, present, waiting };
}
