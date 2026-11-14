import {fileURLToPath} from 'node:url'
import {readFileSync} from 'node:fs'
import react from '@vitejs/plugin-react'
import {defineConfig} from 'vite'

export default defineConfig({
  root: fileURLToPath(new URL('./geographic-viewer', import.meta.url)),
  base: './',
  publicDir: false,
  plugins: [react(), {
    name: 'geographic-viewer-font-licenses',
    generateBundle() {
      for (const family of ['ibm-plex-sans', 'ibm-plex-sans-arabic']) {
        const source = readFileSync(new URL(`./node_modules/@fontsource/${family}/LICENSE`, import.meta.url))
        this.emitFile({type: 'asset', fileName: `licenses/${family}.txt`, source})
      }
      const launcher = readFileSync(new URL('../../scripts/ops/viewing_package.py', import.meta.url))
      this.emitFile({type: 'asset', fileName: 'launcher/viewing_package.py', source: launcher})
    },
  }],
  resolve: {alias: {'@': fileURLToPath(new URL('./src', import.meta.url))}},
  build: {
    outDir: fileURLToPath(new URL('./dist-geographic-viewer', import.meta.url)),
    emptyOutDir: true,
    assetsInlineLimit: 0,
  },
})
