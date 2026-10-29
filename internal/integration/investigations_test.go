package integration

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/investigations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"testing"
	"time"
)

func TestMapFirstDurableInvestigationsMembershipHistoryAndConflicts(t *testing.T) {
	f := newPublicFixture(t)
	store := access.NewStore(f.h.pool)
	admin := access.Bootstrap(context.Background())
	if e := store.AddGrant(admin, "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"investigations.read", "investigations.create", "investigations.edit", "investigations.manage"}}); e != nil {
		t.Fatal(e)
	}
	if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO operators(id,name,role)VALUES('case-member-human','Case member','operator')`); e != nil {
		t.Fatal(e)
	}
	if e := store.CreateHuman(admin, "case-member", "case-member-human", access.Grant{Resource: f.resource, Permissions: []string{"operational.read", "sources.read", "investigations.read", "investigations.edit", "investigations.manage"}}); e != nil {
		t.Fatal(e)
	}
	_, member, e := store.IssueCredential(admin, "case-member", f.resource, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	input := investigations.Input{Title: "Synthetic durable question", Question: "What does the retained evidence establish?", Notes: "Authored note", State: "DRAFT", Scope: analystscope.Input{Area: areas.Selection{Kind: "drawn", Name: "Case local fixture", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[46,24],[47,24],[47,25],[46,25],[46,24]]]}`)}, Temporal: fixedPeriod()}, Evidence: []investigations.Evidence{}}
	call := func(token, method, path string, in any, want int) map[string]any {
		t.Helper()
		code, body := scopeRequest(t, f.h, token, method, path, in, nil)
		if code != want {
			t.Fatalf("case %s %s: %d want %d %v", method, path, code, want, body)
		}
		return body
	}
	created := call(f.manager, "POST", "/api/v1/investigations", input, 201)
	id := created["id"].(string)
	raw, _ := json.Marshal(created)
	var saved investigations.Case
	_ = json.Unmarshal(raw, &saved)
	if !saved.Revision.Scope.Area.Persistent || saved.OwnerHumanID != "operator-01" {
		t.Fatal("case scope/identity was not durable", saved)
	}
	call(member, "GET", "/api/v1/investigations/"+id, nil, 404)
	if call(member, "GET", "/api/v1/investigations", nil, 200)["total"] != float64(0) {
		t.Fatal("unshared title leaked")
	}
	call(f.manager, "POST", "/api/v1/investigations/"+id+"/members", investigations.Membership{ExpectedMembershipVersion: saved.MembershipVersion, ExpectedRevisionID: saved.Revision.ID, OwnerHumanID: saved.OwnerHumanID, Members: []string{saved.OwnerHumanID, "case-member-human"}}, 200)
	call(f.manager, "POST", "/api/v1/investigations/"+id+"/members", investigations.Membership{ExpectedMembershipVersion: saved.MembershipVersion, ExpectedRevisionID: saved.Revision.ID, OwnerHumanID: saved.OwnerHumanID, Members: []string{saved.OwnerHumanID}}, 409)
	call(member, "GET", "/api/v1/investigations/"+id, nil, 200)
	input.ExpectedRevisionID = saved.Revision.ID
	input.Notes = "Saved from another authorized browser identity"
	edited := call(member, "POST", "/api/v1/investigations/"+id+"/revisions", input, 201)
	revision := edited["revision"].(map[string]any)["id"].(string)
	call(f.manager, "POST", "/api/v1/investigations/"+id+"/revisions", input, 409)
	historical := call(member, "GET", "/api/v1/investigations/"+id+"?revision="+saved.Revision.ID, nil, 200)
	if historical["revision"].(map[string]any)["notes"] != "Authored note" {
		t.Fatal("old revision changed")
	}
	input.ExpectedRevisionID = revision
	input.State = "ARCHIVED"
	call(member, "POST", "/api/v1/investigations/"+id+"/revisions", input, 403)
	archived := call(f.manager, "POST", "/api/v1/investigations/"+id+"/revisions", input, 201)
	revision = archived["revision"].(map[string]any)["id"].(string)
	input.ExpectedRevisionID = revision
	input.State = "ACTIVE"
	reopened := call(f.manager, "POST", "/api/v1/investigations/"+id+"/revisions", input, 201)
	revision = reopened["revision"].(map[string]any)["id"].(string)
	// Add a source via an exact reference, then remove it. Retained prose/history
	// stays protected by the cumulative dependency even after removal.
	refRaw := []byte(`{"schema":"synthetic-exact-reference"}`)
	if _, e = f.h.pool.Exec(context.Background(), `INSERT INTO reference_datasets(id,scope_id,source_id,dataset_key,version,kind,document,artifact,spec_hash)VALUES('case-ref',$1,$2,'fixture','1','vector',$3,'bytes','hash')`, f.resource.ID(), f.source, refRaw); e != nil {
		t.Fatal(e)
	}
	input.ExpectedRevisionID = revision
	input.Evidence = []investigations.Evidence{{Kind: "reference", ID: "case-ref"}}
	attached := call(f.manager, "POST", "/api/v1/investigations/"+id+"/revisions", input, 201)
	revision = attached["revision"].(map[string]any)["id"].(string)
	input.ExpectedRevisionID = revision
	input.Evidence = []investigations.Evidence{}
	call(f.manager, "POST", "/api/v1/investigations/"+id+"/revisions", input, 201)
	if _, e = f.h.pool.Exec(context.Background(), `UPDATE principal_grants SET source_ids=ARRAY['unavailable-source'] WHERE principal_id='case-member'`); e != nil {
		t.Fatal(e)
	}
	call(member, "GET", "/api/v1/investigations/"+id, nil, 404)
	call(member, "GET", "/api/v1/investigations/"+id+"?revision="+saved.Revision.ID, nil, 404)
	if call(member, "GET", "/api/v1/investigations", nil, 200)["total"] != float64(0) {
		t.Fatal("restricted case title leaked")
	}
	if _, e = f.h.pool.Exec(context.Background(), `UPDATE investigation_revisions SET document='{}' WHERE case_id=$1`, id); e == nil {
		t.Fatal("case history was mutable")
	}
	call(f.worker, "GET", "/api/v1/investigations/"+id, nil, 403)
}
