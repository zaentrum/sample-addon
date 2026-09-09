import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import federation from '@originjs/vite-plugin-federation'

// The console is a federated REMOTE: the portal shell renders it inside its
// own React tree — one DOM, one React instance, no iframe. This file IS the
// hosting contract, line by line:
//
//   name / filename  the shell fetches <base>/embed/assets/remoteEntry.js
//   exposes          the module './Console' must export a NAMED `Console`
//   shared           react + react-dom at ^18.3.0 — the shell provides them
//   console.css      a predictable stylesheet name the shell links on mount
//
// Copy this file verbatim for your own addon and change only `name`.
export default defineConfig({
  plugins: [
    react(),
    federation({
      name: 'sample',
      filename: 'remoteEntry.js',
      exposes: { './Console': './src/Console.tsx' },
      shared: {
        react: { requiredVersion: '^18.3.0' },
        'react-dom': { requiredVersion: '^18.3.0' },
      },
    }),
  ],
  build: {
    outDir: '../internal/api/web/embed',
    emptyOutDir: true,
    target: 'esnext',
    minify: true,
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        format: 'es',
        assetFileNames: (info) =>
          info.name && info.name.endsWith('.css') ? 'assets/console.css' : 'assets/[name]-[hash][extname]',
      },
    },
  },
})
