import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../internal/web/static/react',
    emptyOutDir: true,
    manifest: true,
    rollupOptions: { output: { entryFileNames: 'assets/index.js', chunkFileNames: 'assets/[name].js', assetFileNames: 'assets/[name][extname]' } }
  }
})
