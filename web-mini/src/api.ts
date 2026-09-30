import type { AlarmSummary, DeviceModelLatest, HistoryQuery, HistoryResponse, LoginExchange, MiniSession, MiniUser, PondSummary, SubscriptionResult } from './types'
import { validateHistoryQuery } from './domain/history'
import { validateLoginCode } from './domain/login'

export interface MiniApi {
  login(code: string): Promise<MiniSession>
  listPonds(): Promise<PondSummary[]>
  latest(deviceNo: string): Promise<unknown | null>
  history(query: HistoryQuery): Promise<HistoryResponse>
  modelLatest(deviceNo: string): Promise<DeviceModelLatest>
  alarms(): Promise<AlarmSummary[]>
  confirmAlarm(id: number): Promise<void>
}

export interface Code2SessionPort { exchange(code: string): Promise<{ uid: string }> }
export interface SubscriptionPort { request(templateIds: string[]): Promise<SubscriptionResult> }

export class WechatCode2SessionAdapter implements Code2SessionPort {
  constructor(private readonly exchangeCode: (code: string) => Promise<{ uid: string }>) {}
  exchange(code: string) { if (!code.trim()) return Promise.reject(new Error('微信登录凭据不能为空')); return this.exchangeCode(code) }
}

export class WechatSubscriptionAdapter implements SubscriptionPort {
  constructor(private readonly requestSubscribeMessage: (templateIds: string[]) => Promise<Record<string, string>>) {}
  async request(templateIds: string[]): Promise<SubscriptionResult> {
    const result = await this.requestSubscribeMessage(templateIds)
    const values = Object.values(result)
    const decision = values.includes('accept') ? 'agreed' : values.includes('reject') ? 'denied' : 'cancelled'
    return { decision, accepted: decision === 'agreed' }
  }
}

type RequestFn = <T>(path: string, init?: RequestInit) => Promise<T>
export class ApiClient implements MiniApi {
  constructor(private readonly request: RequestFn) {}
  async login(code: string): Promise<MiniSession> { const error = validateLoginCode(code); if (error) throw new Error(error); const result = await this.request<LoginExchange>('/auth/login', { method: 'POST', body: JSON.stringify({ code }) }); return { token: result.token, expiresAt: Date.now() + result.expires_in * 1000, user: result.user } }
  listPonds() { return this.request<PondSummary[]>('/ponds') }
  latest(deviceNo: string) { return this.request<unknown | null>(`/water/latest?device_no=${encodeURIComponent(deviceNo)}`) }
  async history(query: HistoryQuery) { const error = validateHistoryQuery(query); if (error) throw new Error(error); return this.request<HistoryResponse>(`/water/history?device_no=${encodeURIComponent(query.deviceNo)}&metric=${encodeURIComponent(query.metric)}&range=${query.range}&max_points=${query.maxPoints}`) }
  modelLatest(deviceNo: string) { if (!deviceNo.trim()) return Promise.reject(new Error('设备编号不能为空')); return this.request<DeviceModelLatest>(`/v2/devices/${encodeURIComponent(deviceNo)}/model/latest`) }
  alarms() { return this.request<AlarmSummary[]>('/alarms') }
  async confirmAlarm(id: number) { await this.request(`/alarms/${id}/confirm`, { method: 'POST' }) }
}

export function createMemoryUser(assigned = false): MiniUser { return { uid: 'demo-user', nickname: '微信用户', assigned } }
