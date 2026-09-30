import type { MiniUser } from '../types'

export function validateLoginCode(code: string): string | null {
  return code.trim() ? null : '微信登录凭据不能为空'
}

export function upsertUser(existing: MiniUser | undefined, uid: string): MiniUser {
  if (existing && existing.uid === uid) return existing
  return { uid, assigned: existing?.assigned ?? false, nickname: existing?.nickname }
}
