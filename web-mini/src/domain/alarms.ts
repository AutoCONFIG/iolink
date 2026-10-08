import type { AlarmSummary, SubscriptionDecision } from '../types'

export function alarmLevelLabel(level: AlarmSummary['level']): string {
  return level === 'critical' ? '严重' : '预警'
}

export function subscriptionDecisionLabel(decision: SubscriptionDecision): string {
  return { agreed: '已同意订阅', denied: '已拒绝订阅', cancelled: '已取消订阅', not_requested: '尚未请求订阅' }[decision]
}

export function openAlarms(alarms: readonly AlarmSummary[]): AlarmSummary[] {
  return alarms.filter((alarm) => alarm.confirmed_at === null)
}
