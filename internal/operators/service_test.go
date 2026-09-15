package operators

import (
	"context"
	"strings"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type fakeRepository struct {
	operators map[string]Operator
	createCue int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{operators: map[string]Operator{}}
}

func (f *fakeRepository) Create(_ context.Context, operator Operator) (Operator, error) {
	if _, ok := f.operators[operator.ID]; ok {
		return Operator{}, apperr.Conflict("operator already exists")
	}
	f.operators[operator.ID] = operator
	f.createCue++
	return operator, nil
}

func (f *fakeRepository) Get(_ context.Context, id string) (Operator, error) {
	operator, ok := f.operators[id]
	if !ok {
		return Operator{}, apperr.NotFound("operator", id)
	}
	return operator, nil
}

func (f *fakeRepository) List(context.Context, int, int) ([]Operator, int, error) {
	out := make([]Operator, 0, len(f.operators))
	for _, operator := range f.operators {
		out = append(out, operator)
	}
	return out, len(out), nil
}

func (f *fakeRepository) Exists(_ context.Context, id string) (bool, error) {
	_, ok := f.operators[id]
	return ok, nil
}

func TestCreateInputNormalize(t *testing.T) {
	in := CreateInput{Name: "  Ada  "}
	in.Normalize()
	if in.Name != "Ada" {
		t.Fatalf("Name = %q, want Ada", in.Name)
	}
	if in.Role != RoleOperator {
		t.Fatalf("Role = %q, want %q", in.Role, RoleOperator)
	}
}

func TestCreateInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateInput
		wantErr bool
	}{
		{name: "valid", in: CreateInput{Name: "Ada", Role: RoleSupervisor}},
		{name: "missing name", in: CreateInput{Role: RoleOperator}, wantErr: true},
		{name: "invalid role", in: CreateInput{Name: "Ada", Role: "root"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestServiceCreateGeneratesID(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo)

	operator, err := svc.Create(context.Background(), CreateInput{Name: "Ada"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !strings.HasPrefix(operator.ID, "opr_") {
		t.Fatalf("Create() id = %q, want opr_ prefix", operator.ID)
	}
	if operator.Role != RoleOperator {
		t.Fatalf("Create() role = %q, want %q", operator.Role, RoleOperator)
	}
}

func TestServiceEnsureIsIdempotent(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo)
	in := CreateInput{ID: "opr_default", Name: "Default Operator"}

	created, isNew, err := svc.Ensure(context.Background(), in)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !isNew {
		t.Fatal("Ensure() isNew = false, want true on first call")
	}
	if created.ID != "opr_default" {
		t.Fatalf("Ensure() id = %q, want opr_default", created.ID)
	}

	existing, isNew, err := svc.Ensure(context.Background(), in)
	if err != nil {
		t.Fatalf("Ensure() second call error = %v", err)
	}
	if isNew {
		t.Fatal("Ensure() isNew = true, want false on second call")
	}
	if existing.ID != created.ID {
		t.Fatalf("Ensure() id = %q, want %q", existing.ID, created.ID)
	}
	if repo.createCue != 1 {
		t.Fatalf("repo.Create() calls = %d, want 1", repo.createCue)
	}
}

func TestServiceListClamps(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo)

	if _, _, err := svc.List(context.Background(), -1, -1); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got := clampLimit(-1); got != DefaultLimit {
		t.Fatalf("clampLimit(-1) = %d, want %d", got, DefaultLimit)
	}
	if got := clampLimit(MaxLimit + 1); got != MaxLimit {
		t.Fatalf("clampLimit(max+1) = %d, want %d", got, MaxLimit)
	}
}

var _ Repository = (*fakeRepository)(nil)
