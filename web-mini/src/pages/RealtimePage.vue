<script setup lang="ts">
import { computed, ref } from 'vue'
import type { DeviceModelLatest } from '../types'

const deviceNo = ref('')
const loading = ref(false)
const error = ref('')
const latest = ref<DeviceModelLatest | null>(null)
const displayValue = (identifier: string) => {
  const value = latest.value?.properties[identifier]
  return value === undefined || value === null ? '暂无' : typeof value === 'boolean' ? (value ? '是' : '否') : String(value)
}
const load = async () => {
  if (!deviceNo.value.trim()) { error.value = '请输入设备编号'; return }
  loading.value = true; error.value = ''
  try {
    const token = localStorage.getItem('iolink.mini.token')
    if (!token) throw new Error('请先登录')
    const response = await fetch(`/api/v2/devices/${encodeURIComponent(deviceNo.value.trim())}/model/latest`, { headers: { Accept: 'application/json', Authorization: `Bearer ${token}` } })
    if (!response.ok) throw new Error(response.status === 404 ? '设备不存在或无权访问' : '实时数据加载失败')
    latest.value = await response.json() as DeviceModelLatest
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '实时数据加载失败' }
  finally { loading.value = false }
}
const readableFields = computed(() => latest.value?.fields.filter((field) => field.readable) ?? [])
</script>

<template>
  <main data-page="realtime">
    <h1>设备实时详情</h1>
    <p>按已发布物模型展示可读字段，枚举和布尔值保留原始语义。</p>
    <form class="device-query" @submit.prevent="load"><label>设备编号 <input v-model="deviceNo" placeholder="例如 pond-01" /></label><button type="submit" :disabled="loading">{{ loading ? '加载中…' : '读取详情' }}</button></form>
    <p v-if="error" role="alert">{{ error }}</p>
    <section v-if="latest" aria-live="polite"><p>模型 v{{ latest.model_version }} · {{ latest.ts ? new Date(latest.ts).toLocaleString() : '暂无上报' }}</p><dl><template v-for="field in readableFields" :key="field.identifier"><dt>{{ field.identifier }}<small v-if="field.unit">（{{ field.unit }}）</small></dt><dd>{{ displayValue(field.identifier) }}</dd></template></dl><p v-if="!readableFields.length">当前模型没有可读字段。</p></section>
  </main>
</template>

<style scoped>
.device-query { display: flex; gap: 8px; align-items: end; margin: 16px 0; }
.device-query label { display: grid; gap: 4px; }
.device-query input { min-width: 220px; padding: 8px; }
.device-query button { padding: 8px 14px; }
dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; }
dt, dd { margin: 0; }
dt { color: #6c8192; }
dd { font-size: 1.3rem; font-weight: 600; }
small { font-weight: 400; }
</style>
