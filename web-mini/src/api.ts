import type { AlarmSummary, DeviceModelLatest, HistoryQuery, HistoryResponse, LoginExchange, MiniSession, MiniUser, PondSummary, SubscriptionResult, WaterLatest } from './types'
import { validateHistoryQuery } from './domain/history'
import { validateLoginCode } from './domain/login'
import { z } from 'zod'
import { alarmsSchema, historySchema, loginSchema, modelSchema, pondsSchema, waterSchema } from './response-schemas'

export interface MiniApi {
  login(code: string): Promise<MiniSession>
  listPonds(): Promise<PondSummary[]>
  latest(deviceNo: string): Promise<WaterLatest | null>
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

export type RequestFn = (path: string, init?: RequestInit) => Promise<unknown>
export class MiniApiError extends Error {
  constructor(message: string, readonly status?: number) { super(message); this.name = 'MiniApiError' }
}

export function createFetchRequest(fetchFn: typeof fetch = fetch, basePath = '/api'): RequestFn {
  return async (path: string, init: RequestInit = {}): Promise<unknown> => {
    const headers = new Headers(init.headers)
    headers.set('Accept', 'application/json')
    if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
    const token = path === '/v1/auth/login' || typeof localStorage === 'undefined' ? null : localStorage.getItem('iolink.mini.token')
    if (token) headers.set('Authorization', `Bearer ${token}`)
    let response: Response
    try { response = await fetchFn(`${basePath}${path}`, { ...init, headers, signal: init.signal ?? AbortSignal.timeout(10_000) }) }
    catch { throw new MiniApiError('网络不可用，请重试') }
    const bodyText = await response.text()
    let body: unknown
    if (bodyText) {
      try { body = JSON.parse(bodyText) } catch { throw new MiniApiError('服务响应格式错误', response.status) }
    }
    if (!response.ok) {
      if (response.status === 401 && typeof localStorage !== 'undefined') localStorage.removeItem('iolink.mini.token')
      throw new MiniApiError({ 400: '请求参数无效', 401: '登录已过期，请重新登录', 403: '没有权限访问该资源', 404: '资源不存在或无权访问' }[response.status] ?? '请求失败', response.status)
    }
    return body
  }
}

export class ApiClient implements MiniApi {
  constructor(private readonly request: RequestFn) {}
  private async decoded<T>(path: string, schema: z.ZodType<T>, init?: RequestInit): Promise<T> {
    const result = schema.safeParse(await this.request(path, init))
    if (!result.success) throw new MiniApiError('服务响应结构错误')
    return result.data
  }
  async login(code: string): Promise<MiniSession> {
    const error = validateLoginCode(code)
    if (error) throw new MiniApiError(error)
    const result: LoginExchange = await this.decoded('/v1/auth/login', loginSchema, { method: 'POST', body: JSON.stringify({ code }) })
    return { token: result.token, expiresAt: Date.now() + result.expires_in * 1000,
      user: { uid: String(result.user.id), nickname: result.user.nickname, assigned: false } }
  }
  listPonds(): Promise<PondSummary[]> { return this.decoded('/v1/ponds', pondsSchema) }
  latest(deviceNo: string): Promise<WaterLatest | null> {
    if (!deviceNo.trim()) return Promise.reject(new MiniApiError('设备编号不能为空'))
    return this.decoded(`/v1/water/latest?device_no=${encodeURIComponent(deviceNo.trim())}`, waterSchema.nullable())
  }
  async history(query: HistoryQuery): Promise<HistoryResponse> {
    const error = validateHistoryQuery(query)
    if (error) throw new MiniApiError(error)
    return this.decoded(`/v1/water/history?device_no=${encodeURIComponent(query.deviceNo)}&metric=${encodeURIComponent(query.metric)}&range=${query.range}&max_points=${query.maxPoints}`, historySchema)
  }
  modelLatest(deviceNo: string): Promise<DeviceModelLatest> {
    if (!deviceNo.trim()) return Promise.reject(new MiniApiError('设备编号不能为空'))
    return this.decoded(`/v2/devices/${encodeURIComponent(deviceNo.trim())}/model/latest`, modelSchema)
  }
  alarms(): Promise<AlarmSummary[]> { return this.decoded('/v1/alarms', alarmsSchema) }
  async confirmAlarm(id: number): Promise<void> {
    if (!Number.isSafeInteger(id) || id <= 0) throw new MiniApiError('报警 ID 无效')
    await this.decoded(`/v1/alarms/${id}/confirm`, z.undefined(), { method: 'POST' })
  }
}

export function createMemoryUser(assigned = false): MiniUser { return { uid: 'demo-user', nickname: '微信用户', assigned } }
