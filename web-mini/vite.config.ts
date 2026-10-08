import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: { proxy: { '/api': process.env['IOLINK_API_TARGET'] ?? 'http://127.0.0.1:8080' } },
  test: { environment: 'happy-dom' },
})
