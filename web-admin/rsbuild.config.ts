import path from 'node:path'
import { defineConfig } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'

export default defineConfig(() => {
  const serverUrl = process.env.VITE_BACKEND_URL || 'http://localhost:8080'

  return {
    plugins: [pluginReact()],
    source: { entry: { index: './src/main.tsx' } },
    resolve: { alias: { '@': path.resolve(import.meta.dirname, './src') } },
    html: { template: './index.html' },
    server: {
      host: '0.0.0.0',
      proxy: {
        '/v1': { target: serverUrl, changeOrigin: true },
        '/api': {
          target: serverUrl,
          changeOrigin: true,
          pathRewrite: { '^/api': '' },
        },
      },
    },
  }
})
