import type { HistoryQuery } from '../types'

export function validateHistoryQuery(query: HistoryQuery): string | null {
  if (!query.deviceNo.trim()) return '设备编号不能为空'
  if (!['temperature', 'dissolved_oxygen', 'ph', 'turbidity', 'salinity'].includes(query.metric)) return '不支持的监测指标'
  if (!['today', '7d', '30d'].includes(query.range)) return '不支持的历史范围'
  if (!Number.isInteger(query.maxPoints) || query.maxPoints < 1 || query.maxPoints > 200) return 'max_points 必须在 1 到 200 之间'
  return null
}

export function staleField(ts: string | null, reportIntervalSeconds: number, now = Date.now()): boolean {
  if (!ts) return true
  return now - Date.parse(ts) > reportIntervalSeconds * 3 * 1000
}
