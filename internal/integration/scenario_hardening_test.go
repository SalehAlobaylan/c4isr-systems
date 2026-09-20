package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestConcurrentScenarioRunsRouteCommandsToTheirOwningEngine(t *testing.T) {
	h := newHarness(t)
	start := func() map[string]any {
		var run map[string]any
		if status := h.do(http.MethodPost, "/api/v1/scenarios/restricted-area-intrusion/start", map[string]any{"speed": 20}, &run); status != http.StatusCreated {
			t.Fatalf("start scenario: status %d (%v)", status, run)
		}
		return run
	}
	first := start()
	second := start()
	firstID := first["id"].(string)
	secondID := second["id"].(string)
	firstNamespace := first["resourceNamespace"].(string)
	secondNamespace := second["resourceNamespace"].(string)
	if firstNamespace == secondNamespace {
		t.Fatalf("concurrent runs reused namespace %q", firstNamespace)
	}

	issue := func(namespace string) string {
		var command map[string]any
		status := h.do(http.MethodPost, "/api/v1/commands", map[string]any{
			"assetId": namespace + "asset__patrol-01",
			"type":    "move_to",
			"payload": map[string]any{"lat": 24.7155, "lng": 46.6785},
		}, &command)
		if status != http.StatusCreated {
			t.Fatalf("issue command for namespace %s: status %d (%v)", namespace, status, command)
		}
		return command["id"].(string)
	}
	firstCommand := issue(firstNamespace)
	secondCommand := issue(secondNamespace)

	waitFor(t, 20*time.Second, 100*time.Millisecond, "first command completion", func() bool {
		return commandState(h, firstCommand) == "COMPLETED"
	})
	waitFor(t, 20*time.Second, 100*time.Millisecond, "second command completion", func() bool {
		return commandState(h, secondCommand) == "COMPLETED"
	})
	for _, item := range []struct {
		runID     string
		assetID   string
		command   string
		namespace string
	}{
		{runID: firstID, assetID: firstNamespace + "asset__patrol-01", command: firstCommand, namespace: firstNamespace},
		{runID: secondID, assetID: secondNamespace + "asset__patrol-01", command: secondCommand, namespace: secondNamespace},
	} {
		var command map[string]any
		if status := h.do(http.MethodGet, "/api/v1/commands/"+item.command, nil, &command); status != http.StatusOK {
			t.Fatalf("get command %s: status %d", item.command, status)
		}
		if command["assetId"] != item.assetID || command["state"] != "COMPLETED" {
			t.Fatalf("command %s was routed to the wrong run: %v", item.command, command)
		}
		waitForRunCompletion(t, h, item.runID)
		assertTerminalScenarioEvents(t, h, item.runID)
		assertScenarioResourcesAreIsolated(t, h, item.runID, item.namespace)
	}
}

