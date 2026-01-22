import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// Read backend port from environment variables for development
const backendPort = process.env.VITE_BACKEND_PORT || '28080'
const frontendPort = parseInt(process.env.VITE_FRONTEND_PORT || '25173')

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: frontendPort,
    proxy: {
      // Proxy API requests to Go backend
      '/v1': {
        target: `http://localhost:${backendPort}`,
        changeOrigin: true,
      },
      '/health': {
        target: `http://localhost:${backendPort}`,
        changeOrigin: true,
      },
      '/openapi.json': {
        target: `http://localhost:${backendPort}`,
        changeOrigin: true,
      },
      '/openapi.yaml': {
        target: `http://localhost:${backendPort}`,
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          recharts: ['recharts'],
          vendor: ['react', 'react-dom', 'react-router-dom'],
        },
      },
    },
  },
})
