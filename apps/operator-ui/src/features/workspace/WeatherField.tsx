import {useState} from 'react'
import type {components} from '@/lib/openapi.generated'

type Field = components['schemas']['EnvironmentWeatherField']
const labels: Record<string,string> = {temperature:'Temperature', u_component_of_wind:'Eastward wind', v_component_of_wind:'Northward wind'}

export function WeatherField({field}: {field: Field}) {
  const [variable, setVariable] = useState('temperature')
  const [time, setTime] = useState(0)
  const [level, setLevel] = useState(0)
  const [cell, setCell] = useState(0)
  const width = field.longitudes.length, height = field.latitudes.length
  const start = (time * field.pressureHPa.length + level) * width * height
  const series = field.variables[variable]
  if (!series || !width || !height) return null
  const values = series.samples.slice(start, start+width*height)
  const finite = values.filter((sample): sample is number => sample !== null)
  const lo = finite.length ? Math.min(...finite) : 0, hi = finite.length ? Math.max(...finite) : 0
  const unit = series.units === 'm s**-1' ? 'm/s' : series.units
  const selected = values[cell]
  const lon = field.longitudes[cell % width], lat = field.latitudes[Math.floor(cell / width)]
  return <details>
    <summary>Historical weather fields</summary>
    <p>{field.edition}. {field.nativeSpacingDegrees}° sample spacing. Pressure levels are in hPa. This reanalysis has no geometric height assignment or local accuracy claim.</p>
    <p>Station observations remain a separate layer. Contains modified Copernicus Climate Change Service information (2022).</p>
    <fieldset className="ux-actions">
      <legend>Native pressure slice</legend>
      <label>Variable <select className="atlas-field" value={variable} onChange={event => setVariable(event.target.value)}>
        {Object.keys(field.variables).map(name => <option key={name} value={name}>{labels[name] ?? name}</option>)}
      </select></label>
      <label>Valid time <select className="atlas-field" value={time} onChange={event => setTime(Number(event.target.value))}>
        {field.validTimes.map((at,i) => <option key={at} value={i}>{at}</option>)}
      </select></label>
      <label>Pressure level <select className="atlas-field" value={level} onChange={event => setLevel(Number(event.target.value))}>
        {field.pressureHPa.map((pressure,i) => <option key={pressure} value={i}>{pressure} hPa</option>)}
      </select></label>
    </fieldset>
    <p>North at top. {finite.length ? `${lo.toFixed(2)} to ${hi.toFixed(2)} ${unit} across available samples.` : 'No available samples.'}</p>
    <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={`${labels[variable]} native grid at ${field.pressureHPa[level]} hPa`}
      style={{width:'min(100%, 18rem)', border:'1px solid currentColor'}} shapeRendering="crispEdges">
      {values.map((value,i) => {
        const scaled = value === null || hi === lo ? 0 : (value-lo)/(hi-lo)
        return <rect key={i} x={i%width} y={Math.floor(i/width)} width="1" height="1"
          fill={value === null ? '#808080' : `hsl(${215-scaled*185} 73% ${36+scaled*20}%)`}
          stroke={i === cell ? '#fff' : 'none'} strokeWidth=".06" onClick={() => setCell(i)} />
      })}
    </svg>
    <label className="block">Inspect native grid point <select className="atlas-field" value={cell} onChange={event => setCell(Number(event.target.value))}>
      {values.map((_,i) => <option key={i} value={i}>{field.latitudes[Math.floor(i/width)]}° latitude, {field.longitudes[i%width]}° longitude</option>)}
    </select></label>
    <p aria-live="polite">{lat}°, {lon}° · {selected === null ? field.aboveGroundMask[start+cell] ? 'Missing source sample' : 'Below-ground or invalid surface pressure' : `${selected} ${unit}`}</p>
  </details>
}
