package geoenv

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

const aerialReviewSchema = "emad.civilian-aerial-review.v1"

type aerialExercise struct {
	Schema       string            `json:"schema"`
	Title        string            `json:"title"`
	Kind         string            `json:"kind"`
	AreaRevision string            `json:"areaRevision"`
	IncidentAt   time.Time         `json:"incidentAt"`
	MinUsable    float64           `json:"minUsableFraction"`
	MinWidthM    float64           `json:"reviewWidthM"`
	Sources      []exerciseSource  `json:"sources"`
	Scenes       []aerialScene     `json:"scenes"`
	Assessments  []aerialProduct   `json:"assessments"`
	Briefing     []handoffCue      `json:"briefing"`
	Traffic      []civilTrafficDay `json:"traffic"`
	Notices      []airportNotice   `json:"notices"`
	Survey       *surveyExercise   `json:"survey,omitempty"`
	Energy       []aerialEnergy    `json:"energy,omitempty"`
}

type aerialScene struct {
	ID              string        `json:"id"`
	SourceID        string        `json:"sourceId"`
	SensorFamily    string        `json:"sensorFamily"`
	AcquiredAt      time.Time     `json:"acquiredAt"`
	AvailableAt     *time.Time    `json:"availableAt"`
	TimeBasis       string        `json:"timeBasis"`
	AreaRevision    string        `json:"areaRevision"`
	Grid            string        `json:"grid"`
	Bounds          []float64     `json:"bounds,omitempty"`
	CatalogCoverage string        `json:"catalogCoverage,omitempty"`
	PixelM          float64       `json:"pixelM"`
	UsableFraction  *float64      `json:"usableFraction"`
	CoveredFraction *float64      `json:"coveredFraction"`
	Assets          []aerialAsset `json:"assets"`
}

type aerialAsset struct {
	Role        string     `json:"role"`
	SHA256      string     `json:"sha256"`
	PNG         string     `json:"png"`
	AvailableAt *time.Time `json:"availableAt,omitempty"`
}

type aerialProduct struct {
	ID               string      `json:"id"`
	SourceID         string      `json:"sourceId"`
	Label            string      `json:"label"`
	Kind             string      `json:"kind"`
	Version          int         `json:"version"`
	AvailableAt      time.Time   `json:"availableAt"`
	SceneRefs        []string    `json:"sceneRefs"`
	SourceImageTimes []time.Time `json:"sourceImageTimes,omitempty"`
	Coverage         string      `json:"coverage"`
	Affected         *bool       `json:"affected"`
	Classes          []string    `json:"classes,omitempty"`
	AssessedAreaWkt  string      `json:"assessedAreaWkt,omitempty"`
}

type handoffCue struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"sourceId"`
	AvailableAt time.Time `json:"availableAt"`
	Briefing    string    `json:"briefing"`
}

type civilTrafficDay struct {
	Day          string     `json:"day"`
	SourceID     string     `json:"sourceId"`
	AvailableAt  time.Time  `json:"availableAt"`
	CivilCount   *int       `json:"civilCount"`
	Unclassified *int       `json:"unclassified"`
	UsableHours  int        `json:"usableHours"`
	OldestAt     *time.Time `json:"oldestAt"`
}

type airportNotice struct {
	ID          string     `json:"id"`
	SourceID    string     `json:"sourceId"`
	DeliveryID  string     `json:"deliveryId"`
	Revision    int        `json:"revision"`
	Kind        string     `json:"kind"`
	Publisher   string     `json:"publisher"`
	Title       string     `json:"title"`
	PublishedAt time.Time  `json:"publishedAt"`
	AvailableAt time.Time  `json:"availableAt"`
	EffectiveAt time.Time  `json:"effectiveAt"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	Status      string     `json:"status"`
}

type surveyExercise struct {
	RequiredCells []string     `json:"requiredCells"`
	Assignments   []surveyTask `json:"assignments"`
}
type surveyTask struct {
	ID          string    `json:"id"`
	AvailableAt time.Time `json:"availableAt"`
	Cells       []string  `json:"cells"`
	Status      string    `json:"status"`
}

type surveyReview struct {
	Assignments int `json:"assignments"`
	Aborted     int `json:"aborted"`
	Required    int `json:"requiredCells"`
	Covered     int `json:"coveredCells"`
	Duplicate   int `json:"duplicateVisits"`
}

type aerialReview struct {
	Title        string             `json:"title"`
	Kind         string             `json:"kind"`
	Cutoff       time.Time          `json:"cutoff"`
	IncidentAt   time.Time          `json:"incidentAt"`
	AreaRevision string             `json:"areaRevision"`
	ImageStatus  string             `json:"imageStatus"`
	Candidates   []sceneReview      `json:"candidates"`
	Before       *aerialScene       `json:"before"`
	After        *aerialScene       `json:"after"`
	Sources      []exerciseSource   `json:"sources"`
	Assessments  []assessmentReview `json:"assessments"`
	Briefing     []handoffCue       `json:"briefing"`
	Traffic      []civilTrafficDay  `json:"traffic"`
	Notices      []noticeReview     `json:"notices"`
	Survey       *surveyReview      `json:"survey"`
	Energy       []aerialEnergy     `json:"energy"`
}

