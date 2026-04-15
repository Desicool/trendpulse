package calculator_test

import (
	"context"
	"testing"

	"trendpulse/internal/calculator"
	"trendpulse/internal/domain"
)

// stubStrategy is a minimal Strategy implementation for registry tests.
type stubStrategy struct {
	id   string
	name string
}

func (s *stubStrategy) ID() string   { return s.id }
func (s *stubStrategy) Name() string { return s.name }
func (s *stubStrategy) Calculate(_ context.Context, _ *domain.Trend, _ calculator.SignalReader) (*domain.TrendStats, error) {
	return nil, nil
}

func TestRegistry_Register_Success(t *testing.T) {
	r := calculator.NewRegistry()
	s := &stubStrategy{id: "alpha_v1", name: "Alpha"}
	if err := r.Register(s); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}
}

func TestRegistry_Register_DuplicateID_ReturnsError(t *testing.T) {
	r := calculator.NewRegistry()
	s := &stubStrategy{id: "alpha_v1", name: "Alpha"}
	if err := r.Register(s); err != nil {
		t.Fatalf("first Register() unexpected error: %v", err)
	}
	if err := r.Register(s); err == nil {
		t.Fatal("second Register() expected error, got nil")
	}
}

func TestRegistry_Get_ExistingID_ReturnsStrategy(t *testing.T) {
	r := calculator.NewRegistry()
	s := &stubStrategy{id: "beta_v1", name: "Beta"}
	_ = r.Register(s)

	got, ok := r.Get("beta_v1")
	if !ok {
		t.Fatal("Get() returned ok=false, want true")
	}
	if got.ID() != "beta_v1" {
		t.Errorf("Get() ID = %q, want %q", got.ID(), "beta_v1")
	}
}

func TestRegistry_Get_MissingID_ReturnsFalse(t *testing.T) {
	r := calculator.NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("Get() returned ok=true for missing ID, want false")
	}
}

func TestRegistry_All_ReturnsAllStrategies(t *testing.T) {
	r := calculator.NewRegistry()
	_ = r.Register(&stubStrategy{id: "a"})
	_ = r.Register(&stubStrategy{id: "b"})
	_ = r.Register(&stubStrategy{id: "c"})

	all := r.All()
	if len(all) != 3 {
		t.Errorf("All() returned %d strategies, want 3", len(all))
	}
}

func TestRegistry_All_EmptyRegistry_ReturnsEmptySlice(t *testing.T) {
	r := calculator.NewRegistry()
	all := r.All()
	if len(all) != 0 {
		t.Errorf("All() returned %d strategies on empty registry, want 0", len(all))
	}
}
