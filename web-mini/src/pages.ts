import type { Page } from './types'

export const PAGES: ReadonlyArray<{ id: Page; title: string }> = [
  { id: 'login', title: '微信登录' },
  { id: 'home', title: '首页' },
  { id: 'ponds', title: '池塘' },
  { id: 'realtime', title: '实时水质' },
  { id: 'history', title: '历史曲线' },
  { id: 'alarms', title: '报警中心' },
]
