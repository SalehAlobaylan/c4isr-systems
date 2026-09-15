package classifications

import "testing"

func floatPtr(v float64) *float64 { return &v }

func TestCreateInputNormalize(t *testing.T) {
	in := CreateInput{
		TrackID:         " trk_1 ",
		Label:           " hostile ",
		SourceReference: " ref-1 ",
		CreatedBy:       " op-1 ",
	}
	in.Normalize()

	if in.TrackID != "trk_1" {
		t.Errorf("trackId = %q, want trk_1", in.TrackID)
	}
	if in.Label != "hostile" {
		t.Errorf("label = %q, want hostile", in.Label)
	}
	if in.SourceReference != "ref-1" {
		t.Errorf("sourceReference = %q, want ref-1", in.SourceReference)
	}
	if in.CreatedBy != "op-1" {
		t.Errorf("createdBy = %q, want op-1", in.CreatedBy)
	}
	if in.Method != MethodOperator {
		t.Errorf("method = %q, want %q", in.Method, MethodOperator)
	}
}

func TestCreateInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateInput
		wantErr bool
	}{
		{
			name:  "valid",
			input: CreateInput{TrackID: "trk_1", Label: "hostile", Method: MethodRule},
		},
		{
			name:  "valid with confidence",
			input: CreateInput{TrackID: "trk_1", Label: "hostile", Confidence: floatPtr(0.42), Method: MethodAI},
		},
		{
			name:  "confidence bounds accepted",
			input: CreateInput{TrackID: "trk_1", Label: "hostile", Confidence: floatPtr(1), Method: MethodScenario},
		},
		{
			name:    "missing track id",
			input:   CreateInput{Label: "hostile", Method: MethodOperator},
			wantErr: true,
		},
		{
			name:    "missing label",
			input:   CreateInput{TrackID: "trk_1", Method: MethodOperator},
			wantErr: true,
		},
		{
			name:    "invalid method",
			input:   CreateInput{TrackID: "trk_1", Label: "hostile", Method: Method("MAGIC")},
			wantErr: true,
		},
		{
			name:    "confidence above range",
			input:   CreateInput{TrackID: "trk_1", Label: "hostile", Confidence: floatPtr(1.01), Method: MethodOperator},
			wantErr: true,
		},
		{
			name:    "confidence below range",
			input:   CreateInput{TrackID: "trk_1", Label: "hostile", Confidence: floatPtr(-0.01), Method: MethodOperator},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
