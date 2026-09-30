export interface PollingController { start(): void; stop(): void; onForeground(): void }

export function createPolling(refresh: () => void, intervalMs = 30_000): PollingController {
  let timer: ReturnType<typeof setInterval> | undefined
  return {
    start() { if (!timer) timer = setInterval(refresh, intervalMs) },
    stop() { if (timer) clearInterval(timer); timer = undefined },
    onForeground() { refresh() },
  }
}
