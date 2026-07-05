import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    proxy: {
      // Le serveur Go tourne sur :8080 (go run .) ; sans ce proxy, les
      // appels relatifs fetch('/api/...') du client atterrissent sur le
      // serveur Vite lui-même et échouent (404).
      '/api': 'http://localhost:8080'
    }
  }
})
