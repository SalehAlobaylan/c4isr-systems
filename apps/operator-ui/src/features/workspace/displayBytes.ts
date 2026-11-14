import type {components} from '@/lib/openapi.generated'
import type {SceneFeature, VectorDisplay} from './GeographicSceneLayers'

export type Display = components['schemas']['EnvironmentDisplay']
const limit = 32 * 1024 * 1024

export type DisplaySource = (id: string, signal: AbortSignal) => Promise<{body: ReadableStream<Uint8Array>; etag: string | null}>

export async function fetchDisplayBytes(source: DisplaySource, display: Display, signal: AbortSignal) {
  signal.throwIfAborted()
  if (!crypto.subtle) throw new Error('Use a secure local connection to verify display bytes.')
  if (!Number.isSafeInteger(display.bytes) || display.bytes < 1 || display.bytes > limit) throw new Error('The display exceeds the browser budget.')
  const {body, etag} = await source(display.id, signal)
  if (etag !== `"${display.sha256}"`) {
    await body.cancel()
    throw new Error('The display response identifies different bytes.')
  }
  const reader = body.getReader()
  const bytes = new Uint8Array(display.bytes)
  let offset = 0
  try {
    while (true) {
      const {done, value} = await reader.read()
      if (done) break
      if (offset + value.byteLength > bytes.byteLength) throw new Error('The display exceeds its declared size.')
      bytes.set(value, offset)
      offset += value.byteLength
    }
  } finally {
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
  if (offset !== bytes.byteLength) throw new Error('The display download ended early.')
  const digest = await crypto.subtle.digest('SHA-256', bytes)
  const hash = Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, '0')).join('')
  if (hash !== display.sha256) throw new Error(`${display.mediaType === 'model/gltf-binary' || display.recipe.includes('-glb-') ? 'Terrain' : 'Layer'} integrity check failed.`)
  signal.throwIfAborted()
  return bytes.buffer
}

export function readSceneFeature(bytes: ArrayBuffer, display: Display, featureId: string): SceneFeature {
  const value = JSON.parse(new TextDecoder('utf-8', {fatal: true}).decode(bytes))
  if (value.schema !== 'emad.display-layer.v1' || value.manifestSha256 !== display.manifestSha256 ||
    !['roads', 'buildings', 'water'].includes(value.layer) || display.recipe !== `${value.layer}-display-v1`) {
    throw new Error('The linked source layer is unavailable.')
  }
  const feature = (value.data as VectorDisplay).features.find(item => item.id === featureId)
  if (!feature) throw new Error('The linked feature is unavailable in this edition.')
  return feature
}
