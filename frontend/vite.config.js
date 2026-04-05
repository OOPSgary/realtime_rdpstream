import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      // 开发模式下将 WHIP/WHEP 请求代理到 Go 后端
      '/whip': 'http://localhost:8080',
      '/whep': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
  },
})
