import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// base './' makes the production build relocatable: umbra-api can serve the
// static bundle, or any static host, without rewriting asset paths.
export default defineConfig({
  plugins: [react()],
  base: './',
  server: {
    port: 5173,
    strictPort: true,
  },
})
