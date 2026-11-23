package geoenv

import (
	"cmp"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

var cemsActivationCode = regexp.MustCompile("^EMSR[0-9]{1,8}$")

type floodProduct struct {
	ProductID        int          `json:"productId"`
	Type             string       `json:"type"`
	AreaNumber       int          `json:"areaNumber"`
	AreaName         string       `json:"areaName"`
	VersionID        string       `json:"versionId"`
	VersionNumber    int          `json:"versionNumber"`
	DeliveredAt      time.Time    `json:"deliveredAt"`
	MonitoringNumber int          `json:"monitoringNumber"`
	SourceImages     []floodImage `json:"sourceImages"`
}

type floodImage struct {
	SourceImageID string     `json:"sourceImageId"`
	Sensor        string     `json:"sensor"`
	AcquisitionAt *time.Time `json:"acquisitionAt"`
}

type floodProducts struct {
	Activation      string         `json:"activation"`
	Cutoff          time.Time      `json:"cutoff"`
	VersionCoverage string         `json:"versionCoverage"`
	Products        []floodProduct `json:"products"`
}

func (s Service) reviewFloodProducts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	activation := query.Get("activation")
	cutoff, err := time.Parse(time.RFC3339Nano, query.Get("knownAt"))
	if !cemsActivationCode.MatchString(activation) || err != nil || cutoff.After(time.Now()) {
		httpx.Error(w, apperr.Validation("provide a public EMSR activation and a historical cutoff with a UTC offset"))
		return
	}
	s.reviewSimulationSource(w, r, "copernicus-ems-rapid-mapping", "emergency-response-v1", query.Get("inputSha256"),
		func(raw []byte) (string, any, error) {
			report, err := summarizeFloodProducts(raw, activation, cutoff)
			return "floodMapping", report, err
		})
}

// CEMS documents its provider times as UTC and publishes offset-free values.
func parseCEMSTime(value string) (time.Time, error) {
	at, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return at.UTC(), nil
	}
	return time.Parse("2006-01-02T15:04:05", value)
}

func summarizeFloodProducts(raw []byte, activation string, cutoff time.Time) (floodProducts, error) {
	out := floodProducts{Activation: activation, Cutoff: cutoff.UTC(), VersionCoverage: "versions-in-supplied-response",
		Products: []floodProduct{}}
	var page struct {
		Count   int     `json:"count"`
		Next    *string `json:"next"`
		Results []struct {
			Code      string `json:"code"`
			Category  string `json:"category"`
			Sensitive *bool  `json:"sensitive"`
			Areas     []struct {
				Number   int    `json:"number"`
				Name     string `json:"name"`
				Products []struct {
					ID               int    `json:"id"`
					Type             string `json:"type"`
					ActivationCode   string `json:"activationCode"`
					AreaNumber       int    `json:"aoiNumber"`
					MonitoringNumber int    `json:"monitoringNumber"`
					Version          struct {
						ID           string `json:"uuid"`
						Number       int    `json:"number"`
						Status       string `json:"statusCode"`
						DeliveryTime string `json:"deliveryTime"`
					} `json:"version"`
					Images []struct {
						ID          string `json:"uuid"`
						Sensor      string `json:"sensorName"`
						Acquisition string `json:"acquisitionTime"`
					} `json:"images"`
				} `json:"products"`
			} `json:"aois"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &page); err != nil || page.Count != 1 || page.Next != nil || len(page.Results) != 1 {
		return out, apperr.Validation("expected a complete response for one CEMS activation")
	}
	entry := page.Results[0]
	if entry.Code != activation || entry.Category != "Flood" || entry.Sensitive == nil || *entry.Sensitive {
		return out, apperr.Validation("expected the requested public flood activation")
	}
	seen := map[string]bool{}
	for _, area := range entry.Areas {
		for _, product := range area.Products {
			version := product.Version
			if (product.Type != "DEL" && product.Type != "GRA") || version.Status != "F" || version.DeliveryTime == "" {
				continue
			}
			delivered, err := parseCEMSTime(version.DeliveryTime)
			if err != nil {
				return out, apperr.Validation("CEMS product has an invalid delivery time")
			}
			if delivered.After(cutoff) {
				continue
			}
			if product.ID < 1 || area.Number < 1 || product.AreaNumber != area.Number || product.ActivationCode != activation ||
				strings.TrimSpace(area.Name) == "" || !keyPattern.MatchString(version.ID) || version.Number < 1 || seen[version.ID] {
				return out, apperr.Validation("CEMS product has an inconsistent identity, area or version")
			}
			seen[version.ID] = true
			summary := floodProduct{
				ProductID: product.ID, Type: product.Type, AreaNumber: area.Number, AreaName: area.Name,
				VersionID: version.ID, VersionNumber: version.Number, DeliveredAt: delivered,
				MonitoringNumber: product.MonitoringNumber, SourceImages: []floodImage{},
			}
			for _, image := range product.Images {
				if !keyPattern.MatchString(image.ID) {
					return out, apperr.Validation("CEMS product has an invalid image identity")
				}
				source := floodImage{SourceImageID: image.ID, Sensor: image.Sensor}
				if image.Acquisition != "" {
					acquired, err := parseCEMSTime(image.Acquisition)
					if err != nil || acquired.After(delivered) {
						return out, apperr.Validation("CEMS product has an inconsistent image acquisition time")
					}
					source.AcquisitionAt = &acquired
				}
				summary.SourceImages = append(summary.SourceImages, source)
			}
			out.Products = append(out.Products, summary)
		}
	}
	slices.SortFunc(out.Products, func(a, b floodProduct) int {
		if order := a.DeliveredAt.Compare(b.DeliveredAt); order != 0 {
			return order
		}
		return cmp.Compare(a.VersionID, b.VersionID)
	})
	return out, nil
}
