<script setup lang="ts">
import { computed, ref } from 'vue'
import { PAGES } from './src/pages'
import { createMiniApp } from './src/app'
import LoginPage from './src/pages/LoginPage.vue'
import HomePage from './src/pages/HomePage.vue'
import PondsPage from './src/pages/PondsPage.vue'
import RealtimePage from './src/pages/RealtimePage.vue'
import HistoryPage from './src/pages/HistoryPage.vue'
import AlarmsPage from './src/pages/AlarmsPage.vue'
import ApiDemo from './src/ApiDemo.vue'

const app = createMiniApp(() => undefined)
const current = ref(app.current)
const components = { login: LoginPage, home: HomePage, ponds: PondsPage, realtime: RealtimePage, history: HistoryPage, alarms: AlarmsPage }
const view = computed(() => components[current.value])
function navigate(page: typeof current.value) { app.go(page); current.value = app.current }
</script>
<template><div class="mini-shell"><component :is="view" /><ApiDemo v-if="current !== 'alarms'" :key="current" :page="current" /><nav aria-label="小程序页面导航"><button v-for="page in PAGES" :key="page.id" :aria-current="current === page.id ? 'page' : undefined" @click="navigate(page.id)">{{ page.title }}</button></nav></div></template>
