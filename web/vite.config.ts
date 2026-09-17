import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      preserveEntrySignatures: 'exports-only',
      input: {
        main: './index.html',
        embed: './src/embed.tsx',
      },
      output: {
        entryFileNames: (chunkInfo) => {
          if (chunkInfo.name === 'embed') {
            return 'assets/embed.js'
          }
          return 'assets/[name]-[hash].js'
        },
        assetFileNames: (assetInfo) => {
          if (assetInfo.name && assetInfo.name.endsWith('.css')) {
            return 'assets/embed.css'
          }
          return 'assets/[name]-[hash][extname]'
        },
      },
    },
  },
  server: {
    port: 5175,
    proxy: {
      '/dobby/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/dobby/, ''),
        headers: {
          Authorization: 'Bearer dev-admin-token',
        },
      },
    },
  },
})
