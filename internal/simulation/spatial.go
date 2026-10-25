// Package simulation defines renderer-independent simulation contracts.
package simulation

import (
	"fmt"
	"math"
)

const TransformVersion = "wgs84-ecef-enu-v1"
const semiMajor = 6378137.0
const flattening = 1 / 298.257223563
const eccentricity2 = flattening * (2 - flattening)

// Position uses explicit fields to avoid ambiguous latitude/longitude arrays.
// Height is optional evidence; a transform requires resolved ellipsoidal metres.
type Position struct {
	Longitude     float64  `json:"longitude" yaml:"longitude"`
	Latitude      float64  `json:"latitude" yaml:"latitude"`
	Height        *float64 `json:"height" yaml:"height"`
	VerticalDatum string   `json:"verticalDatum" yaml:"vertical_datum"`
}
type Vector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}
type Frame struct {
	CRS         string   `json:"crs" yaml:"crs"`
	Order       string   `json:"order" yaml:"order"`
	Axes        string   `json:"axes" yaml:"axes"`
	Units       string   `json:"units" yaml:"units"`
	Orientation string   `json:"orientation" yaml:"orientation"`
	Transform   string   `json:"transform" yaml:"transform"`
	Origin      Position `json:"origin" yaml:"origin"`
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (p Position) Validate() error {
	if !finite(p.Longitude) || !finite(p.Latitude) || math.Abs(p.Longitude) > 180 || math.Abs(p.Latitude) > 90 {
		return fmt.Errorf("invalid WGS84 longitude/latitude")
	}
	if p.Height == nil || !finite(*p.Height) || *p.Height < -12000 || *p.Height > 1000000 {
		return fmt.Errorf("resolved height in metres required")
	}
	if p.VerticalDatum != "WGS84_ELLIPSOID" {
		return fmt.Errorf("unsupported vertical conversion from %q; MSL/AGL require a versioned geoid/terrain model", p.VerticalDatum)
	}
	return nil
}
func (f Frame) Validate() error {
	if f.CRS != "EPSG:4326" || f.Order != "longitude_latitude" || f.Axes != "east_north_up" || f.Units != "metres" || f.Transform != TransformVersion || f.Orientation != "heading_clockwise_from_north_pitch_nose_up_roll_right_degrees" {
		return fmt.Errorf("unsupported scene coordinate/unit/orientation contract")
	}
	return f.Origin.Validate()
}
func DefaultFrame(origin Position) Frame {
	return Frame{"EPSG:4326", "longitude_latitude", "east_north_up", "metres", "heading_clockwise_from_north_pitch_nose_up_roll_right_degrees", TransformVersion, origin}
}
func ECEF(p Position) (Vector, error) {
	if err := p.Validate(); err != nil {
		return Vector{}, err
	}
	lat, lon := p.Latitude*math.Pi/180, p.Longitude*math.Pi/180
	n := semiMajor / math.Sqrt(1-eccentricity2*math.Pow(math.Sin(lat), 2))
	return Vector{(n + *p.Height) * math.Cos(lat) * math.Cos(lon), (n + *p.Height) * math.Cos(lat) * math.Sin(lon), (n*(1-eccentricity2) + *p.Height) * math.Sin(lat)}, nil
}
func (f Frame) ToLocal(p Position) (Vector, error) {
	if err := f.Validate(); err != nil {
		return Vector{}, err
	}
	a, err := ECEF(p)
	if err != nil {
		return Vector{}, err
	}
	o, _ := ECEF(f.Origin)
	x, y, z := a.X-o.X, a.Y-o.Y, a.Z-o.Z
	lat, lon := f.Origin.Latitude*math.Pi/180, f.Origin.Longitude*math.Pi/180
	v := Vector{-math.Sin(lon)*x + math.Cos(lon)*y, -math.Sin(lat)*math.Cos(lon)*x - math.Sin(lat)*math.Sin(lon)*y + math.Cos(lat)*z, math.Cos(lat)*math.Cos(lon)*x + math.Cos(lat)*math.Sin(lon)*y + math.Sin(lat)*z}
	if math.Sqrt(v.X*v.X+v.Y*v.Y+v.Z*v.Z) > 500000 {
		return Vector{}, fmt.Errorf("local scene radius exceeds 500 km")
	}
	return v, nil
}
func (f Frame) FromLocal(v Vector) (Position, error) {
	if err := f.Validate(); err != nil {
		return Position{}, err
	}
	if !finite(v.X) || !finite(v.Y) || !finite(v.Z) || math.Sqrt(v.X*v.X+v.Y*v.Y+v.Z*v.Z) > 500000 {
		return Position{}, fmt.Errorf("local scene radius exceeds 500 km")
	}
	o, _ := ECEF(f.Origin)
	lat, lon := f.Origin.Latitude*math.Pi/180, f.Origin.Longitude*math.Pi/180
	x := o.X - math.Sin(lon)*v.X - math.Sin(lat)*math.Cos(lon)*v.Y + math.Cos(lat)*math.Cos(lon)*v.Z
	y := o.Y + math.Cos(lon)*v.X - math.Sin(lat)*math.Sin(lon)*v.Y + math.Cos(lat)*math.Sin(lon)*v.Z
	z := o.Z + math.Cos(lat)*v.Y + math.Sin(lat)*v.Z
	r := math.Hypot(x, y)
	phi := math.Atan2(z, r*(1-eccentricity2))
	for i := 0; i < 15; i++ {
		n := semiMajor / math.Sqrt(1-eccentricity2*math.Pow(math.Sin(phi), 2))
		phi = math.Atan2(z+eccentricity2*n*math.Sin(phi), r)
	}
	n := semiMajor / math.Sqrt(1-eccentricity2*math.Pow(math.Sin(phi), 2))
	h := r*math.Cos(phi) + z*math.Sin(phi) - n*(1-eccentricity2*math.Pow(math.Sin(phi), 2))
	p := Position{math.Atan2(y, x) * 180 / math.Pi, phi * 180 / math.Pi, &h, "WGS84_ELLIPSOID"}
	return p, p.Validate()
}

// Fidelity describes a modeled component, never a claim of physical validation.
type Fidelity struct {
	Profile         string   `json:"profile" yaml:"profile"`
	ModelVersion    string   `json:"modelVersion" yaml:"model_version"`
	Simplifications []string `json:"simplifications" yaml:"simplifications"`
}

func (f Fidelity) Validate() error {
	if f.Profile != "visual_only" && f.Profile != "abstract_behavior" && f.Profile != "kinematic" {
		return fmt.Errorf("unsupported fidelity profile")
	}
	if f.ModelVersion == "" || len(f.ModelVersion) > 160 || len(f.Simplifications) < 1 || len(f.Simplifications) > 32 {
		return fmt.Errorf("fidelity version and simplifications required")
	}
	for _, s := range f.Simplifications {
		if s == "" || len(s) > 1000 {
			return fmt.Errorf("invalid fidelity simplification")
		}
	}
	return nil
}
