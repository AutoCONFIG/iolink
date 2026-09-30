export type Page = 'login' | 'home' | 'ponds' | 'realtime' | 'history' | 'alarms'
export type SessionState = 'signed_out' | 'ready' | 'expired' | 'offline' | 'unassigned'
export type SubscriptionDecision = 'agreed' | 'denied' | 'cancelled' | 'not_requested'
export type HistoryRange = 'today' | '7d' | '30d'

export interface MiniUser { uid: string; nickname?: string; assigned: boolean }
export interface MiniSession { token: string; expiresAt: number; user: MiniUser }
export interface HistoryQuery { deviceNo: string; metric: string; range: HistoryRange; maxPoints: number }
export interface LoginExchange { token: string; expires_in: number; user: MiniUser }
export interface SubscriptionResult { decision: SubscriptionDecision; accepted: boolean }
export interface PondSummary { id: number; name: string; farm_name?: string; latest?: Record<string, number | string | null> | null }
export interface AlarmSummary { id: number; level: 'warning' | 'critical'; message: string; confirmed_at?: string | null }
export interface HistoryResponse { metric: string; unit?: string; points: Array<{ ts: string; value: number | null }> }
export type ModelFieldType = 'number' | 'integer' | 'boolean' | 'string'
export interface DeviceModelField { identifier: string; type: ModelFieldType; unit: string; minimum?: number | null; maximum?: number | null; enum_values?: string[] | null; readable: boolean; writable: boolean; nullable: boolean }
export interface DeviceModelLatest { device_no: string; product_id: number; model_version: number; fields: DeviceModelField[]; ts: string | null; properties: Record<string, unknown> }
