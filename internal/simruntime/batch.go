package simruntime

import (
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"math"
	"regexp"
)

var sampleID = regexp.MustCompile(`^sample_[a-f0-9]{32}$`)
var contactID = regexp.MustCompile(`^contact_[a-f0-9]{32}$`)

// The lease keys contact identities without making hidden actor IDs public.
// A sender cannot reuse one sample identity for another slot or recipient.
func ValidateBatchIdentity(b simulation.PerceptionBatch, token string) error {
	s := Sampler{secret: token}
	for _, r := range b.Readings {
		identity := r.Contact
		if r.Kind == "observation" && r.Contact != "contact_"+s.opaque(r.SensorID+"|"+r.ActorID) {
			return fmt.Errorf("contact identity does not match lease")
		}
		if r.ID != "sample_"+s.opaque(identity+fmt.Sprint(r.SampleMs)) {
			return fmt.Errorf("sample identity does not match lease")
		}
	}
	return nil
}

func ValidateBatch(f simscene.Frozen, b simulation.PerceptionBatch, elapsed int64) error {
	if f.Runtime != simscene.PerceptionRuntime && len(b.Readings)+len(b.Outcomes) > 0 {
		return fmt.Errorf("legacy runtime cannot publish native perception")
	}
	actors := map[string]bool{}
	controlled := map[string]simscene.Controlled{}
	sensors := map[string]simscene.Sensor{}
	for _, a := range f.Input.Actors {
		actors[a.ID] = true
	}
	for _, c := range f.Input.Controlled {
		controlled[c.ActorID] = c
	}
	for _, s := range f.Input.Sensors {
		sensors[s.ID] = s
	}
	seen := map[string]bool{}
	for _, r := range b.Readings {
		if !sampleID.MatchString(r.ID) || seen[r.ID] || !actors[r.ActorID] || r.SampleMs < 0 || r.DeliverMs < r.SampleMs || r.DeliverMs > elapsed || math.IsNaN(r.Heading) || math.IsInf(r.Heading, 0) || r.Heading < 0 || r.Heading >= 360 {
			return fmt.Errorf("invalid synthetic reading identity/time")
		}
		seen[r.ID] = true
		if _, e := f.Environment.Frame.FromLocal(r.Local); e != nil {
			return fmt.Errorf("invalid synthetic position")
		}
		switch r.Kind {
		case "observation":
			s, ok := sensors[r.SensorID]
			if !ok || !contactID.MatchString(r.Contact) || r.AssetID != "" || r.SampleMs%s.PeriodMs != 0 || (s.DropEvery > 0 && (r.SampleMs/s.PeriodMs+1)%int64(s.DropEvery) == 0) || r.DeliverMs != r.SampleMs+s.DelayMs || r.AccuracyM != math.Sqrt2*s.NoiseM || r.Confidence != s.Confidence {
				return fmt.Errorf("reading conflicts with frozen sensor")
			}
		case "telemetry":
			c, ok := controlled[r.ActorID]
			if !ok || r.AssetID == "" || r.Contact != r.AssetID || r.SensorID != "self-report" || r.SampleMs%c.TelemetryMs != 0 || r.DeliverMs != r.SampleMs || r.Confidence != 1 || r.AccuracyM != 0 {
				return fmt.Errorf("unbound controlled telemetry")
			}
		default:
			return fmt.Errorf("invalid perception kind")
		}
	}
	for _, o := range b.Outcomes {
		if o.CommandID == "" || o.ElapsedMs < 0 || o.ElapsedMs > elapsed {
			return fmt.Errorf("invalid task outcome")
		}
		switch o.State {
		case "ACKNOWLEDGED", "COMPLETED", "REJECTED", "FAILED":
		default:
			return fmt.Errorf("invalid task state")
		}
		if o.Code != "" && o.Code != "UNSUPPORTED_ACTION" && o.Code != "ACTOR_UNAVAILABLE" {
			return fmt.Errorf("invalid outcome code")
		}
	}
	return nil
}
