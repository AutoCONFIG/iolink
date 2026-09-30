import type { SubscriptionDecision } from '../types'

export function applySubscriptionDecision(decision: SubscriptionDecision): { decision: SubscriptionDecision; canViewAlarms: boolean; shouldPrompt: boolean } {
  return { decision, canViewAlarms: true, shouldPrompt: decision === 'not_requested' }
}
