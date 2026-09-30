import type { MiniSession, SessionState } from '../types'

export function sessionState(session: MiniSession | null, now = Date.now(), online = true): SessionState {
  if (!online) return 'offline'
  if (!session) return 'signed_out'
  if (session.expiresAt <= now) return 'expired'
  if (!session.user.assigned) return 'unassigned'
  return 'ready'
}

export function nextPageForState(state: SessionState): 'login' | 'home' {
  return state === 'signed_out' || state === 'expired' ? 'login' : 'home'
}
