// 0090 PoC (PoC branch only, never merged into main): one liveness query to
// a runner page over the PoC BroadcastChannel, used by the control page and
// the service worker. The result holds no secret: the round trip, the
// runner's instance ID, status, and visibility state.

export interface PingResult {
  pingId: string;
  sentAt: string;
  rttMs: number | null;
  instanceId: string | null;
  runnerStatus: unknown;
  runnerVisibility: unknown;
}

const timeoutMs = 3000;

/** Pings the runner of runId and resolves when it answers or after 3 s. */
export function pingRunner(runId: string): Promise<PingResult> {
  const channel = new BroadcastChannel("poc0090-runner");
  const pingId = crypto.randomUUID();
  const sentAt = new Date().toISOString();
  const start = performance.now();
  return new Promise((resolve) => {
    const done = (answer: Partial<PingResult>): void => {
      clearTimeout(timer);
      channel.close();
      resolve({
        pingId,
        sentAt,
        rttMs: null,
        instanceId: null,
        runnerStatus: null,
        runnerVisibility: null,
        ...answer,
      });
    };
    const timer = setTimeout(() => done({}), timeoutMs);
    channel.addEventListener("message", (event: MessageEvent<unknown>) => {
      const data = event.data as {
        type?: unknown;
        runId?: unknown;
        pingId?: unknown;
        instanceId?: unknown;
        status?: unknown;
        visibilityState?: unknown;
      };
      if (
        data.type === "pong" &&
        data.runId === runId &&
        data.pingId === pingId
      ) {
        done({
          rttMs: Math.round((performance.now() - start) * 10) / 10,
          instanceId:
            typeof data.instanceId === "string" ? data.instanceId : null,
          runnerStatus: data.status,
          runnerVisibility: data.visibilityState,
        });
      }
    });
    channel.postMessage({ type: "ping", runId, pingId });
  });
}
