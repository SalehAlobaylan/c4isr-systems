/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_TOKEN?: string
  readonly VITE_LOCAL_AUTO_ENTRY?: string
  readonly VITE_API_BASE_URL?: string
  readonly VITE_MAP_STYLE_URL?: string
  readonly VITE_LOCAL_BASEMAP_ONLY?: string
  readonly VITE_GEOGRAPHIC_PREVIEW_ENABLED?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

interface Window {
  __C4ISR_CONFIG__?: {
    apiBaseUrl?: string
    apiToken?: string
    localAutoEntry?: boolean
    mapStyleUrl?: string
    localBasemapOnly?: string
    geographicPreviewEnabled?: string
  }
}
