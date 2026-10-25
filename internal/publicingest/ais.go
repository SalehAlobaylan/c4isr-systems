package publicingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/coder/websocket"
	"strings"
	"time"
)

const AISAdapter = "aisstream-position-v1"
const AISURL = "wss://stream.aisstream.io/v0/stream"
const AISReference = "https://aisstream.io/documentation"
const AISDataset = "aisstream-position-reports"

func DecodeAIS(raw []byte, in FetchRequest) (*Record, error) {
	var frame struct {
		MessageType string `json:"MessageType"`
		Meta        struct {
			MMSI     int64  `json:"MMSI"`
			ShipName string `json:"ShipName"`
			Time     string `json:"time_utc"`
		} `json:"MetaData"`
		Message map[string]json.RawMessage `json:"Message"`
	}
	fail := func() (*Record, error) { return nil, &RetrievalError{Code: "invalid_ais_frame"} }
	if len(raw) > 65536 || json.Unmarshal(raw, &frame) != nil {
		return fail()
	}
	if frame.MessageType != "PositionReport" && frame.MessageType != "StandardClassBPositionReport" && frame.MessageType != "ExtendedClassBPositionReport" {
		return nil, nil
	}
	var body struct {
		UserID   int64    `json:"UserID"`
		Valid    bool     `json:"Valid"`
		Lat      *float64 `json:"Latitude"`
		Lng      *float64 `json:"Longitude"`
		Speed    *float64 `json:"Sog"`
		Course   *float64 `json:"Cog"`
		Heading  *int     `json:"TrueHeading"`
		Accuracy bool     `json:"PositionAccuracy"`
		Status   *int     `json:"NavigationalStatus"`
	}
	if json.Unmarshal(frame.Message[frame.MessageType], &body) != nil || body.UserID != frame.Meta.MMSI {
		return fail()
	}
	if !body.Valid || body.Lat == nil || body.Lng == nil || *body.Lat == 91 || *body.Lng == 181 {
		return nil, nil
	}
	at, err := time.Parse(time.RFC3339Nano, frame.Meta.Time)
	if err != nil {
		at, err = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", frame.Meta.Time)
	}
	if err != nil {
		return fail()
	}
	point := geo.Point{Lng: *body.Lng, Lat: *body.Lat}
	if !point.Valid() {
		return fail()
	}
	// A bounded live capture can span beyond the query's starting timestamp.
	// Origin time comes from provider metadata, never the AIS UTC-second alone.
	if !within(in.Admission.Policy.AOI, point) || at.Before(in.Now.Add(-time.Duration(in.Admission.Policy.LookbackSeconds)*time.Second)) || at.After(time.Now().UTC().Add(5*time.Second)) {
		return nil, nil
	}
	speed := body.Speed
	if speed != nil {
		if *speed >= 102.3 {
			speed = nil
		} else {
			v := *speed * 1852 / 3600
			speed = &v
		}
	}
	course := body.Course
	if course != nil && *course >= 360 {
		course = nil
	}
	claims := map[string]any{"mmsi": fmt.Sprintf("%09d", body.UserID), "shipName": strings.TrimSpace(frame.Meta.ShipName), "navigationStatus": body.Status, "affiliation": "unassessed", "identityStatus": "self_reported_not_verified"}
	if body.Heading != nil && *body.Heading != 511 {
		claims["trueHeadingDegrees"] = *body.Heading
	}
	o := MovingObservation{Kind: "vessel", IdentityClaim: fmt.Sprintf("%09d", body.UserID), ObservedAt: at.UTC(), Position: point, SpeedMPS: speed, Course: course, Claims: claims, Quality: map[string]any{"positionAccuracyFlag": body.Accuracy, "messageType": frame.MessageType, "analystConfidence": nil, "upstreamLineage": "AISStream receiver aggregation; MMSI and names are unverified claims; no durable replay"}}
	if o.Validate(time.Now()) != nil {
		return fail()
	}
	return &Record{ProviderRecordID: fmt.Sprintf("mmsi:%09d:%s", body.UserID, at.UTC().Format(time.RFC3339Nano)), OriginalRecord: append(json.RawMessage(nil), raw...), ObservedAt: &o.ObservedAt, ReferenceURL: AISReference, Moving: &o}, nil
}
func (f *HTTPFetcher) fetchAIS(ctx context.Context, location Location, in FetchRequest) (Page, Retrieval, error) {
	meta := Retrieval{LocationID: location.ID, Discontinuous: true, CoverageMeaning: "Bounded live capture; reconnect gaps and upstream drops cannot be replayed."}
	captureStart := time.Now().UTC()
	p := Page{Records: []Record{}, CoverageFrom: captureStart, CoverageTo: captureStart.Add(time.Microsecond)}
	select {
	case f.aisSlots <- struct{}{}:
		defer func() { <-f.aisSlots }()
	case <-ctx.Done():
		return p, meta, ctx.Err()
	}
	conn, response, err := websocket.Dial(ctx, AISURL, &websocket.DialOptions{HTTPClient: f.client, CompressionMode: websocket.CompressionContextTakeover})
	if err != nil {
		return p, meta, &RetrievalError{Code: "ais_connection_unavailable"}
	}
	defer conn.CloseNow()
	conn.SetReadLimit(65536)
	if response == nil || !strings.Contains(response.Header.Get("Sec-WebSocket-Extensions"), "permessage-deflate") {
		return p, meta, &RetrievalError{Code: "ais_compression_required"}
	}
	if response != nil {
		meta.HTTPStatus = response.StatusCode
	}
	b := in.Admission.Policy.AOI
	sub, _ := json.Marshal(map[string]any{"APIKey": location.token, "BoundingBoxes": [][][]float64{{{b.South, b.West}, {b.North, b.East}}}, "FilterMessageTypes": []string{"PositionReport", "StandardClassBPositionReport", "ExtendedClassBPositionReport"}})
	write, cancel := context.WithTimeout(ctx, 2*time.Second)
	err = conn.Write(write, websocket.MessageText, sub)
	cancel()
	if err != nil {
		return p, meta, &RetrievalError{Code: "ais_subscription_unavailable"}
	}
	confirmed := false
	total := 0
	h := sha256.New()
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			if !confirmed {
				return p, meta, &RetrievalError{Code: "ais_subscription_not_confirmed"}
			}
			if !errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
				p.PartialReason = "ais_disconnected"
			}
			break
		}
		if location.token != "" && bytes.Contains(raw, []byte(location.token)) {
			return p, meta, &RetrievalError{Code: "ais_credential_echo_rejected"}
		}
		total += len(raw)
		if total > in.Admission.Policy.MaxResponseBytes {
			p.PartialReason = "stream_byte_budget"
			break
		}
		var control struct {
			Type    string `json:"MessageType"`
			Message struct {
				Compression bool `json:"CompressionEnabled"`
			} `json:"Message"`
			Error any `json:"error"`
		}
		if json.Unmarshal(raw, &control) != nil {
			return p, meta, &RetrievalError{Code: "invalid_ais_frame"}
		}
		if control.Type == "SubscriptionConfirmation" {
			if !control.Message.Compression {
				return p, meta, &RetrievalError{Code: "ais_compression_required"}
			}
			confirmed = true
			meta.CompressionEnabled = true
			continue
		}
		if control.Error != nil || control.Type == "Error" {
			return p, meta, &RetrievalError{Code: "ais_subscription_rejected"}
		}
		if !confirmed {
			return p, meta, &RetrievalError{Code: "ais_unconfirmed_data"}
		}
		record, err := DecodeAIS(raw, in)
		if err != nil {
			return p, meta, err
		}
		if record == nil {
			p.ExcludedRecords++
			continue
		}
		h.Write(raw)
		h.Write([]byte{10})
		p.Records = append(p.Records, *record)
		if len(p.Records) >= in.Admission.Policy.MaxRecordsPerPage {
			p.PartialReason = "stream_record_budget"
			break
		}
	}
	p.CoverageTo = time.Now().UTC()
	meta.ResponseHash = hex.EncodeToString(h.Sum(nil))
	meta.SelectionExcluded = p.ExcludedRecords
	return p, meta, nil
}
