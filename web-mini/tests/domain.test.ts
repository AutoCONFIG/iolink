import { describe, expect, it, vi } from 'vitest'
import { validateHistoryQuery, staleField } from '../src/domain/history'
import { nextPageForState, sessionState } from '../src/domain/session'
import { applySubscriptionDecision } from '../src/domain/subscription'
import { PAGES } from '../src/pages'
import { PAGE_CONTRACTS, canRenderPage } from '../src/page-contract'
import { upsertUser, validateLoginCode } from '../src/domain/login'
import { createPolling } from '../src/domain/polling'
import { createMiniApp } from '../src/app'
import { ApiClient, createFetchRequest, WechatCode2SessionAdapter, WechatSubscriptionAdapter } from '../src/api'
import { openAlarms } from '../src/domain/alarms'

describe('M4 page boundary', () => {
  it('declares exactly the six required pages', () => expect(PAGES.map((p) => p.id)).toEqual(['login', 'home', 'ponds', 'realtime', 'history', 'alarms']))
  it('declares loading, empty, offline and action contracts for every page', () => {
    expect(PAGE_CONTRACTS).toHaveLength(6)
    for (const contract of PAGE_CONTRACTS) expect(contract.loading && contract.empty && contract.offline && contract.actions.length).toBeTruthy()
    expect(canRenderPage('home', 'unassigned')).toBe(true)
    expect(canRenderPage('home', 'signed_out')).toBe(false)
  })
  it('handles signed out, expired, offline, and unassigned states', () => {
    expect(sessionState(null)).toBe('signed_out')
    expect(sessionState({ token: 't', expiresAt: 1, user: { uid: 'u', assigned: true } }, 2)).toBe('expired')
    expect(sessionState({ token: 't', expiresAt: 999, user: { uid: 'u', assigned: false } }, 2)).toBe('unassigned')
    expect(sessionState({ token: 't', expiresAt: 999, user: { uid: 'u', assigned: true } }, 2, false)).toBe('offline')
    expect(nextPageForState('expired')).toBe('login')
  })
  it('rejects empty login codes and creates users idempotently', () => {
    expect(validateLoginCode(' ')).toContain('不能为空')
    const first = upsertUser(undefined, 'uid-1')
    expect(upsertUser(first, 'uid-1')).toBe(first)
    expect(upsertUser(first, 'uid-1').assigned).toBe(false)
  })
  it('polls every 30 seconds, pauses by stop, and refreshes on foreground', () => {
    let calls = 0
    const polling = createPolling(() => { calls += 1 }, 30_000)
    expect(polling).toHaveProperty('start')
    vi.useFakeTimers(); polling.start(); vi.advanceTimersByTime(30_000); expect(calls).toBe(1); polling.stop(); vi.advanceTimersByTime(30_000); expect(calls).toBe(1); polling.onForeground(); expect(calls).toBe(2); vi.useRealTimers()
  })
  it('uses typed API paths and rejects invalid login before transport', async () => {
    const paths: string[] = []
    const api = new ApiClient(async <T>(path: string): Promise<T> => { paths.push(path); return { token: 't', expires_in: 60, user: { uid: 'u', assigned: false } } as T })
    await expect(api.login('')).rejects.toThrow('不能为空')
    await api.login('wx-code')
    await api.history({ deviceNo: 'dev-1', metric: 'ph', range: '7d', maxPoints: 20 })
    await api.confirmAlarm(7)
    expect(paths).toEqual(['/auth/login', '/water/history?device_no=dev-1&metric=ph&range=7d&max_points=20', '/alarms/7/confirm'])
  })
  it('types latest responses and rejects blank device numbers before transport', async () => {
    const api = new ApiClient(async <T>(): Promise<T> => ({ device_no: 'dev-1', pond_id: 2, ts: new Date(0).toISOString(), temperature: null, dissolved_oxygen: null, ph: 7, turbidity: null, salinity: null, signal: -60, timestamps: { temperature: null, dissolved_oxygen: null, ph: new Date(0).toISOString(), turbidity: null, salinity: null, signal: null }, report_interval: 60 } as T))
    await expect(api.latest('dev-1')).resolves.toMatchObject({ device_no: 'dev-1', ph: 7 })
    await expect(api.latest(' ')).rejects.toThrow('设备编号不能为空')
  })
  it('maps HTTP, malformed JSON, and empty 204 responses at the transport boundary', async () => {
    const response = (status: number, body: string) => new Response(status === 204 ? null : body, { status, headers: { 'content-type': 'application/json' } })
    await expect(createFetchRequest(async () => response(401, '{"message":"expired"}'))('/alarms')).rejects.toMatchObject({ status: 401, message: 'expired' })
    await expect(createFetchRequest(async () => response(200, '{'))('/alarms')).rejects.toMatchObject({ message: '服务响应格式错误' })
    await expect(createFetchRequest(async () => response(204, ''))('/alarms/1/confirm', { method: 'POST' })).resolves.toBeUndefined()
  })
  it('keeps confirmed alarms out of the pending count', () => {
    expect(openAlarms([{ id: 1, device_no: 'dev-1', pond_id: 1, metric: 'ph', current_value: 8, threshold: 7, level: 'warning', message: 'pH过高', confirmed_at: null, created_at: new Date(0).toISOString() }, { id: 2, device_no: 'dev-1', pond_id: 1, metric: 'ph', current_value: 6, threshold: 7, level: 'critical', message: 'pH过低', confirmed_at: new Date(0).toISOString(), created_at: new Date(0).toISOString() }])).toHaveLength(1)
  })
  it('keeps code2session exchange behind a narrow adapter', async () => {
    const adapter = new WechatCode2SessionAdapter(async (code) => ({ uid: `uid:${code}` }))
    await expect(adapter.exchange('')).rejects.toThrow('不能为空')
    await expect(adapter.exchange('code')).resolves.toEqual({ uid: 'uid:code' })
  })
  it('maps WeChat subscription agree, deny, and cancel results', async () => {
    await expect(new WechatSubscriptionAdapter(async () => ({ 'tmpl-1': 'accept' })).request(['tmpl-1'])).resolves.toEqual({ decision: 'agreed', accepted: true })
    await expect(new WechatSubscriptionAdapter(async () => ({ 'tmpl-1': 'reject' })).request(['tmpl-1'])).resolves.toEqual({ decision: 'denied', accepted: false })
    await expect(new WechatSubscriptionAdapter(async () => ({ 'tmpl-1': 'cancel' })).request(['tmpl-1'])).resolves.toEqual({ decision: 'cancelled', accepted: false })
  })
  it('wires the six page controller and foreground/background polling lifecycle', () => {
    const app = createMiniApp(() => undefined)
    expect(app.pages).toHaveLength(6)
    app.go('realtime'); expect(app.current).toBe('realtime')
    app.onShow(); app.onHide()
  })
  it('enforces history range, metric and max point bounds', () => {
    const valid = { deviceNo: 'dev-1', metric: 'ph', range: '7d', maxPoints: 200 } as const
    expect(validateHistoryQuery(valid)).toBeNull()
    expect(validateHistoryQuery({ ...valid, maxPoints: 201 })).toContain('200')
    expect(validateHistoryQuery({ ...valid, range: 'bad' as never })).toContain('历史范围')
  })
  it('marks readings older than three reporting intervals stale', () => {
    expect(staleField(new Date(0).toISOString(), 30, 91_000)).toBe(true)
    expect(staleField(new Date(80_000).toISOString(), 30, 91_000)).toBe(false)
    expect(staleField(null, 30)).toBe(true)
  })
  it('keeps alarm centre usable after every subscription decision', () => {
    for (const decision of ['agreed', 'denied', 'cancelled'] as const) expect(applySubscriptionDecision(decision)).toMatchObject({ decision, canViewAlarms: true, shouldPrompt: false })
    expect(applySubscriptionDecision('not_requested').shouldPrompt).toBe(true)
  })
})
