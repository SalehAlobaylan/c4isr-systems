package assessments

import (
	"strings"
	"testing"
)

func validInput() CreateInput {
	confidence := 0.75
	return CreateInput{
		SubjectType: SubjectTrack,
		SubjectID:   "trk_1",
		Type:        "threat",
		Conclusion:  "likely hostile",
		Confidence:  &confidence,
		Method:      MethodAlgorithm,
		CreatedBy:   "opr_1",
		Evidence: []Evidence{
			{Type: EvidenceTrack, ID: "trk_1"},
			{Type: EvidenceClassification, ID: "cls_1"},
		},
	}
}

func TestCreateInputValidate(t *testing.T) {
	low := -0.1
	high := 1.1
	zero := 0.0
	one := 1.0

	tests := []struct {
		name    string
		mutate  func(*CreateInput)
		wantErr bool
	}{
		{name: "valid", mutate: func(*CreateInput) {}},
		{name: "confidence zero allowed", mutate: func(in *CreateInput) { in.Confidence = &zero }},
		{name: "confidence one allowed", mutate: func(in *CreateInput) { in.Confidence = &one }},
		{name: "confidence nil allowed", mutate: func(in *CreateInput) { in.Confidence = nil }},
		{name: "missing subject type", mutate: func(in *CreateInput) { in.SubjectType = "" }, wantErr: true},
		{name: "invalid subject type", mutate: func(in *CreateInput) { in.SubjectType = "planet" }, wantErr: true},
		{name: "missing subject id", mutate: func(in *CreateInput) { in.SubjectID = "" }, wantErr: true},
		{name: "missing type", mutate: func(in *CreateInput) { in.Type = "" }, wantErr: true},
		{name: "missing conclusion", mutate: func(in *CreateInput) { in.Conclusion = "" }, wantErr: true},
		{name: "invalid method", mutate: func(in *CreateInput) { in.Method = "VIBES" }, wantErr: true},
		{name: "confidence below zero", mutate: func(in *CreateInput) { in.Confidence = &low }, wantErr: true},
		{name: "confidence above one", mutate: func(in *CreateInput) { in.Confidence = &high }, wantErr: true},
		{name: "invalid evidence type", mutate: func(in *CreateInput) { in.Evidence = []Evidence{{Type: "rumor", ID: "x"}} }, wantErr: true},
		{name: "empty evidence id", mutate: func(in *CreateInput) { in.Evidence = []Evidence{{Type: EvidenceTrack, ID: ""}} }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			tt.mutate(&in)
			in.Normalize()
			err := in.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestCreateInputNormalize(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreateInput)
		check  func(*testing.T, CreateInput)
	}{
		{
			name:   "method defaults to operator",
			mutate: func(in *CreateInput) { in.Method = "" },
			check: func(t *testing.T, in CreateInput) {
				if in.Method != MethodOperator {
					t.Fatalf("Method = %q, want %q", in.Method, MethodOperator)
				}
			},
		},
		{
			name: "fields are trimmed",
			mutate: func(in *CreateInput) {
				in.SubjectID = " trk_1 "
				in.Type = " threat "
				in.Conclusion = " hostile "
				in.CreatedBy = " opr_1 "
				in.Evidence = []Evidence{{Type: EvidenceTrack, ID: " ev_1 "}}
			},
			check: func(t *testing.T, in CreateInput) {
				if in.SubjectID != "trk_1" || in.Type != "threat" || in.Conclusion != "hostile" || in.CreatedBy != "opr_1" {
					t.Fatalf("unexpected trimmed values: %+v", in)
				}
				if in.Evidence[0].ID != "ev_1" {
					t.Fatalf("evidence id = %q, want ev_1", in.Evidence[0].ID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			tt.mutate(&in)
			in.Normalize()
			tt.check(t, in)
			if err := in.Validate(); err != nil {
				t.Fatalf("Validate() after Normalize() = %v, want nil", err)
			}
		})
	}
}

func TestEvidenceTypeValidate(t *testing.T) {
	valid := []EvidenceType{
		EvidenceObservation, EvidenceTrack, EvidenceAsset, EvidenceIncident,
		EvidenceSource, EvidenceClassification, EvidenceAlert,
	}
	for _, typ := range valid {
		if err := typ.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", typ, err)
		}
	}
	if err := EvidenceType("rumor").Validate(); err == nil {
		t.Error("Validate(rumor) = nil, want error")
	}
}

func TestClampLimit(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{in: 0, want: DefaultLimit},
		{in: -5, want: DefaultLimit},
		{in: 25, want: 25},
		{in: MaxLimit + 1, want: MaxLimit},
	}
	for _, tt := range tests {
		if got := clampLimit(tt.in); got != tt.want {
			t.Errorf("clampLimit(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestServiceCreate(t *testing.T) {
	repo := &fakeRepository{}
	bus, published := testBus()
	svc := NewService(repo, bus)

	created, err := svc.Create(t.Context(), validInput())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !strings.HasPrefix(created.ID, "asm_") {
		t.Fatalf("Create() id = %q, want asm_ prefix", created.ID)
	}
	if len(repo.created) != 1 {
		t.Fatalf("repo.Create() calls = %d, want 1", len(repo.created))
	}
	if repo.created[0].Evidence[0].AddedAt.IsZero() {
		t.Fatal("evidence addedAt was not defaulted")
	}
	if len(*published) != 1 {
		t.Fatalf("published events = %d, want 1", len(*published))
	}
	if (*published)[0].AssessmentID != created.ID {
		t.Fatalf("event assessment id = %q, want %q", (*published)[0].AssessmentID, created.ID)
	}
	if (*published)[0].SubjectID != "trk_1" || (*published)[0].Method != string(MethodAlgorithm) {
		t.Fatalf("unexpected event: %+v", (*published)[0])
	}
}

func TestServiceCreateRejectsInvalidInput(t *testing.T) {
	repo := &fakeRepository{}
	bus, _ := testBus()
	svc := NewService(repo, bus)

	in := validInput()
	in.SubjectType = "planet"
	if _, err := svc.Create(t.Context(), in); err == nil {
		t.Fatal("Create() = nil error, want validation error")
	}
	if len(repo.created) != 0 {
		t.Fatalf("repo.Create() calls = %d, want 0", len(repo.created))
	}
}

func TestServiceListClamps(t *testing.T) {
	repo := &fakeRepository{}
	bus, _ := testBus()
	svc := NewService(repo, bus)

	if _, _, err := svc.List(t.Context(), "track", "trk_1", -1, -1); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.listLimit != DefaultLimit || repo.listOffset != 0 {
		t.Fatalf("List() limit/offset = %d/%d, want %d/0", repo.listLimit, repo.listOffset, DefaultLimit)
	}
	if repo.listSubjectType != "track" || repo.listSubjectID != "trk_1" {
		t.Fatalf("List() subject = %q/%q, want track/trk_1", repo.listSubjectType, repo.listSubjectID)
	}
}

func TestServiceListFiltersAreTrimmed(t *testing.T) {
	repo := &fakeRepository{}
	bus, _ := testBus()
	svc := NewService(repo, bus)

	if _, _, err := svc.List(t.Context(), "  track ", "  trk_1  ", 10, 20); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.listSubjectType != "track" || repo.listSubjectID != "trk_1" {
		t.Fatalf("List() subject = %q/%q, want track/trk_1", repo.listSubjectType, repo.listSubjectID)
	}
	if repo.listLimit != 10 || repo.listOffset != 20 {
		t.Fatalf("List() limit/offset = %d/%d, want 10/20", repo.listLimit, repo.listOffset)
	}
}
