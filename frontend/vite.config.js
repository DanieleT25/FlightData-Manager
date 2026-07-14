import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte()],
  server: {
    proxy: {
      '/api': {
        target: 'https://localhost:3443', 
        changeOrigin: true,
        secure: false, 
        rewrite: (path) => path.replace(/^\/api/, '') 
      }
    }
  }
})