func TestOperationalFailureMatrixPersistsFailureAndRevisionEvidence(t *testing.T) {
	h := newHarness(t)
	var run map[string]any
	if status := h.do(http.MethodPost, "/api/v1/scenarios/operational-failure-matrix/start", map[string]any{"speed": 50}, &run); status != http.StatusCreated {
		t.Fatalf("start failure matrix: status %d (%v)", status, run)
	}
	runID := run["id"].(string)
	namespace := run["resourceNamespace"].(string)
	waitForRunCompletion(t, h, runID)

	var eventList struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if status := h.do(http.MethodGet, "/api/v1/scenarios/runs/"+runID+"/events", nil, &eventList); status != http.StatusOK {
		t.Fatalf("inspect failure matrix events: status %d", status)
	}
	if eventList.Total == 0 || len(eventList.Items) != eventList.Total {
		t.Fatalf("failure matrix events = %#v, want durable event rows", eventList)
	}
	hasFailedObservation := false
	hasCompletedRejection := false
	for _, event := range eventList.Items {
		name, _ := event["name"].(string)
		status, _ := event["status"].(string)
		if strings.HasPrefix(name, "observe.") && status == "failed" {
			hasFailedObservation = true
		}
		if name == "command.reject" && status == "completed" {
			hasCompletedRejection = true
		}
		if status == "pending" || status == "running" {
			t.Fatalf("terminal failure matrix event remained active: %v", event)
		}
	}
	if !hasFailedObservation || !hasCompletedRejection {
		t.Fatalf("failure matrix events missing failed observation or command rejection: %v", eventList.Items)
	}

	var runState map[string]any
	if status := h.do(http.MethodGet, "/api/v1/scenarios/runs/"+runID, nil, &runState); status != http.StatusOK {
		t.Fatalf("get failure matrix run: status %d", status)
	}
	if runState["eventsRun"].(float64) > runState["eventsTotal"].(float64) {
		t.Fatalf("eventsRun exceeds eventsTotal: %v", runState)
	}

	var assessments struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	assessmentPath := "/api/v1/assessments?subject_type=track&subject_id=" + url.QueryEscape(namespace+"track__ambiguous-01")
	if status := h.do(http.MethodGet, assessmentPath, nil, &assessments); status != http.StatusOK {
		t.Fatalf("list scenario assessments: status %d", status)
	}
	if assessments.Total != 2 {
		t.Fatalf("assessment revisions = %d, want 2", assessments.Total)
	}
	if assessments.Items[0]["conclusion"] == assessments.Items[1]["conclusion"] {
		t.Fatalf("assessment revisions did not preserve distinct conclusions: %v", assessments.Items)
	}
	for _, item := range assessments.Items {
		if item["createdBy"] != "scenario:"+runID {
			t.Fatalf("assessment actor = %v, want scenario run actor", item["createdBy"])
		}
	}

	var commands struct {
		Items []map[string]any `json:"items"`
	}
	if status := h.do(http.MethodGet, "/api/v1/commands?limit=1000", nil, &commands); status != http.StatusOK {
		t.Fatalf("list scenario commands: status %d", status)
	}
	commandRejected := false
	for _, command := range commands.Items {
		payload, _ := command["payload"].(map[string]any)
		if payload["scenarioRunId"] == runID {
			if command["state"] != "REJECTED" {
				t.Fatalf("scenario command state = %v, want REJECTED", command["state"])
			}
			commandRejected = true
		}
	}
	if !commandRejected {
		t.Fatal("failure matrix command was not retained with a rejected terminal state")
	}

	var audit struct {
		Items []map[string]any `json:"items"`
	}
	if status := h.do(http.MethodGet, "/api/v1/audit?limit=1000", nil, &audit); status != http.StatusOK {
		t.Fatalf("list scenario audit: status %d", status)
	}
	incidentCreated := false
	incidentUpdated := false
	for _, entry := range audit.Items {
		data, _ := entry["data"].(map[string]any)
		if data["scenarioRunId"] != runID {
			continue
		}
		switch entry["action"] {
		case "incident.created":
			incidentCreated = true
		case "incident.updated":
			incidentUpdated = true
		}
	}
	if !incidentCreated || !incidentUpdated {
		t.Fatalf("scenario incident revision evidence missing: created=%v updated=%v", incidentCreated, incidentUpdated)
	}
}

func commandState(h *harness, commandID string) string {
	var command map[string]any
	if status := h.do(http.MethodGet, "/api/v1/commands/"+commandID, nil, &command); status != http.StatusOK {
		return ""
	}
	state, _ := command["state"].(string)
	return state
}

func waitForRunCompletion(t *testing.T, h *harness, runID string) {
	t.Helper()
	waitFor(t, 30*time.Second, 100*time.Millisecond, "scenario completion "+runID, func() bool {
		var run map[string]any
		if status := h.do(http.MethodGet, "/api/v1/scenarios/runs/"+runID, nil, &run); status != http.StatusOK {
			return false
		}
		return run["status"] == "COMPLETED"
	})
}

func assertTerminalScenarioEvents(t *testing.T, h *harness, runID string) {
	t.Helper()
	var eventList struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if status := h.do(http.MethodGet, "/api/v1/scenarios/runs/"+runID+"/events", nil, &eventList); status != http.StatusOK {
		t.Fatalf("inspect terminal events for %s: status %d", runID, status)
	}
	if eventList.Total == 0 || len(eventList.Items) != eventList.Total {
		t.Fatalf("terminal events for %s = %#v", runID, eventList)
	}
	for _, event := range eventList.Items {
		if event["status"] == "pending" || event["status"] == "running" {
			t.Fatalf("terminal event for %s remained active: %v", runID, event)
		}
	}
	var run map[string]any
	if status := h.do(http.MethodGet, "/api/v1/scenarios/runs/"+runID, nil, &run); status != http.StatusOK {
		t.Fatalf("get terminal run %s: status %d", runID, status)
	}
	if run["eventsRun"].(float64) > run["eventsTotal"].(float64) {
		t.Fatalf("terminal run %s has eventsRun > eventsTotal: %v", runID, run)
	}
}
