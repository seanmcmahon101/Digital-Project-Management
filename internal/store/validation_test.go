package store

import (
	"math"
	"testing"
)

func TestBenefitRejectsNonFiniteNumbers(t *testing.T) {
	s := testStore(t)
	p, err := s.CreateProject("Benefits validation", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	nan := math.NaN()
	if err := s.CreateBenefit(p.ID, "Invalid", "custom", "units", "increase", &nan, nil, "", 0, ""); err == nil {
		t.Fatal("NaN baseline was accepted")
	}
	if err := s.CreateBenefit(p.ID, "Negative value", "custom", "units", "increase", nil, nil, "", -1, ""); err == nil {
		t.Fatal("negative annual value was accepted")
	}
	if err := s.AddMeasurement(1, math.Inf(1), Today(), ""); err == nil {
		t.Fatal("infinite measurement was accepted")
	}
}
