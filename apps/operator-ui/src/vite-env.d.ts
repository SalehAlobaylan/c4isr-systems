/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_TOKEN?: string
  readonly VITE_API_BASE_URL?: string
  readonly VITE_MAP_STYLE_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

interface Window {
  __C4ISR_CONFIG__?: {
    apiBaseUrl?: string
    apiToken?: string
    mapStyleUrl?: string
  }
}
