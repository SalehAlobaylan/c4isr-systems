import {useMemo, useState} from 'react'

type Coastal = {
  grid: {edition: string; datum: string; window: number[]; elevation: number[]; tid: number[]; spacingDegrees: number}
  summary: {coastline: number[][][]; minimumElevationM: number; coastVersion: string; missingSourceType: number; nearCoast: Record<string,number>}
}
const sourceTypes: Record<number,string> = {0:'Land',10:'Singlebeam',11:'Multibeam',12:'Seismic',13:'Isolated sounding',
  14:'Chart sounding',15:'Lidar',16:'Optical depth',17:'Multiple direct methods',40:'Gravity-guided prediction',41:'Interpolation',
  42:'Chart contours',43:'ENC contours',44:'Mixed measured and derived depths',45:'Aircraft gravity prediction',46:'Grounded iceberg draft',
  47:'Grounded Argo float',48:'Animal-borne data logger',70:'Pre-generated mixed-source grid',71:'Unknown source',72:'Steering points',127:'Missing source type'}
export function CoastalContext({value}: {value: Coastal}) {
  const {grid,summary}=value
  const [x0,y0,x1,y1]=grid.window
  const w=x1-x0+1,h=y1-y0+1
  const [col,setCol]=useState(0),[row,setRow]=useState(0)
  const [showCoast,setShowCoast]=useState(true)
  const raster=useMemo(()=>{
    const canvas=document.createElement('canvas')
    canvas.width=w;canvas.height=h
    const ctx=canvas.getContext('2d')
    if(!ctx)return undefined
    const pixels=ctx.createImageData(w,h)
    for(let r=0;r<h;r++)for(let c=0;c<w;c++){
      const i=r*w+c,e=grid.elevation[i],t=grid.tid[i],out=((h-1-r)*w+c)*4
      const q=Math.min(1,Math.max(0,-e/Math.max(1,-summary.minimumElevationM)))
      pixels.data.set(t===127?[78,83,95,255]:e<0&&t!==0?[29,Math.round(166-105*q),Math.round(217-57*q),255]:[144,131,100,255],out)
    }
    ctx.putImageData(pixels,0,0)
    return canvas.toDataURL()
  },[grid,h,w,summary.minimumElevationM])
  const i=row*w+col
  const depth=grid.elevation[i]<0 && grid.tid[i]!==0 && grid.tid[i]!==127 ? -grid.elevation[i] : null
  return <details open>
    <summary>Regional seabed and coastline</summary>
    <p>{grid.edition} · 15 arc-seconds, pixel centres · {grid.datum}. Regional context only. The grid cannot support navigation or port-scale depth decisions.</p>
    <label><input type="checkbox" checked={showCoast} onChange={e=>setShowCoast(e.target.checked)} /> Show separate Natural Earth coastline</label>
    <svg viewBox={`0 0 ${w} ${h}`} role="img" aria-label="Native seabed samples and independent coastline"
      style={{width:'min(100%, 44rem)', display:'block'}}>
      {raster&&<image href={raster} width={w} height={h} style={{imageRendering:'pixelated'}} />}
      {showCoast&&summary.coastline.map((line,k)=><polyline key={k} points={line.map(([lon,lat])=>`${(lon+180)*240-x0},${h-((lat+90)*240-y0)}`).join(' ')}
        fill="none" stroke="#ffdb80" strokeWidth=".8" />)}
      <rect x={col} y={h-1-row} width="1" height="1" fill="none" stroke="white" strokeWidth=".45" />
    </svg>
    <p>Blue marks seabed depth. Brown marks land or other non-depth cells. Gray marks missing source type. The coastline is a separate reference and may not align with depth cells.</p>
    <label>Native column <input className="atlas-field" type="number" min="1" max={w} value={col+1}
      onChange={e=>setCol(Math.min(w-1,Math.max(0,Math.trunc(Number(e.target.value))-1)))} /></label>
    <label> Native row, south to north <input className="atlas-field" type="number" min="1" max={h} value={row+1}
      onChange={e=>setRow(Math.min(h-1,Math.max(0,Math.trunc(Number(e.target.value))-1)))} /></label>
    <p aria-live="polite">{(-90+(y0+row+.5)/240).toFixed(6)}° N, {(-180+(x0+col+.5)/240).toFixed(6)}° E.
      {' '}Signed elevation {grid.elevation[i]} m. {depth!==null?`Depth ${depth} m.`:'No seabed depth assigned.'}
      {' '}Source type {grid.tid[i]}: {sourceTypes[grid.tid[i]] ?? 'Unknown'}.</p>
    <p>{summary.nearCoast.cells ?? 0} cells within one grid spacing of Natural Earth coastline v{summary.coastVersion}.
      {' '}{summary.nearCoast.nonLandNonnegative ?? 0} have a marine source type and nonnegative elevation.
      {' '}{summary.missingSourceType} cells have no source type. No vertical conversion or shoreline stitching was applied.</p>
  </details>
}
