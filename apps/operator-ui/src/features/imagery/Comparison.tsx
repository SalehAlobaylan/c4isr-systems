import { useEffect, useRef, useState } from 'react'
import type { ImageryResult } from '@/lib/api'
import {ChangeMap} from './ChangeMap'

function color(value: number, difference: boolean): string {
  const magnitude = Math.min(1, Math.abs(value) / (difference ? 2 : 1))
  if (difference) return value < 0 ? `rgb(${245 - magnitude * 190},${242 - magnitude * 140},${235 - magnitude * 35})` : `rgb(${245 - magnitude * 15},${242 - magnitude * 135},${235 - magnitude * 200})`
  const f = (Math.max(-1, Math.min(1, value)) + 1) / 2
  return `rgb(${190 - f * 150},${125 + f * 40},${65 + f * 35})`
}
function PixelPanel({ values, result, label, difference, selected, select }: { values: (number | null)[]; result: ImageryResult; label: string; difference: boolean; selected: number; select: (index: number) => void }) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const box = useRef<HTMLButtonElement>(null)
  const { width, height } = result.grid
  useEffect(() => {
    const node = canvas.current, container = box.current
    if (!node || !container || width * height !== values.length || width > 256 || height > 256) return
    const pixels = document.createElement('canvas'); pixels.width = width; pixels.height = height
    const source = pixels.getContext('2d'); if (!source) return
    values.forEach((value, i) => { source.fillStyle = value === null ? ((i % width + Math.floor(i / width)) % 2 ? '#9b9d9d' : '#d3d4d4') : color(value, difference); source.fillRect(i % width, Math.floor(i / width), 1, 1) })
    const draw = () => {
      const scale = Math.min(512 / width, 512 / height, container.clientWidth * Math.min(window.devicePixelRatio || 1, 2) / width)
      node.width = Math.max(1, Math.round(width * scale)); node.height = Math.max(1, Math.round(height * scale))
      const ctx = node.getContext('2d'); if (!ctx) return
      ctx.imageSmoothingEnabled = false; ctx.drawImage(pixels, 0, 0, node.width, node.height)
      if (difference && result.findingGeometry) {
        const geometry = result.findingGeometry as { type?: string; coordinates?: number[][][] | number[][][][] }
        const polygons = geometry.type === 'Polygon' ? [geometry.coordinates as number[][][]] : geometry.type === 'MultiPolygon' ? geometry.coordinates as number[][][][] : []
        ctx.strokeStyle = '#96225c'; ctx.lineWidth = 2; ctx.setLineDash([4, 2])
        for (const polygon of polygons ?? []) for (const ring of polygon) {
          ctx.beginPath()
          ring.forEach(([lng, lat], i) => { const x = (lng - result.grid.west) / result.grid.stepLng * node.width / width; const y = (result.grid.north - lat) / result.grid.stepLat * node.height / height; if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y) })
          ctx.closePath(); ctx.stroke()
        }
        ctx.setLineDash([])
      }
      ctx.strokeStyle = '#101010'; ctx.lineWidth = 2
      ctx.strokeRect((selected % width) * node.width / width, Math.floor(selected / width) * node.height / height, node.width / width, node.height / height)
    }
    draw(); const observer = new ResizeObserver(draw); observer.observe(container)
    return () => observer.disconnect()
  }, [values, width, height, selected, difference, result])
  return <figure className="min-w-0"><figcaption className="mb-1 font-semibold">{label}</figcaption><button type="button" ref={box} style={{ maxWidth: Math.min(512, 384 * width / height) }} className="block w-full border border-edge focus-visible:outline-2" aria-label={`Inspect ${label} cells; arrow keys move the selected cell`} onClick={(e) => { const rect = e.currentTarget.getBoundingClientRect(); const col = Math.min(width - 1, Math.max(0, Math.floor((e.clientX - rect.left) / rect.width * width))); const row = Math.min(height - 1, Math.max(0, Math.floor((e.clientY - rect.top) / rect.height * height))); select(row * width + col) }} onKeyDown={(e) => { const delta = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : e.key === 'ArrowDown' ? width : e.key === 'ArrowUp' ? -width : 0; if (delta) { e.preventDefault(); select(Math.max(0, Math.min(values.length - 1, selected + delta))) } }}><canvas ref={canvas} aria-hidden="true" style={{ width: '100%', aspectRatio: `${width}/${height}`, imageRendering: 'pixelated' }} /></button><p className="text-xs">{difference ? '−2 decrease (blue) · 0 unchanged · +2 increase (orange)' : 'NDVI −1 to +1 · brown to green'}. Grey checks: masked/no data.{difference && ' Dashed purple boundary: thresholded finding geometry.'}</p></figure>
}
export function Comparison({ result }: { result: ImageryResult }) {
  const [selected, select] = useState(0)
  const count = result.grid.width * result.grid.height
  if (count < 1 || count > 65_536 || result.beforeNdvi.length !== count || result.afterNdvi.length !== count || result.delta.length !== count) return <p role="alert">The comparison grid is unavailable or exceeds its display limit.</p>
  const cell = Math.min(count - 1, selected)
  const number = (v: number | null | undefined) => v == null ? 'Masked/no data' : v.toFixed(6)
  const g = result.grid, row = Math.floor(cell / g.width), col = cell % g.width
  return <section aria-label="Aligned NDVI comparison" className="space-y-3">
    <h3 className="font-semibold">Aligned vegetation-index comparison</h3>
    {result.synthetic && <p role="status" className="font-semibold">SYNTHETIC FIXTURE — these pixels are not observed environmental evidence.</p>}
    <p>{result.validCells} common clear cells · {result.maskedCells} masked cells · {result.declineCells} cells at or below −{result.parameters.declineThreshold} · mean difference {result.meanDelta.toFixed(6)}.</p>
    <ChangeMap result={result}/>
    <div className="grid grid-cols-1 gap-4 md:grid-cols-3"><PixelPanel values={result.beforeNdvi} result={result} label="Before NDVI" difference={false} selected={cell} select={select} /><PixelPanel values={result.afterNdvi} result={result} label="After NDVI" difference={false} selected={cell} select={select} /><PixelPanel values={result.delta} result={result} label="NDVI difference" difference selected={cell} select={select} /></div>
    <label className="block">Inspect cell index (zero-based)<input className="ml-2 w-24 rounded border border-edge bg-panel p-2" type="number" min={0} max={count - 1} value={cell} onChange={(e) => select(Math.max(0, Math.min(count - 1, Number(e.target.value) || 0)))} /></label>
    <p aria-live="polite">Cell {cell} · row {row}, column {col}. Before: {number(result.beforeNdvi[cell])}; after: {number(result.afterNdvi[cell])}; difference: {number(result.delta[cell])}.</p>
    <p className="text-xs">EPSG:4326 cell edges · west {(g.west + col * g.stepLng).toFixed(6)}, north {(g.north - row * g.stepLat).toFixed(6)} · spacing {g.stepLng}° × {g.stepLat}°. Angular cells do not imply constant metre resolution or a measured affected area.</p>
    <ul className="list-disc space-y-1 pl-5">{result.limitations.map((line) => <li key={line}>{line}</li>)}</ul>
    <details><summary>Derived finding geometry and processing lineage</summary><p>{result.algorithm} · {result.geometryProcessor}</p><p>The finding geometry describes thresholded analysis cells, not a verified environmental boundary.</p><pre className="max-h-48 overflow-auto whitespace-pre-wrap">{JSON.stringify(result.findingGeometry, null, 2)}</pre></details>
  </section>
}
