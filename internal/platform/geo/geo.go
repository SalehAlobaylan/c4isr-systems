// Package geo holds lightweight geometry value objects shared by the domain,
// scenario simulation, and transports. All authoritative geospatial decisions
// are made by PostGIS; this package is for validation and simulation math.
package geo

import "math"

// EarthRadiusMeters is the mean Earth radius used for simulation math.
const EarthRadiusMeters = 6371000.0

// Point is a WGS84 coordinate.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Valid reports whether the point is a finite WGS84 coordinate.
func (p Point) Valid() bool {
	if math.IsNaN(p.Lat) || math.IsNaN(p.Lng) || math.IsInf(p.Lat, 0) || math.IsInf(p.Lng, 0) {
		return false
	}
	return p.Lat >= -90 && p.Lat <= 90 && p.Lng >= -180 && p.Lng <= 180
}

// DistanceMeters returns the haversine distance between two points.
func DistanceMeters(a, b Point) float64 {
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * EarthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(h)))
}

// BearingDegrees returns the initial bearing from a to b in [0, 360).
func BearingDegrees(a, b Point) float64 {
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180

	y := math.Sin(dLng) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLng)
	deg := math.Atan2(y, x) * 180 / math.Pi
	return math.Mod(deg+360, 360)
}

// MoveToward returns a point meters along the straight path from → to and
// whether the destination was reached. Intended for synthetic scenario
// movement only; it makes no operational decisions.
func MoveToward(from, to Point, meters float64) (Point, bool) {
	dist := DistanceMeters(from, to)
	if dist == 0 || meters >= dist {
		return to, true
	}
	frac := meters / dist
	return Point{
		Lat: from.Lat + (to.Lat-from.Lat)*frac,
		Lng: from.Lng + (to.Lng-from.Lng)*frac,
	}, false
}

// PolygonAreaMeters2 approximates the area of a polygon ring via the shoelace
// formula projected to meters at the ring centroid latitude.
func PolygonAreaMeters2(ring []Point) float64 {
	if len(ring) < 3 {
		return 0
	}
	var latSum, lngSum float64
	for _, p := range ring {
		latSum += p.Lat
		lngSum += p.Lng
	}
	lat0 := latSum / float64(len(ring))
	mPerDegLat := 111320.0
	mPerDegLng := 111320.0 * math.Cos(lat0*math.Pi/180)

	var area float64
	for i := 0; i < len(ring); i++ {
		j := (i + 1) % len(ring)
		xi := ring[i].Lng * mPerDegLng
		yi := ring[i].Lat * mPerDegLat
		xj := ring[j].Lng * mPerDegLng
		yj := ring[j].Lat * mPerDegLat
		area += xi*yj - xj*yi
	}
	return math.Abs(area) / 2
}

// Centroid returns the arithmetic centroid of a ring (scenario helper).
func Centroid(ring []Point) Point {
	if len(ring) == 0 {
		return Point{}
	}
	var lat, lng float64
	for _, p := range ring {
		lat += p.Lat
		lng += p.Lng
	}
	n := float64(len(ring))
	return Point{Lat: lat / n, Lng: lng / n}
}
