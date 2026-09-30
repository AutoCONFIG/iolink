import { createPolling, type PollingController } from './domain/polling'
import { PAGES } from './pages'
import type { Page } from './types'

export interface MiniAppController { readonly pages: typeof PAGES; current: Page; polling: PollingController; go(page: Page): void; onHide(): void; onShow(): void }

export function createMiniApp(refreshRealtime: () => void): MiniAppController {
  return { pages: PAGES, current: 'login', polling: createPolling(refreshRealtime), go(page) { this.current = page }, onHide() { this.polling.stop() }, onShow() { this.polling.start(); this.polling.onForeground() } }
}
