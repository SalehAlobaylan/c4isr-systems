export interface LngLat {
  lng: number
  lat: number
}

const EARTH_RADIUS_M = 6_371_008.8

const toRadians = (degrees: number) => (degrees * Math.PI) / 180

/** Great-circle distance between two WGS84 points in meters. */
export function haversineMeters(a: LngLat, b: LngLat): number {
  const dLat = toRadians(b.lat - a.lat)
  const dLng = toRadians(b.lng - a.lng)
  const lat1 = toRadians(a.lat)
  const lat2 = toRadians(b.lat)
  const h =
    Math.sin(dLat / 2) ** 2 + Math.sin(dLng / 2) ** 2 * Math.cos(lat1) * Math.cos(lat2)
  return 2 * EARTH_RADIUS_M * Math.asin(Math.min(1, Math.sqrt(h)))
}