type sceneReview struct {
	ID              string   `json:"id"`
	State           string   `json:"state"`
	UsableFraction  *float64 `json:"usableFraction"`
	CoveredFraction *float64 `json:"coveredFraction"`
	CatalogCoverage string   `json:"catalogCoverage,omitempty"`
}

type assessmentReview struct {
	aerialProduct
	SharesScene bool `json:"sharesScene"`
}

type noticeReview struct {
	airportNotice
	DeliveryIDs []string `json:"deliveryIds"`
}

func (s Service) reviewCivilianAerial(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	cutoff, err := time.Parse(time.RFC3339Nano, query.Get("knownAt"))
	if err != nil {
		httpx.Error(w, apperr.Validation("provide an evidence cutoff with a UTC offset"))
		return
	}
	s.reviewSimulationSource(w, r, "civilian-aerial-review", "v1", query.Get("inputSha256"),
		func(raw []byte) (string, any, error) {
			data, err := reviewAerialExercise(raw, cutoff)
			return "review", data, err
		})
}

func reviewAerialExercise(raw []byte, cutoff time.Time) (aerialReview, error) {
	var e aerialExercise
	var out aerialReview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&e); err != nil || decoder.Decode(new(any)) != io.EOF {
		return out, apperr.Validation("expected one civilian aerial exercise package")
	}
	if e.Schema != aerialReviewSchema || (e.Kind != "authored-exercise" && e.Kind != "reviewed-history") ||
		strings.TrimSpace(e.Title) == "" || !keyPattern.MatchString(e.AreaRevision) || e.IncidentAt.IsZero() ||
		e.MinUsable <= 0 || e.MinUsable > 1 || e.MinWidthM <= 0 || e.MinWidthM > 10000 ||
		len(e.Sources) == 0 || len(e.Sources) > 32 || len(e.Scenes) > 40 || len(e.Assessments) > 250 ||
		len(e.Briefing) > 250 || len(e.Traffic) > 180 || len(e.Notices) > 250 {
		return out, apperr.Validation("invalid aerial exercise, review threshold or input budget")
	}
	sources := map[string]bool{}
	for _, s := range e.Sources {
		if !keyPattern.MatchString(s.ID) || sources[s.ID] || !digestPattern.MatchString(s.SHA256) ||
			strings.TrimSpace(s.Attribution) == "" || strings.TrimSpace(s.Edition) == "" {
			return out, apperr.Validation("missing or duplicate source identity, edition, attribution or checksum")
		}
		sources[s.ID] = true
	}
	out = aerialReview{Title: e.Title, Kind: e.Kind, Cutoff: cutoff.UTC(), IncidentAt: e.IncidentAt,
		AreaRevision: e.AreaRevision, Sources: e.Sources, ImageStatus: "needs-another-observation",
		Candidates: []sceneReview{}, Assessments: []assessmentReview{}, Briefing: []handoffCue{},
		Traffic: []civilTrafficDay{}, Notices: []noticeReview{}, Energy: []aerialEnergy{}}
	if err := e.reviewScenes(cutoff, sources, &out); err != nil {
		return out, err
	}
	if err := e.reviewContext(cutoff, sources, &out); err != nil {
		return out, err
	}
	return out, e.reviewSurvey(cutoff, &out)
}

// ReadAerialReview reconstructs an already issued civilian imagery exercise
// with the same reader used by Atlas. It checks the independent package pin.
func ReadAerialReview(raw []byte, cutoff time.Time, inputDigest string) (any, error) {
	if len(raw) == 0 || len(raw) > simulationReviewMaxBytes || !digestPattern.MatchString(inputDigest) ||
		artifacts.Hash(raw) != inputDigest {
		return nil, apperr.Validation("aerial package differs from its trusted digest or exceeds the input budget")
	}
	result, err := reviewAerialExercise(raw, cutoff)
	if err != nil {
		return nil, err
	}
	return map[string]any{"schema": "emad.civilian-aerial-review-result.v1", "inputSha256": inputDigest, "review": result}, nil
}

