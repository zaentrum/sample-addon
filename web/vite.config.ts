import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Standalone dev server only. `npm run dev` proxies /api to a locally running
// addon (go run .) so the console can be developed without the portal.
export default defineConfig({
  plugins: [react()],
  server: { proxy: { '/api': 'http://localhost:8080' } },
})
