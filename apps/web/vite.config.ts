import path from 'node:path'
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// 通过环境变量 VITE_BASE 控制部署路径前缀，例如：
//   - 默认 '/'（直接挂在域名根下）
//   - '/doc/'（被反向代理到 /doc/ 子路径下）
// 末尾必须带 '/'。
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const rawBase = (env.VITE_BASE || '/').trim()
  const base = rawBase.endsWith('/') ? rawBase : rawBase + '/'

  return {
    base,
    plugins: [react()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      port: 5173,
      proxy: {
        '/api': { target: 'http://localhost:8787', changeOrigin: true },
        '/d':   { target: 'http://localhost:8787', changeOrigin: true },
        '/p':   { target: 'http://localhost:8787', changeOrigin: true },
        '/ws':  { target: 'ws://localhost:8787', ws: true, changeOrigin: true },
      },
    },
  }
})
