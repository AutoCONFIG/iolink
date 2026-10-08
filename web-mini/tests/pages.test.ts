import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from '../App.vue'
import AlarmsPage from '../src/pages/AlarmsPage.vue'
import ApiDemo from '../src/ApiDemo.vue'
import { ApiClient, createFetchRequest, MiniApiError } from '../src/api'
import type { AlarmSummary } from '../src/types'

const alarm: AlarmSummary = { id: 7, device_no: 'dev-1', pond_id: 1, metric: 'ph', current_value: 9,
  threshold: 8, level: 'critical', message: 'pH 偏高', confirmed_at: null, created_at: '2026-10-08T00:00:00Z' }
const wrappers: VueWrapper[] = []
afterEach(() => { wrappers.forEach((wrapper) => wrapper.unmount()); wrappers.length = 0; localStorage.clear(); vi.useRealTimers(); vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('alarm page interactions', () => {
  it('shows loading, confirms an alarm only after a successful response', async () => {
    let resolveLoad: (value: unknown) => void = () => undefined
    const request = vi.fn(async (path: string) => path === '/v1/alarms' ? new Promise((resolve) => { resolveLoad = resolve }) : undefined)
    const wrapper = mount(AlarmsPage, { props: { api: new ApiClient(request) } }); wrappers.push(wrapper)
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('正在加载报警')
    resolveLoad([alarm]); await flushPromises()
    expect(wrapper.text()).toContain('pH 偏高')
    await wrapper.findAll('button').find((button) => button.text() === '确认报警')?.trigger('click')
    await flushPromises()
    expect(request).toHaveBeenCalledWith('/v1/alarms/7/confirm', { method: 'POST' })
    expect(wrapper.text()).toContain('已确认')
    expect(wrapper.text()).toContain('未确认 0 条 / 共 1 条')
  })
  it('keeps errors exclusive from empty and refreshes after a failure', async () => {
    const request = vi.fn().mockRejectedValueOnce(new MiniApiError('没有权限', 403)).mockResolvedValueOnce([])
    const wrapper = mount(AlarmsPage, { props: { api: new ApiClient(request) } }); wrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('没有权限')
    expect(wrapper.text()).not.toContain('当前没有报警')
    await wrapper.findAll('button').find((button) => button.text() === '刷新')?.trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('当前没有报警')
  })
  it('preserves an unconfirmed alarm when confirmation fails', async () => {
    const api = new ApiClient(async (path) => {
      if (path === '/v1/alarms') return [alarm]
      throw new MiniApiError('资源不存在或无权访问', 404)
    })
    const wrapper = mount(AlarmsPage, { props: { api } }); wrappers.push(wrapper); await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === '确认报警')?.trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('资源不存在或无权访问')
    expect(wrapper.text()).toContain('未确认 1 条 / 共 1 条')
  })
  it.each([['agreed', '模拟同意', '已同意订阅'], ['denied', '模拟拒绝', '已拒绝订阅'], ['cancelled', '模拟取消', '已取消订阅']] as const)('renders %s without blocking the alarm page', async (_decision, label, expected) => {
    const wrapper = mount(AlarmsPage, { props: { api: new ApiClient(async () => []) } }); wrappers.push(wrapper); await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === label)?.trigger('click')
    expect(wrapper.text()).toContain(expected)
    expect(wrapper.text()).toContain('当前为浏览器演示')
    expect(wrapper.get('h1').text()).toBe('报警中心')
  })
  it('requires a configured template and handles platform failure', async () => {
    const request = vi.fn(async () => { throw new Error('订阅失败') })
    const wrapper = mount(AlarmsPage, { props: { api: new ApiClient(async () => []), subscription: { request } } }); wrappers.push(wrapper); await flushPromises()
    const button = wrapper.findAll('button').find((item) => item.text() === '请求微信订阅')
    await button?.trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('尚未配置微信订阅模板'); expect(request).not.toHaveBeenCalled()
    await wrapper.setProps({ templateId: 'test-template' }); await button?.trigger('click'); await flushPromises()
    expect(request).toHaveBeenCalledWith(['test-template']); expect(wrapper.text()).toContain('订阅失败')
  })
})

describe('interface demonstration', () => {
  it('logs in through the actual transport and hides the returned token', async () => {
    const transport = vi.fn(async () => new Response(JSON.stringify({ token: 'private-test-token', expires_in: 60, user: { id: 1, nickname: '测试用户' } })))
    const wrapper = mount(ApiDemo, { props: { page: 'login', api: new ApiClient(createFetchRequest(transport)) } }); wrappers.push(wrapper)
    await wrapper.get('input').setValue('valid-code'); await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(transport).toHaveBeenCalledWith('/api/v1/auth/login', expect.objectContaining({ method: 'POST', body: '{"code":"valid-code"}' }))
    expect(wrapper.get('pre').text()).toContain('测试用户'); expect(wrapper.get('pre').text()).not.toContain('private-test-token')
    expect(localStorage.getItem('iolink.mini.token')).toBe('private-test-token')
  })
  it('calls the history API with the selected range and shows units and points', async () => {
    const request = vi.fn(async () => ({ metric: 'ph', unit: '', points: [{ ts: '2026-10-08T00:00:00Z', value: 7.5 }] }))
    const wrapper = mount(ApiDemo, { props: { page: 'history', api: new ApiClient(request) } }); wrappers.push(wrapper)
    await wrapper.findAll('input')[1]?.setValue('dev-1'); await wrapper.findAll('select')[1]?.setValue('7d')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(request).toHaveBeenCalledWith('/v1/water/history?device_no=dev-1&metric=ph&range=7d&max_points=200', undefined)
    expect(wrapper.get('pre').text()).toContain('7.5'); expect(wrapper.get('pre').text()).toContain('unit')
  })
  it('polls realtime at 30 seconds, pauses when hidden and stops when unmounted', async () => {
    vi.useFakeTimers()
    const request = vi.fn(async () => null)
    const wrapper = mount(ApiDemo, { props: { page: 'realtime', api: new ApiClient(request) } }); wrappers.push(wrapper)
    await wrapper.findAll('input')[1]?.setValue('dev-1'); await wrapper.get('input[type="checkbox"]').setValue(true)
    await vi.advanceTimersByTimeAsync(30_000); expect(request).toHaveBeenCalledTimes(1)
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true); document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(60_000); expect(request).toHaveBeenCalledTimes(1)
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false); document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises(); expect(request).toHaveBeenCalledTimes(2)
    wrapper.unmount(); wrappers.length = 0; await vi.advanceTimersByTimeAsync(60_000); expect(request).toHaveBeenCalledTimes(2)
  })
  it('renders the six actual pages and exercises pond and alarm transport', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('[]')))
    const wrapper = mount(App); wrappers.push(wrapper)
    for (const [id, label] of [['home', '首页'], ['ponds', '池塘'], ['realtime', '实时水质'], ['history', '历史曲线'], ['alarms', '报警中心'], ['login', '微信登录']]) {
      const navButton = wrapper.findAll('nav button').find((button) => button.text() === label)
      await navButton?.trigger('click'); await flushPromises()
      expect(wrapper.find(`[data-page="${id}"]`).exists()).toBe(true)
    }
  })
})
