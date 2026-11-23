package geoenv

import (
	"cmp"
	"slices"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type supportDelivery struct {
	Kind             string           `json:"kind"`
	SourceID         string           `json:"sourceId"`
	From             time.Time        `json:"from"`
	PauseFrom        time.Time        `json:"pauseFrom"`
	ResumeAt         time.Time        `json:"resumeAt"`
	SecondsPerReport int              `json:"secondsPerReport"`
	Messages         []supportMessage `json:"messages"`
}

type supportMessage struct {
	ID                      string    `json:"id"`
	Recipient               string    `json:"recipient"`
	SentAt                  time.Time `json:"sentAt"`
	AcknowledgeAfterSeconds *int      `json:"acknowledgeAfterSeconds"`
}

type supportReceipt struct {
	SupportMessageID string     `json:"id"`
	Recipient        string     `json:"recipient"`
	SentAt           time.Time  `json:"sentAt"`
	ReceivedAt       *time.Time `json:"receivedAt"`
	AcknowledgedAt   *time.Time `json:"acknowledgedAt"`
}

type supportQueue struct {
	Mode            string           `json:"mode"`
	Pending         int              `json:"pending"`
	PeakBacklog     int              `json:"peakBacklog"`
	Received        int              `json:"received"`
	P50LagSeconds   *float64         `json:"p50LagSeconds"`
	P90LagSeconds   *float64         `json:"p90LagSeconds"`
	RecoverySeconds *float64         `json:"recoverySeconds"`
	RecoveryStatus  string           `json:"recoveryStatus"`
	Receipts        []supportReceipt `json:"receipts"`
}

func (d supportDelivery) review(cutoff time.Time, sources map[string]bool) ([]supportQueue, error) {
	if d.Kind != "authored-exercise" || !sources[d.SourceID] || d.From.IsZero() || d.PauseFrom.Before(d.From) ||
		!d.ResumeAt.After(d.PauseFrom) || d.ResumeAt.Sub(d.From) > 48*time.Hour ||
		d.SecondsPerReport < 1 || d.SecondsPerReport > 3600 || len(d.Messages) == 0 || len(d.Messages) > 1000 {
		return nil, apperr.Validation("delivery comparison requires a bounded authored schedule")
	}
	seen := map[string]bool{}
	messages := []supportMessage{}
	for _, msg := range d.Messages {
		if !keyPattern.MatchString(msg.ID) || seen[msg.ID] || !keyPattern.MatchString(msg.Recipient) || msg.SentAt.Before(d.From) ||
			msg.SentAt.Sub(d.From) > 48*time.Hour || (msg.AcknowledgeAfterSeconds != nil && (*msg.AcknowledgeAfterSeconds < 0 || *msg.AcknowledgeAfterSeconds > 86400)) {
			return nil, apperr.Validation("invalid authored report, send time or acknowledgement delay")
		}
		seen[msg.ID] = true
		if !msg.SentAt.After(cutoff) {
			messages = append(messages, msg)
		}
	}
	slices.SortFunc(messages, func(a, b supportMessage) int {
		if order := a.SentAt.Compare(b.SentAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return []supportQueue{d.run("normal", cutoff, messages), d.run("authored-delay", cutoff, messages), d.run("fictional-dependency-removed", cutoff, messages)}, nil
}

func (d supportDelivery) run(mode string, cutoff time.Time, messages []supportMessage) supportQueue {
	type queuePoint struct {
		at    time.Time
		delta int
	}
	points := []queuePoint{}
	lag := []float64{}
	free := d.From
	var lastHeldDelivery time.Time
	out := supportQueue{Mode: mode, RecoveryStatus: "not-applicable", Receipts: []supportReceipt{}}
	for _, msg := range messages {
		start := msg.SentAt
		if free.After(start) {
			start = free
		}
		held := mode == "authored-delay" && !start.Before(d.PauseFrom) && start.Before(d.ResumeAt)
		if held {
			start = d.ResumeAt
		}
		free = start.Add(time.Duration(d.SecondsPerReport) * time.Second)
		if held {
			lastHeldDelivery = free
		}
		if !lastHeldDelivery.IsZero() && msg.SentAt.Before(d.ResumeAt) && free.After(lastHeldDelivery) {
			lastHeldDelivery = free
		}
		entry := supportReceipt{SupportMessageID: msg.ID, Recipient: msg.Recipient, SentAt: msg.SentAt}
		points = append(points, queuePoint{msg.SentAt, 1})
		if !free.After(cutoff) {
			received := free
			entry.ReceivedAt = &received
			points = append(points, queuePoint{received, -1})
			lag = append(lag, received.Sub(msg.SentAt).Seconds())
			if msg.AcknowledgeAfterSeconds != nil {
				ack := received.Add(time.Duration(*msg.AcknowledgeAfterSeconds) * time.Second)
				if !ack.After(cutoff) {
					entry.AcknowledgedAt = &ack
				}
			}
		}
		out.Receipts = append(out.Receipts, entry)
	}
	slices.SortFunc(points, func(a, b queuePoint) int {
		if order := a.at.Compare(b.at); order != 0 {
			return order
		}
		return cmp.Compare(a.delta, b.delta)
	})
	for _, point := range points {
		out.Pending += point.delta
		out.PeakBacklog = max(out.PeakBacklog, out.Pending)
	}
	out.Received = len(lag)
	if len(lag) > 0 {
		slices.Sort(lag)
		p50 := (lag[(len(lag)-1)/2] + lag[len(lag)/2]) / 2
		p90 := lag[(9*len(lag)+9)/10-1]
		out.P50LagSeconds, out.P90LagSeconds = &p50, &p90
	}
	if mode == "authored-delay" && !lastHeldDelivery.IsZero() {
		out.RecoveryStatus = "unresolved"
		if !lastHeldDelivery.After(cutoff) {
			seconds := lastHeldDelivery.Sub(d.ResumeAt).Seconds()
			out.RecoverySeconds, out.RecoveryStatus = &seconds, "drained"
		}
	}
	return out
}
