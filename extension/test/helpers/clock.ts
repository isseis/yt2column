// A fake clock for the timing rules: the interval, the query window and the
// deadlines. Time moves only when the test calls advance, which runs each due
// callback at its own due time, in due order.

export interface FakeClock {
  now(): number;
  /** Runs callback once ms after now unless cancelled; returns the cancel function. */
  schedule(ms: number, callback: () => void): () => void;
  /**
   * Moves the time forward by ms, running every callback that falls due on
   * the way at its due time: the earliest first, and in scheduling order for
   * the same time. A callback scheduled on the way runs too when it falls due
   * within ms.
   */
  advance(ms: number): void;
  /** How many callbacks are scheduled and neither run nor cancelled. */
  pending(): number;
}

interface Timer {
  readonly due: number;
  readonly order: number;
  readonly callback: () => void;
}

export function fakeClock(start = 0): FakeClock {
  let time = start;
  let order = 0;
  const timers = new Set<Timer>();
  /** The earliest timer due no later than limit, or undefined. */
  const nextDue = (limit: number): Timer | undefined => {
    let next: Timer | undefined;
    for (const timer of timers) {
      if (
        timer.due <= limit &&
        (next === undefined ||
          timer.due < next.due ||
          (timer.due === next.due && timer.order < next.order))
      ) {
        next = timer;
      }
    }
    return next;
  };
  return {
    now: () => time,
    schedule: (ms, callback) => {
      const timer = { due: time + ms, order: order++, callback };
      timers.add(timer);
      return () => {
        timers.delete(timer);
      };
    },
    advance: (ms) => {
      if (ms < 0) {
        throw new RangeError("the clock only moves forward");
      }
      const target = time + ms;
      for (let timer = nextDue(target); timer; timer = nextDue(target)) {
        timers.delete(timer);
        time = timer.due;
        timer.callback();
      }
      time = target;
    },
    pending: () => timers.size,
  };
}

/**
 * Lets queued tasks and the promise chains they start run: the fakes deliver
 * messages as tasks, as the browser does. Waits a fixed number of turns, so
 * code that keeps queueing tasks forever would still return.
 */
export async function settle(): Promise<void> {
  for (let turn = 0; turn < 20; turn += 1) {
    await new Promise((resolve) => setImmediate(resolve));
  }
}
