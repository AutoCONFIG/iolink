<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ApiClient, createFetchRequest, type MiniApi, type SubscriptionPort } from '../api'
import { alarmLevelLabel, openAlarms, subscriptionDecisionLabel } from '../domain/alarms'
import type { AlarmSummary, SubscriptionDecision } from '../types'

const props = defineProps<{ api?: MiniApi; subscription?: SubscriptionPort; templateId?: string }>()
const api = computed(() => props.api ?? new ApiClient(createFetchRequest()))
const alarms = ref<AlarmSummary[]>([])
const loading = ref(false)
const error = ref('')
const confirming = ref<number | null>(null)
const subscriptionDecision = ref<SubscriptionDecision>('not_requested')
const subscriptionBusy = ref(false)

const pendingAlarms = computed(() => openAlarms(alarms.value))
const demoMode = computed(() => !props.subscription)

async function load() {
  loading.value = true
  error.value = ''
  try { alarms.value = await api.value.alarms() }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '报警加载失败' }
  finally { loading.value = false }
}

async function confirmAlarm(alarm: AlarmSummary) {
  confirming.value = alarm.id
  error.value = ''
  try {
    await api.value.confirmAlarm(alarm.id)
    alarms.value = alarms.value.map((item) => item.id === alarm.id ? { ...item, confirmed_at: new Date().toISOString() } : item)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '确认报警失败' }
  finally { confirming.value = null }
}

async function requestSubscription() {
  subscriptionBusy.value = true
  error.value = ''
  try {
    if (!props.subscription) { subscriptionDecision.value = 'cancelled'; return }
    const result = await props.subscription.request([props.templateId ?? 'alarm-template-demo'])
    subscriptionDecision.value = result.decision
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '订阅授权失败' }
  finally { subscriptionBusy.value = false }
}

function setDemoDecision(decision: Exclude<SubscriptionDecision, 'not_requested'>) { subscriptionDecision.value = decision }

onMounted(load)
</script>

<template>
  <main data-page="alarms" class="alarm-page">
    <header class="alarm-header"><div><h1>报警中心</h1><p>查看未确认报警，并演示通知订阅授权。</p></div><button type="button" :disabled="loading" @click="load">{{ loading ? '加载中…' : '刷新' }}</button></header>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="loading" class="state" role="status">正在加载报警</p>
    <p v-else-if="!alarms.length" class="state">当前没有报警</p>
    <ul v-else class="alarm-list" aria-live="polite">
      <li v-for="alarm in alarms" :key="alarm.id" :class="['alarm-item', `alarm-${alarm.level}`, { confirmed: alarm.confirmed_at }]">
        <div class="alarm-copy"><strong>{{ alarmLevelLabel(alarm.level) }} · {{ alarm.message }}</strong><small>{{ alarm.metric }} · 当前 {{ alarm.current_value }} · 阈值 {{ alarm.threshold }}</small><time :datetime="alarm.created_at">{{ new Date(alarm.created_at).toLocaleString() }}</time></div>
        <button v-if="!alarm.confirmed_at" type="button" :disabled="confirming === alarm.id" @click="confirmAlarm(alarm)">{{ confirming === alarm.id ? '确认中…' : '确认报警' }}</button>
        <span v-else class="confirmed-label">已确认</span>
      </li>
    </ul>
    <section class="subscription" aria-labelledby="subscription-title">
      <div><h2 id="subscription-title">报警通知订阅</h2><p>{{ subscriptionDecisionLabel(subscriptionDecision) }}。拒绝或取消不会影响报警中心。</p></div>
      <div class="subscription-actions">
        <button type="button" :disabled="subscriptionBusy" @click="requestSubscription">{{ subscriptionBusy ? '处理中…' : '请求微信订阅' }}</button>
        <template v-if="demoMode"><button type="button" class="secondary" @click="setDemoDecision('agreed')">模拟同意</button><button type="button" class="secondary" @click="setDemoDecision('denied')">模拟拒绝</button><button type="button" class="secondary" @click="setDemoDecision('cancelled')">模拟取消</button></template>
      </div>
      <small v-if="demoMode" class="demo-note">当前为浏览器演示，真实微信弹窗需在小程序运行时接入。</small>
    </section>
    <p class="summary">未确认 {{ pendingAlarms.length }} 条 / 共 {{ alarms.length }} 条</p>
  </main>
</template>

<style scoped>
.alarm-page { --mini-ink: #183047; --mini-muted: #6c8192; --mini-line: #dbe6ec; --mini-accent: #0fae9b; --mini-warning: #e7a83a; --mini-danger: #e85f5f; color: var(--mini-ink); display: grid; gap: 16px; padding: 20px; }
.alarm-header { align-items: center; display: flex; gap: 16px; justify-content: space-between; }
h1, h2, p { margin: 0; } h1 { font-size: 1.5rem; } h2 { font-size: 1rem; } p, small, time { color: var(--mini-muted); }
button { background: var(--mini-accent); border: 0; border-radius: 8px; color: #fff; cursor: pointer; padding: 8px 12px; } button:disabled { cursor: wait; opacity: .6; }
.secondary { background: transparent; border: 1px solid var(--mini-line); color: var(--mini-ink); }
.error { color: var(--mini-danger); } .state { border: 1px dashed var(--mini-line); padding: 20px; text-align: center; }
.alarm-list { display: grid; gap: 8px; list-style: none; margin: 0; padding: 0; }
.alarm-item { align-items: center; border: 1px solid var(--mini-line); border-radius: 10px; display: flex; gap: 12px; justify-content: space-between; padding: 12px; }
.alarm-item.alarm-critical { border-color: color-mix(in srgb, var(--mini-danger) 45%, var(--mini-line)); } .alarm-item.alarm-warning { border-color: color-mix(in srgb, var(--mini-warning) 45%, var(--mini-line)); }
.alarm-item.confirmed { opacity: .7; } .alarm-copy { display: grid; gap: 4px; } .alarm-copy strong { color: var(--mini-ink); } .confirmed-label { color: var(--mini-muted); }
.subscription { border-top: 1px solid var(--mini-line); display: grid; gap: 10px; padding-top: 16px; } .subscription-actions { display: flex; flex-wrap: wrap; gap: 8px; } .demo-note, .summary { font-size: .875rem; }
@media (max-width: 560px) { .alarm-page { padding: 16px; } .alarm-header, .alarm-item { align-items: stretch; flex-direction: column; } .alarm-item button { align-self: flex-start; } }
</style>
