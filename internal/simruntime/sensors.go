package simruntime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sort"

	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
)

type Sampler struct {
	scene    simscene.Frozen
	world    *World
	seed     int64
	secret   string
	next     map[string]int64
	pending  []simulation.Reading
	bindings map[string]string
}

func NewSampler(scene simscene.Frozen, world *World, seed int64, secret string, bindings []simulation.ControlledBinding) *Sampler {
	s := &Sampler{scene: scene, world: world, seed: seed, secret: secret, next: map[string]int64{}, bindings: map[string]string{}}
	for _, b := range bindings {
		s.bindings[b.ActorID] = b.AssetID
	}
	return s
}
func (s *Sampler) opaque(text string) string {
	h := hmac.New(sha256.New, []byte(s.secret))
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))[:32]
}
func (s *Sampler) jitter(sensor, actor string, at int64, axis string, rangeM float64) float64 {
	h := sha256.Sum256([]byte(fmt.Sprintf("uniform-xy-v1|%d|%s|%s|%d|%s", s.seed, sensor, actor, at, axis)))
	u := float64(binary.LittleEndian.Uint64(h[:8])>>11) / 9007199254740992
	return (2*u - 1) * rangeM
}

// Advance is bounded by the frozen admission budget and executes only in the
// independent runtime. Missing observations never turn into truth-shaped marks.
func (s *Sampler) Advance(to int64) ([]simulation.Reading, error) {
	generated := 0
	for _, sensor := range s.scene.Input.Sensors {
		key := "sensor:" + sensor.ID
		for at := s.next[key]; at <= to; at += sensor.PeriodMs {
			s.next[key] = at + sensor.PeriodMs
			if sensor.DropEvery > 0 && ((at/sensor.PeriodMs)+1)%int64(sensor.DropEvery) == 0 {
				continue
			}
			actors, e := s.world.At(at)
			if e != nil {
				return nil, e
			}
			for _, a := range actors {
				if !a.Active {
					continue
				}
				contact := "contact_" + s.opaque(sensor.ID+"|"+a.ID)
				local := a.Local
				local.X += s.jitter(sensor.ID, a.ID, at, "x", sensor.NoiseM)
				local.Y += s.jitter(sensor.ID, a.ID, at, "y", sensor.NoiseM)
				s.pending = append(s.pending, simulation.Reading{ID: "sample_" + s.opaque(contact+fmt.Sprint(at)), ActorID: a.ID, Contact: contact, SensorID: sensor.ID, Kind: "observation", SampleMs: at, DeliverMs: at + sensor.DelayMs, Local: local, Heading: a.Heading, Confidence: sensor.Confidence, AccuracyM: math.Sqrt2 * sensor.NoiseM})
				generated++
			}
		}
	}
	for _, c := range s.scene.Input.Controlled {
		key := "telemetry:" + c.ActorID
		for at := s.next[key]; at <= to; at += c.TelemetryMs {
			s.next[key] = at + c.TelemetryMs
			actors, e := s.world.At(at)
			if e != nil {
				return nil, e
			}
			for _, a := range actors {
				if a.ID != c.ActorID || !a.Active {
					continue
				}
				asset := s.bindings[a.ID]
				if asset == "" {
					return nil, fmt.Errorf("controlled telemetry binding unavailable")
				}
				s.pending = append(s.pending, simulation.Reading{ID: "sample_" + s.opaque(asset+fmt.Sprint(at)), ActorID: a.ID, Contact: asset, AssetID: asset, SensorID: "self-report", Kind: "telemetry", SampleMs: at, DeliverMs: at, Local: a.Local, Heading: a.Heading, Confidence: 1})
				generated++
			}
		}
	}
	if generated > 512 || len(s.pending) > 1024 {
		return nil, fmt.Errorf("synthetic sample window budget exceeded")
	}
	sort.Slice(s.pending, func(i, j int) bool {
		a, b := s.pending[i], s.pending[j]
		if a.DeliverMs != b.DeliverMs {
			return a.DeliverMs < b.DeliverMs
		}
		return a.ID < b.ID
	})
	out := []simulation.Reading{}
	remaining := []simulation.Reading{}
	for _, r := range s.pending {
		if r.DeliverMs <= to {
			out = append(out, r)
		} else {
			remaining = append(remaining, r)
		}
	}
	s.pending = remaining
	return out, nil
}
