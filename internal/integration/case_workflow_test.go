package integration

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/investigations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
)

// Fault injection uses an isolated database and private in-memory object store.
// These credentials represent protocol fixtures, not observed human review.
type caseBlobStore struct {
	mu         sync.Mutex
	data       map[string][]byte
	afterPut   func()
	afterGet   func()
	failDelete bool
}

func (s *caseBlobStore) Put(_ context.Context, k string, b []byte, _ string) error {
	s.mu.Lock()
	s.data[k] = append([]byte{}, b...)
	f := s.afterPut
	s.mu.Unlock()
	if f != nil {
		f()
	}
	return nil
}
func (s *caseBlobStore) Get(_ context.Context, k string, _ int64) ([]byte, error) {
	s.mu.Lock()
	b := append([]byte{}, s.data[k]...)
	f := s.afterGet
	s.mu.Unlock()
	if f != nil {
		f()
	}
	return b, nil
}
func (s *caseBlobStore) Delete(_ context.Context, k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failDelete {
		return errors.New("injected delete failure")
	}
	delete(s.data, k)
	return nil
}
func caseInput() investigations.Input {
	return investigations.Input{RequestKey: "initial-save", Title: "Case workflow fixture", Question: "What does the retained evidence establish?", State: "DRAFT", Notes: "Original authored note", Scope: analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Case fixture", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`)}, Temporal: fixedPeriod()}, Evidence: []investigations.Evidence{}, Findings: []investigations.FindingInput{}}
}
func caseCall(t *testing.T, f *publicFixture, token, method, path string, in any, want int) map[string]any {
	t.Helper()
	code, body := scopeRequest(t, f.h, token, method, path, in, nil)
	if code != want {
		t.Fatalf("%s %s: %d want %d: %v", method, path, code, want, body)
	}
	return body
}
func caseDecode(t *testing.T, v map[string]any) investigations.Case {
	t.Helper()
	raw, _ := json.Marshal(v)
	var c investigations.Case
	if e := json.Unmarshal(raw, &c); e != nil {
		t.Fatal(e)
	}
	return c
}
func grantCase(t *testing.T, f *publicFixture) {
	t.Helper()
	if e := access.NewStore(f.h.pool).AddGrant(access.Bootstrap(context.Background()), "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"investigations.read", "investigations.create", "investigations.edit", "investigations.manage", "investigations.review", "outputs.read", "outputs.create", "outputs.distribute"}}); e != nil {
		t.Fatal(e)
	}
}
func caseReviewer(t *testing.T, f *publicFixture) string {
	t.Helper()
	ctx := context.Background()
	if _, e := f.h.pool.Exec(ctx, `INSERT INTO operators(id,name,role) VALUES('independent-case-human','Reviewer fixture','operator')`); e != nil {
		t.Fatal(e)
	}
	s := access.NewStore(f.h.pool)
	a := access.Bootstrap(ctx)
	if e := s.CreateHuman(a, "independent-case-reviewer", "independent-case-human", access.Grant{Resource: f.resource, Permissions: []string{"sources.read", "observations.read", "investigations.read", "investigations.review", "investigations.edit", "outputs.read", "outputs.create", "outputs.distribute"}}); e != nil {
		t.Fatal(e)
	}
	_, token, e := s.IssueCredential(a, "independent-case-reviewer", f.resource, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	return token
}
func shareCase(t *testing.T, f *publicFixture, c investigations.Case) {
	t.Helper()
	caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/members", investigations.Membership{ExpectedMembershipVersion: c.MembershipVersion, ExpectedRevisionID: c.Revision.ID, OwnerHumanID: c.OwnerHumanID, Members: []string{c.OwnerHumanID, "independent-case-human"}}, 200)
}

func TestMapFirstCaseSaveIntentsIndependentReviewAndRework(t *testing.T) {
	f := newPublicFixture(t)
	grantCase(t, f)
	reviewer := caseReviewer(t, f)
	in := caseInput()
	c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", in, 201))
	shareCase(t, f, c)
	next := in
	next.RequestKey = "second-save"
	next.ExpectedRevisionID = c.Revision.ID
	next.Notes = "A concurrent editor saved the next revision"
	head := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/revisions", next, 201))
	retry := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", in, 201))
	if retry.ID != c.ID || retry.Revision.ID != c.Revision.ID {
		t.Fatal("lost response retry did not return the original result")
	}
	aliases := access.NewStore(f.h.pool)
	admin := access.Bootstrap(context.Background())
	if e := aliases.CreateHuman(admin, "case-author-alias", "operator-01", access.Grant{Resource: f.resource, Permissions: []string{"sources.read", "investigations.read", "investigations.create", "investigations.review"}}); e != nil {
		t.Fatal(e)
	}
	_, alias, e := aliases.IssueCredential(admin, "case-author-alias", f.resource, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	aliasRetry := caseDecode(t, caseCall(t, f, alias, "POST", "/api/v1/investigations", in, 201))
	if aliasRetry.Revision.ID != c.Revision.ID {
		t.Fatal("canonical alias lost original save identity")
	}
	in.Notes = "changed intent payload"
	caseCall(t, f, f.manager, "POST", "/api/v1/investigations", in, 409)
	caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/revisions", next, 201) // same intent, no third revision
	request := investigations.ReviewInput{Action: "REVIEW_REQUESTED", ExpectedRevisionID: head.Revision.ID, ReviewerHumanID: "operator-01", Reason: "Review the frozen current case revision"}
	path := "/api/v1/investigations/" + c.ID + "/review"
	caseCall(t, f, f.manager, "POST", path, request, 403)
	request.ReviewerHumanID = "independent-case-human"
	event := caseCall(t, f, f.manager, "POST", path, request, 201)
	inbox := caseCall(t, f, reviewer, "GET", "/api/v1/investigations/review-inbox", nil, 200)
	if len(inbox["items"].([]any)) != 1 {
		t.Fatal("assigned review missing from inbox", inbox)
	}
	decision := investigations.ReviewInput{Action: "CHANGES_REQUESTED", ExpectedRevisionID: head.Revision.ID, ExpectedEventID: event["id"].(string), Reason: "Please clarify the uncertainty before approval"}
	changes := caseCall(t, f, reviewer, "POST", path, decision, 201)
	caseCall(t, f, reviewer, "POST", path, decision, 409)
	next.RequestKey = "rework-save"
	next.ExpectedRevisionID = head.Revision.ID
	next.Notes = "Clarified uncertainty after independent review"
	head = caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/revisions", next, 201))
	request.ExpectedRevisionID = head.Revision.ID
	request.ExpectedEventID = changes["id"].(string)
	event = caseCall(t, f, f.manager, "POST", path, request, 201)
	decision.Action = "APPROVED"
	decision.ExpectedRevisionID = head.Revision.ID
	decision.ExpectedEventID = event["id"].(string)
	approved := caseCall(t, f, reviewer, "POST", path, decision, 201)
	if approved["contentDigest"] != event["contentDigest"] {
		t.Fatal("approval changed exact reviewed content")
	}
	// If the reviewer edits, they become a contributor and cannot review the successor.
	next.RequestKey = "reviewer-edit"
	next.ExpectedRevisionID = head.Revision.ID
	next.Notes = "Reviewer is now also a content contributor"
	head = caseDecode(t, caseCall(t, f, reviewer, "POST", "/api/v1/investigations/"+c.ID+"/revisions", next, 201))
	request.ExpectedRevisionID = head.Revision.ID
	request.ExpectedEventID = approved["id"].(string)
	caseCall(t, f, f.manager, "POST", path, request, 403)
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE investigation_review_events SET document='{}' WHERE case_id=$1`, c.ID); e == nil {
		t.Fatal("append-only review history changed")
	}
	caseCall(t, f, f.worker, "POST", path, request, 403)
}

func TestMapFirstCompositeFrozenRenderAndIndependentAttestation(t *testing.T) {
	store := &caseBlobStore{data: map[string][]byte{}}
	f := newPublicFixture(t, app.WithArtifactStore(store))
	grantCase(t, f)
	reviewer := caseReviewer(t, f)
	c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", caseInput(), 201))
	shareCase(t, f, c)
	in := investigations.OutputInput{RequestKey: "output-1", ExpectedRevisionID: c.Revision.ID, Purpose: "distribution", RendererVersion: "fixture-json-v1", StyleVersion: "fixture-style-v1", RetainUntil: time.Now().Add(time.Hour), ExpectedBytes: 4096}
	frozen := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/outputs", in, 201)
	id := frozen["id"].(string)
	path := "/api/v1/outputs/" + id
	again := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/outputs", in, 201)
	if again["id"] != id {
		t.Fatal("output retry changed identity")
	}
	rendered := caseCall(t, f, f.manager, "PUT", path+"/content", map[string]string{"fixture": "frozen JSON rendering"}, 200)
	if rendered["renderHumanId"] != "operator-01" || rendered["contentDigest"] != frozen["contentDigest"] {
		t.Fatal("render identity lost", rendered)
	}
	caseCall(t, f, f.manager, "PUT", path+"/content", map[string]string{"fixture": "changed rendering"}, 409)
	caseCall(t, f, f.manager, "GET", path+"/content?distribution=true", nil, 409)
	request := investigations.ReviewInput{Action: "REVIEW_REQUESTED", ExpectedRevisionID: c.Revision.ID, ReviewerHumanID: "independent-case-human", OutputID: id, Reason: "Review this exact frozen manifest and rendered identity"}
	event := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/review", request, 201)
	decision := investigations.ReviewInput{Action: "APPROVED", ExpectedRevisionID: c.Revision.ID, ExpectedEventID: event["id"].(string), Reason: "Verified exact manifest, content and rendered identity"}
	approved := caseCall(t, f, reviewer, "POST", "/api/v1/investigations/"+c.ID+"/review", decision, 201)
	if approved["renderSha256"] != rendered["renderSha256"] {
		t.Fatal("approval not bound to actual rendered bytes")
	}
	caseCall(t, f, f.manager, "GET", path+"/content?distribution=true", nil, 200)
	if _, e := f.h.pool.Exec(context.Background(), `UPDATE composite_outputs SET state='FROZEN' WHERE id=$1`, id); e == nil {
		t.Fatal("ready output could be reopened")
	}
	store.mu.Lock()
	for k := range store.data {
		store.data[k] = []byte(`{"corrupt":true}`)
	}
	store.mu.Unlock()
	caseCall(t, f, f.manager, "GET", path+"/content", nil, 409)
}

func TestMapFirstCompositeGrantRevokedDuringStorage(t *testing.T) {
	store := &caseBlobStore{data: map[string][]byte{}}
	f := newPublicFixture(t, app.WithArtifactStore(store))
	grantCase(t, f)
	c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", caseInput(), 201))
	in := investigations.OutputInput{RequestKey: "io-revocation", ExpectedRevisionID: c.Revision.ID, Purpose: "internal", RendererVersion: "fixture-json-v1", StyleVersion: "v1", RetainUntil: time.Now().Add(time.Hour), ExpectedBytes: 4096}
	out := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/outputs", in, 201)
	id := out["id"].(string)
	store.afterPut = func() {
		if _, e := f.h.pool.Exec(context.Background(), `UPDATE principals SET disabled_at=now() WHERE id='source-reviewer'`); e != nil {
			t.Error(e)
		}
	}
	caseCall(t, f, f.manager, "PUT", "/api/v1/outputs/"+id+"/content", map[string]string{"fixture": "must not become ready"}, 401)
	var state string
	if e := f.h.pool.QueryRow(context.Background(), `SELECT state FROM composite_outputs WHERE id=$1`, id).Scan(&state); e != nil {
		t.Fatal(e)
	}
	if state != "FROZEN" {
		t.Fatal("revoked grant committed rendered bytes", state)
	}
}

func TestMapFirstFindingScopeHistoryAndCurrentOutputRights(t *testing.T) {
	f := newPublicFixture(t)
	grantCase(t, f)
	reviewer := caseReviewer(t, f)
	ctx := context.Background()
	rights := map[string]any{"retainRaw": true, "allowAnalysis": true, "allowSimulationExport": true, "outputPermissions": map[string]bool{"internalAnalysis": true, "citation": true, "includedImagery": true, "rawDistribution": true, "geographicEnvironment": true, "metadataRetention": true}}
	doc, _ := json.Marshal(map[string]any{"schema": "fixture-reference", "approvedAt": time.Now().UTC(), "rights": rights})
	if _, e := f.h.pool.Exec(ctx, `INSERT INTO reference_datasets(id,scope_id,source_id,dataset_key,version,kind,document,artifact,spec_hash)VALUES('finding-reference',$1,$2,'finding-fixture','1','vector',$3,'fixture','hash');`, f.resource.ID(), f.source, doc); e != nil {
		t.Fatal(e)
	}
	if _, e := f.h.pool.Exec(ctx, `INSERT INTO content_purpose_revisions(id,scope_id,entity_kind,entity_id,source_id,version,purpose,reason,receipt_reference,actor_id)VALUES('finding-purpose',$1,'reference','finding-reference',$2,1,'working','Owned synthetic evidence for a workflow test','test-only','operator-01')`, f.resource.ID(), f.source); e != nil {
		t.Fatal(e)
	}
	in := caseInput()
	in.Evidence = []investigations.Evidence{{Kind: "reference", ID: "finding-reference"}}
	in.Findings = []investigations.FindingInput{{Title: "Authored finding", Conclusion: "Fixture conclusion supported by an exact reference", Confidence: "Limited synthetic fixture", Uncertainty: "No real-world inference", Purpose: "working", Evidence: in.Evidence}}
	c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", in, 201))
	shareCase(t, f, c)
	if len(c.Revision.Findings) != 1 || c.Revision.Findings[0].Evidence[0].SHA256 == "" || len(c.Notices) != 0 {
		t.Fatal("finding lost exact identity or support", c.Notices)
	}
	original := c.Revision.Findings[0]
	out := investigations.OutputInput{RequestKey: "rights-output", ExpectedRevisionID: c.Revision.ID, Purpose: "internal", SourceUses: map[string]string{f.source: "reference"}, RendererVersion: "fixture-v1", StyleVersion: "v1", RetainUntil: time.Now().Add(time.Hour), ExpectedBytes: 1024}
	frozen := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/outputs", out, 201)
	deps := frozen["manifest"].(map[string]any)["dependencies"].([]any)
	if len(deps) != 1 {
		t.Fatal("missing frozen rights lineage", frozen)
	}
	// A scope change creates a case successor while preserving the finding's scope.
	in.RequestKey = "scope-successor"
	in.ExpectedRevisionID = c.Revision.ID
	in.Findings[0] = original.FindingInput
	in.Findings[0].ExpectedRevisionID = original.RevisionID
	in.Scope.Area.Geometry = json.RawMessage(`{"type":"Polygon","coordinates":[[[46.1,24.1],[46.9,24.1],[46.9,24.9],[46.1,24.9],[46.1,24.1]]]}`)
	c = caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/revisions", in, 201))
	if c.Revision.Findings[0].RevisionID != original.RevisionID {
		t.Fatal("scope change relabeled an unchanged finding")
	}
	found := false
	for _, n := range c.Notices {
		found = found || n.State == "different_scope"
	}
	if !found {
		t.Fatal("scope incompatibility was hidden")
	}
	// A genuinely edited conclusion becomes a new immutable finding revision.
	in.RequestKey = "finding-successor"
	in.ExpectedRevisionID = c.Revision.ID
	in.Findings[0].Conclusion = "Reassessed conclusion for the new frozen area"
	c = caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/revisions", in, 201))
	if c.Revision.Findings[0].PreviousID != original.RevisionID {
		t.Fatal("finding revision chain missing")
	}
	request := investigations.ReviewInput{Action: "REVIEW_REQUESTED", ExpectedRevisionID: c.Revision.ID, ReviewerHumanID: "independent-case-human", Reason: "Review exact source-backed finding revision"}
	pending := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/review", request, 201)
	if _, e := f.h.pool.Exec(ctx, `UPDATE sources SET status='offline' WHERE id=$1`, f.source); e != nil {
		t.Fatal(e)
	}
	decision := investigations.ReviewInput{Action: "APPROVED", ExpectedRevisionID: c.Revision.ID, ExpectedEventID: pending["id"].(string), Reason: "Support must still be current before approval"}
	caseCall(t, f, reviewer, "POST", "/api/v1/investigations/"+c.ID+"/review", decision, 409)
	decision.Action = "CHANGES_REQUESTED"
	caseCall(t, f, reviewer, "POST", "/api/v1/investigations/"+c.ID+"/review", decision, 201)
	if _, e := f.h.pool.Exec(ctx, `UPDATE sources SET status='active' WHERE id=$1`, f.source); e != nil {
		t.Fatal(e)
	}
	// Removing citations never removes historical source dependencies.
	in.RequestKey = "remove-current-citations"
	in.ExpectedRevisionID = c.Revision.ID
	in.Evidence = []investigations.Evidence{}
	in.Findings = []investigations.FindingInput{}
	c = caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/revisions", in, 201))
	out.ExpectedRevisionID = c.Revision.ID
	out.RequestKey = "history-rights-output"
	caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/outputs", out, 201)
	rights["outputPermissions"] = map[string]bool{"internalAnalysis": false, "citation": true, "metadataRetention": true}
	doc, _ = json.Marshal(map[string]any{"approvedAt": time.Now().UTC().Add(time.Second), "rights": rights})
	if _, e := f.h.pool.Exec(ctx, `INSERT INTO reference_datasets(id,scope_id,source_id,dataset_key,version,kind,document,artifact,spec_hash)VALUES('finding-reference-v2',$1,$2,'finding-fixture','2','vector',$3,'fixture','hash')`, f.resource.ID(), f.source, doc); e != nil {
		t.Fatal(e)
	}
	out.RequestKey = "denied-current-rights"
	caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/outputs", out, 403)
	if _, e := f.h.pool.Exec(ctx, `UPDATE principal_grants SET source_ids=ARRAY['another-source'] WHERE principal_id='independent-case-reviewer'`); e != nil {
		t.Fatal(e)
	}
	caseCall(t, f, reviewer, "GET", "/api/v1/investigations/"+c.ID, nil, 404)
	if _, e := f.h.pool.Exec(ctx, `UPDATE investigation_finding_revisions SET document='{}' WHERE case_id=$1`, c.ID); e == nil {
		t.Fatal("finding history mutable")
	}
}

func TestMapFirstCompositeExpiryDeletionRetryAndTombstone(t *testing.T) {
	store := &caseBlobStore{data: map[string][]byte{}, failDelete: true}
	f := newPublicFixture(t, app.WithArtifactStore(store))
	grantCase(t, f)
	c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", caseInput(), 201))
	in := investigations.OutputInput{RequestKey: "short-retention", ExpectedRevisionID: c.Revision.ID, Purpose: "internal", RendererVersion: "fixture-v1", StyleVersion: "v1", RetainUntil: time.Now().Add(2 * time.Second), ExpectedBytes: 1024}
	out := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/outputs", in, 201)
	id := out["id"].(string)
	path := "/api/v1/outputs/" + id
	caseCall(t, f, f.manager, "PUT", path+"/content", map[string]string{"fixture": "expires"}, 200)
	caseCall(t, f, f.manager, "POST", path+"/expire", nil, 409)
	time.Sleep(time.Until(in.RetainUntil) + 10*time.Millisecond)
	caseCall(t, f, f.manager, "GET", path+"/content", nil, 409)
	caseCall(t, f, f.manager, "POST", path+"/expire", nil, 409)
	pending := caseCall(t, f, f.manager, "GET", path, nil, 200)
	if pending["state"] != "DELETE_PENDING" {
		t.Fatal("failed deletion was not retryable", pending)
	}
	store.mu.Lock()
	store.failDelete = false
	store.mu.Unlock()
	caseCall(t, f, f.manager, "POST", path+"/expire", nil, 200)
	caseCall(t, f, f.manager, "POST", path+"/expire", nil, 200)
	var raw []byte
	if e := f.h.pool.QueryRow(context.Background(), `SELECT document FROM composite_outputs WHERE id=$1`, id).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	if len(doc) != 3 || doc["tombstone"] != true {
		t.Fatal("tombstone retained full case content", string(raw))
	}
	store.mu.Lock()
	remaining := len(store.data)
	store.mu.Unlock()
	if remaining != 0 {
		t.Fatal("expired object retained after successful delete")
	}
	caseCall(t, f, f.manager, "PUT", path+"/content", map[string]string{"fixture": "expires"}, 409)
}

func TestMapFirstCompositePrivateS3RoundTrip(t *testing.T) {
	store := newS3Fixture(t)
	f := newPublicFixture(t, app.WithArtifactStore(store))
	grantCase(t, f)
	c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", caseInput(), 201))
	in := investigations.OutputInput{RequestKey: "private-s3", ExpectedRevisionID: c.Revision.ID, Purpose: "internal", RendererVersion: "fixture-v1", StyleVersion: "v1", RetainUntil: time.Now().Add(time.Hour), ExpectedBytes: 1024}
	out := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/outputs", in, 201)
	path := "/api/v1/outputs/" + out["id"].(string)
	code, body := scopeRequest(t, f.h, f.manager, "PUT", path+"/content", map[string]string{"fixture": "active content is refused"}, map[string]string{"Content-Type": "text/html"})
	if code != 400 {
		t.Fatal("active HTML accepted", code, body)
	}
	ready := caseCall(t, f, f.manager, "PUT", path+"/content", map[string]string{"fixture": "private S3 bytes"}, 200)
	if _, leaked := ready["objectKey"]; leaked {
		t.Fatal("private storage key exposed")
	}
	read := caseCall(t, f, f.manager, "GET", path+"/content", nil, 200)
	if read["fixture"] != "private S3 bytes" {
		t.Fatal("private bytes changed", read)
	}
	caseCall(t, f, f.manager, "GET", path+"/content?distribution=true", nil, 409)
	// A small claimed render size cannot bypass the retained manifest budget.
	if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO resource_capacity_reservations(id,scope_id,source_id,kind,estimated_bytes,lease_until) VALUES('manifest-budget',$1,$2,'fixture',$3,now()+interval '1 hour')`, f.resource.ID(), f.source, int64(8<<30)-1500); e != nil {
		t.Fatal(e)
	}
	in.RequestKey = "manifest-budget-check"
	in.ExpectedBytes = 1
	caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/outputs", in, 409)
}