func assetSizes(assets []aerialAsset) (bool, error) {
	sizes := map[string][2]int{}
	for _, asset := range assets {
		if (asset.Role != "visual" && asset.Role != "quality-mask" && asset.Role != "flood-overlay") || sizes[asset.Role] != [2]int{} ||
			!digestPattern.MatchString(asset.SHA256) || len(asset.PNG) > 700000 {
			return false, apperr.Validation("invalid review image or mask identity")
		}
		raw, err := base64.StdEncoding.Strict().DecodeString(asset.PNG)
		if err != nil || artifacts.Hash(raw) != asset.SHA256 {
			return false, apperr.Validation("aerial preview checksum differs")
		}
		shape, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil || shape.Width < 1 || shape.Height < 1 || shape.Width > 768 || shape.Height > 768 {
			return false, apperr.Validation("expected a bounded PNG preview")
		}
		if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
			return false, apperr.Validation("aerial preview has corrupt pixels")
		}
		sizes[asset.Role] = [2]int{shape.Width, shape.Height}
	}
	visual, mask, flood := sizes["visual"], sizes["quality-mask"], sizes["flood-overlay"]
	if visual == [2]int{} || mask == [2]int{} {
		if flood != [2]int{} {
			return false, apperr.Validation("flood overlay requires its paired visual and quality mask")
		}
		return false, nil
	}
	if visual != mask || (flood != [2]int{} && flood != visual) {
		return false, apperr.Validation("preview and quality mask pixels are not aligned")
	}
	return true, nil
}

func (e aerialExercise) reviewScenes(cutoff time.Time, sources map[string]bool, out *aerialReview) error {
	seen := map[string]bool{}
	candidates := []aerialScene{}
	for _, scene := range e.Scenes {
		if !keyPattern.MatchString(scene.ID) || seen[scene.ID] || !sources[scene.SourceID] ||
			strings.TrimSpace(scene.SensorFamily) == "" || strings.TrimSpace(scene.Grid) == "" ||
			!keyPattern.MatchString(scene.AreaRevision) || scene.AcquiredAt.IsZero() || scene.PixelM <= 0 ||
			(scene.AvailableAt != nil && (scene.AcquiredAt.After(*scene.AvailableAt) || strings.TrimSpace(scene.TimeBasis) == "")) {
			return apperr.Validation("invalid scene identity, source, grid or dates")
		}
		if b := scene.Bounds; len(b) != 0 && (len(b) != 4 || b[0] < -180 || b[2] > 180 ||
			b[1] < -85 || b[3] > 85 || b[0] >= b[2] || b[1] >= b[3] || b[2]-b[0] > .1 || b[3]-b[1] > .1) {
			return apperr.Validation("aerial preview has invalid bounded WGS84 extent")
		}
		if scene.CatalogCoverage != "" && scene.CatalogCoverage != "partial-page" && scene.CatalogCoverage != "supplied-catalog" {
			return apperr.Validation("unrecognized imagery catalog coverage")
		}
		seen[scene.ID] = true
		if scene.AvailableAt == nil || scene.AvailableAt.After(cutoff) {
			continue
		}
		visibleAssets := make([]aerialAsset, 0, len(scene.Assets))
		for _, asset := range scene.Assets {
			if asset.Role == "flood-overlay" && (asset.AvailableAt == nil || asset.AvailableAt.Before(*scene.AvailableAt)) {
				return apperr.Validation("flood overlay requires a release time on or after the scene release")
			}
			if asset.AvailableAt == nil || !asset.AvailableAt.After(cutoff) {
				visibleAssets = append(visibleAssets, asset)
			}
		}
		scene.Assets = visibleAssets
		if scene.UsableFraction != nil && (*scene.UsableFraction < 0 || *scene.UsableFraction > 1) ||
			scene.CoveredFraction != nil && (*scene.CoveredFraction < 0 || *scene.CoveredFraction > 1) ||
			scene.UsableFraction != nil && scene.CoveredFraction != nil && *scene.UsableFraction > *scene.CoveredFraction {
			return apperr.Validation("local image coverage must be between zero and one")
		}
		pairedAssets, err := assetSizes(scene.Assets)
		if err != nil {
			return err
		}
		state := "usable"
		switch {
		case scene.AreaRevision != e.AreaRevision:
			state = "review-area-mismatch"
		case !pairedAssets:
			state = "assets-unavailable"
		case e.MinWidthM < 2*scene.PixelM:
			state = "insufficient-resolution"
		case scene.UsableFraction == nil:
			state = "quality-unknown"
		case *scene.UsableFraction < e.MinUsable:
			state = "usable-area-below-threshold"
		default:
			candidates = append(candidates, scene)
		}
		out.Candidates = append(out.Candidates, sceneReview{scene.ID, state, scene.UsableFraction, scene.CoveredFraction, scene.CatalogCoverage})
	}
	slices.SortFunc(candidates, func(a, b aerialScene) int {
		if order := a.AcquiredAt.Compare(b.AcquiredAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	for i, after := range candidates {
		if after.AcquiredAt.Before(e.IncidentAt) {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			before := candidates[j]
			if before.AcquiredAt.Before(e.IncidentAt) && before.SensorFamily == after.SensorFamily && before.Grid == after.Grid {
				out.Before, out.After, out.ImageStatus = &before, &after, "paired"
				return nil
			}
		}
	}
	return nil
}
