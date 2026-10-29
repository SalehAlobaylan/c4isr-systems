// Accept Arabic/Persian digits and Arabic separators while keeping coordinate
// order and signs explicit. Direction controls cannot change parsed numbers.
export const normalizeNumbers=(value:string)=>value.replace(/[\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069]/g,'').replace(/[٠-٩]/g,c=>String(c.charCodeAt(0)-0x660)).replace(/[۰-۹]/g,c=>String(c.charCodeAt(0)-0x6f0)).replace(/٫/g,'.').replace(/،/g,',').replace(/−/g,'-')
export function longitudeLatitude(value:string):[number,number]|undefined{
 const parts=normalizeNumbers(value).split(',');if(parts.length!==2||parts.some(v=>!/^[-+]?(?:\d+\.?\d*|\.\d+)$/.test(v.trim())))return
 const [lng,lat]=parts.map(Number);if(!Number.isFinite(lng)||!Number.isFinite(lat)||Math.abs(lng)>180||Math.abs(lat)>90)return
 return [lng,lat]
}
export const pointerLabel=([lng,lat]:[number,number])=>`${lng.toFixed(4)}° longitude, ${lat.toFixed(4)}° latitude · WGS84`
