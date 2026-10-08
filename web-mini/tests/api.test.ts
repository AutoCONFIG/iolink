import { createServer } from 'node:http'
import { request as httpRequest } from 'node:http'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { ApiClient, createFetchRequest } from '../src/api'

describe('API HTTP boundary', () => {
  let baseUrl = ''
  const observed: string[] = []
  const server = createServer(async (req, res) => {
    observed.push(req.url ?? '')
    const chunks: Buffer[] = []
    for await (const chunk of req) chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
    const body = Buffer.concat(chunks).toString()
    res.setHeader('Content-Type', 'application/json')
    if (req.url === '/api/v1/auth/login') {
      if (req.headers['content-type'] !== 'application/json' || JSON.parse(body).code !== 'valid-code') { res.writeHead(400); res.end('{}'); return }
      res.end(JSON.stringify({ token: 'fixture-token', expires_in: 60, user: { id: 17, nickname: '测试用户' } }))
    } else if (req.url === '/api/v1/ponds') res.end(JSON.stringify([{ pond_id: 1, pond_name: '测试塘', status: 'normal', device_count: 0, latest: null }]))
    else if (req.url === '/api/v1/alarms/7/confirm') { res.writeHead(204); res.end() }
    else if (req.url?.startsWith('/api/v1/water/latest')) res.end('null')
    else { res.writeHead(404); res.end('{"error":"not_found"}') }
  })
  beforeAll(async () => {
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
    const address = server.address()
    if (!address || typeof address === 'string') throw new Error('HTTP fixture failed to listen')
    baseUrl = `http://127.0.0.1:${address.port}/api`
  })
  afterAll(async () => { await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve())) })
  async function nodeFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
    const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url)
    return new Promise((resolve, reject) => {
      const request = httpRequest({ hostname: url.hostname, port: url.port, path: `${url.pathname}${url.search}`, method: init.method ?? 'GET', headers: Object.fromEntries(new Headers(init.headers).entries()) }, (response) => {
        const chunks: Buffer[] = []
        response.on('data', (chunk: Buffer) => chunks.push(chunk))
        response.on('end', () => resolve(new Response(Buffer.concat(chunks), { status: response.statusCode, headers: response.headers as Record<string, string> })))
      })
      request.on('error', reject)
      if (typeof init.body === 'string') request.write(init.body)
      request.end()
    })
  }
  it('exercises JSON login, target pond shape, null latest and 204 through real HTTP', async () => {
    const api = new ApiClient(createFetchRequest(nodeFetch, baseUrl))
    await expect(api.login('valid-code')).resolves.toMatchObject({ token: 'fixture-token', user: { uid: '17', nickname: '测试用户' } })
    await expect(api.listPonds()).resolves.toEqual([{ pond_id: 1, pond_name: '测试塘', status: 'normal', device_count: 0, latest: null }])
    await expect(api.latest('dev /1')).resolves.toBeNull()
    await expect(api.confirmAlarm(7)).resolves.toBeUndefined()
    await expect(api.modelLatest('dev /1')).rejects.toMatchObject({ status: 404 })
    expect(observed).toContain('/api/v2/devices/dev%20%2F1/model/latest')
  })
  it.each([401, 403, 404, 500])('propagates HTTP %i without displaying raw internal text', async (status) => {
    const api = new ApiClient(createFetchRequest(async () => new Response('{"error":"private_internal_detail"}', { status })))
    await expect(api.alarms()).rejects.toMatchObject({ status })
    await expect(api.alarms()).rejects.not.toThrow('private_internal_detail')
  })
  it.each(['login', 'ponds', 'latest', 'history', 'model', 'alarms', 'confirm'] as const)('rejects malformed successful %s payloads', async (operation) => {
    const api = new ApiClient(async () => ({ invalid: true }))
    const calls = {
      login: () => api.login('code'), ponds: () => api.listPonds(), latest: () => api.latest('dev'),
      history: () => api.history({ deviceNo: 'dev', metric: 'ph', range: 'today', maxPoints: 20 }),
      model: () => api.modelLatest('dev'), alarms: () => api.alarms(), confirm: () => api.confirmAlarm(1),
    }
    await expect(calls[operation]()).rejects.toThrow('服务响应结构错误')
  })
  it('handles empty, malformed JSON, network failure and invalid confirm IDs', async () => {
    await expect(new ApiClient(async () => []).alarms()).resolves.toEqual([])
    await expect(new ApiClient(createFetchRequest(async () => new Response('{'))).alarms()).rejects.toThrow('服务响应格式错误')
    await expect(new ApiClient(createFetchRequest(async () => { throw new TypeError('offline') })).alarms()).rejects.toThrow('网络不可用')
    const api = new ApiClient(async () => { throw new Error('should never send') })
    for (const id of [0, -1, 1.5, NaN]) await expect(api.confirmAlarm(id)).rejects.toThrow('报警 ID 无效')
  })
})
