<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { ApiClient, createFetchRequest, MiniApiError, type MiniApi } from './api'
import { createPolling } from './domain/polling'
import type { HistoryRange, Page, WaterMetric } from './types'

const props = defineProps<{ page: Exclude<Page, 'alarms'>; api?: MiniApi }>()
const api = props.api ?? new ApiClient(createFetchRequest())
const code = ref('')
const token = ref(localStorage.getItem('iolink.mini.token') ?? '')
const device = ref('')
const range = ref<HistoryRange>('today')
const metric = ref<WaterMetric>('ph')
const busy = ref(false)
const result = ref('')
const error = ref('')
const updated = ref('')
const autoRefresh = ref(false)
const poll = createPolling(() => { if (autoRefresh.value) void run() })

function saveToken() {
  if (token.value.trim()) localStorage.setItem('iolink.mini.token', token.value.trim())
  else localStorage.removeItem('iolink.mini.token')
}

async function run() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  result.value = ''
  try {
    let response: unknown
    if (props.page === 'login') {
      const session = await api.login(code.value)
      token.value = session.token
      saveToken()
      response = { user: session.user, expiresAt: session.expiresAt, token: '已保存（隐藏）' }
    } else {
      saveToken()
      if (props.page === 'home' || props.page === 'ponds') response = await api.listPonds()
      else if (props.page === 'realtime') response = await api.latest(device.value)
      else response = await api.history({ deviceNo: device.value, metric: metric.value, range: range.value, maxPoints: 200 })
    }
    result.value = JSON.stringify(response, null, 2)
    updated.value = new Date().toLocaleTimeString()
  } catch (cause) {
    error.value = cause instanceof MiniApiError && cause.status ? `HTTP ${cause.status}：${cause.message}` : cause instanceof Error ? cause.message : '请求失败'
  } finally { busy.value = false }
}

function visibilityChanged() {
  if (document.hidden) poll.stop()
  else { poll.start(); poll.onForeground() }
}
onMounted(() => {
  if (props.page === 'realtime') { poll.start(); document.addEventListener('visibilitychange', visibilityChanged) }
})
onBeforeUnmount(() => { poll.stop(); document.removeEventListener('visibilitychange', visibilityChanged) })
</script>

<template>
  <section class="api-demo" aria-label="接口演示">
    <form @submit.prevent="run">
      <label v-if="page === 'login'">微信登录 code<input v-model="code" type="password" autocomplete="off" /></label>
      <label v-else>测试用户 Bearer Token<input v-model="token" type="password" autocomplete="off" @change="saveToken" /></label>
      <template v-if="page === 'realtime' || page === 'history'">
        <label>设备编号<input v-model="device" /></label>
      </template>
      <template v-if="page === 'history'">
        <label>指标<select v-model="metric"><option value="temperature">温度</option><option value="dissolved_oxygen">溶氧</option><option value="ph">pH</option><option value="turbidity">浊度</option><option value="salinity">盐度</option></select></label>
        <label>范围<select v-model="range"><option value="today">今日</option><option value="7d">7 天</option><option value="30d">30 天</option></select></label>
      </template>
      <label v-if="page === 'realtime'"><input v-model="autoRefresh" type="checkbox" />每 30 秒刷新（后台暂停）</label>
      <button :disabled="busy" type="submit">{{ busy ? '请求中…' : '调用接口' }}</button>
    </form>
    <p v-if="page === 'login'">需使用有效微信 code；也可在其他页面填入测试用户 Token 验证资源接口。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="result" role="status">请求成功 · {{ updated }}</p>
    <pre v-if="result" aria-label="接口返回">{{ result }}</pre>
  </section>
</template>

<style scoped>
.api-demo { padding: 16px; }
form, label { display: grid; gap: 8px; }
form { max-width: 440px; }
input, select, button { padding: 8px; font: inherit; }
pre { overflow: auto; max-height: 420px; }
</style>
