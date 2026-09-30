import type { Page, SessionState } from './types'

export interface PageContract { page: Page; requiresAuth: boolean; loading: string; empty: string; offline: string; actions: readonly string[] }

export const PAGE_CONTRACTS: ReadonlyArray<PageContract> = [
  { page: 'login', requiresAuth: false, loading: '正在登录', empty: '请输入微信登录凭据', offline: '网络不可用，请重试', actions: ['wx.login'] },
  { page: 'home', requiresAuth: true, loading: '正在加载概览', empty: '暂无池塘，请联系管理员分配', offline: '离线，显示上次同步结果', actions: ['refresh'] },
  { page: 'ponds', requiresAuth: true, loading: '正在加载池塘', empty: '暂无可见池塘', offline: '网络不可用，无法刷新池塘', actions: ['open-pond'] },
  { page: 'realtime', requiresAuth: true, loading: '正在读取实时水质', empty: '暂无实时数据', offline: '离线，数据标记为过期', actions: ['poll-30s', 'pause-on-background'] },
  { page: 'history', requiresAuth: true, loading: '正在加载历史曲线', empty: '该范围暂无数据', offline: '网络不可用，无法加载历史', actions: ['range-today', 'range-7d', 'range-30d'] },
  { page: 'alarms', requiresAuth: true, loading: '正在加载报警', empty: '当前没有报警', offline: '离线仍可查看已缓存报警', actions: ['confirm', 'subscribe'] },
]

export function canRenderPage(page: Page, state: SessionState): boolean {
  const contract = PAGE_CONTRACTS.find((item) => item.page === page)
  return Boolean(contract && (!contract.requiresAuth || state === 'ready' || state === 'offline' || state === 'unassigned'))
}
