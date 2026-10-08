import { z } from 'zod'

const id = z.number().int().positive()
const timestamp = z.iso.datetime({ offset: true })
export const metricSchema = z.enum(['temperature', 'dissolved_oxygen', 'ph', 'turbidity', 'salinity'])
const numericReading = z.number().nullable()
const fieldTime = timestamp.nullable()
export const waterSchema = z.object({
  device_no: z.string().min(1), pond_id: id, ts: timestamp,
  temperature: numericReading, dissolved_oxygen: numericReading, ph: numericReading,
  turbidity: numericReading, salinity: numericReading, signal: z.number().int().nullable(),
  timestamps: z.object({ temperature: fieldTime, dissolved_oxygen: fieldTime, ph: fieldTime,
    turbidity: fieldTime, salinity: fieldTime, signal: fieldTime }),
  report_interval: z.union([z.literal(60), z.literal(300)]),
})
export const loginSchema = z.object({
  token: z.string().min(1), expires_in: z.number().int().positive(),
  user: z.object({ id, nickname: z.string() }),
})
export const pondsSchema = z.array(z.object({
  pond_id: id, pond_name: z.string(), status: z.enum(['normal', 'warning', 'critical']),
  device_count: z.number().int().nonnegative(), latest: waterSchema.nullable(),
}))
export const historySchema = z.object({
  metric: metricSchema, unit: z.enum(['℃', 'mg/L', '', 'NTU', 'ppt']),
  points: z.array(z.object({ ts: timestamp, value: z.number() })).max(200),
})
export const alarmsSchema = z.array(z.object({
  id, device_no: z.string(), pond_id: id, metric: metricSchema,
  current_value: z.number(), threshold: z.number(), level: z.enum(['warning', 'critical']),
  message: z.string(), confirmed_at: fieldTime, created_at: timestamp,
}))
export const modelSchema = z.object({
  device_no: z.string().min(1), product_id: id, model_version: id, ts: fieldTime,
  fields: z.array(z.object({ identifier: z.string().min(1), type: z.enum(['number', 'integer', 'boolean', 'string']),
    unit: z.string(), minimum: z.number().nullable().optional(), maximum: z.number().nullable().optional(),
    enum_values: z.array(z.string()).nullable().optional(), readable: z.boolean(), writable: z.boolean(), nullable: z.boolean() })),
  properties: z.record(z.string(), z.union([z.number(), z.boolean(), z.string(), z.null()])),
})
