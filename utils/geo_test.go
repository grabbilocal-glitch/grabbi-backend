package utils

import (
	"math"
	"testing"
)

func TestHaversineSamePoint(t *testing.T) {
	d := Haversine(51.5074, -0.1278, 51.5074, -0.1278)
	if d != 0 {
		t.Errorf("expected 0, got %f", d)
	}
}

func TestHaversineLondonToParis(t *testing.T) {
	// London (51.5074, -0.1278) to Paris (48.8566, 2.3522) ~ 213 miles
	d := Haversine(51.5074, -0.1278, 48.8566, 2.3522)
	if math.Abs(d-213) > 5 {
		t.Errorf("expected ~213 miles, got %f", d)
	}
}

func TestHaversineAntipodal(t *testing.T) {
	// From (0,0) to (0,180) ~ half circumference ~ 12,451 miles
	d := Haversine(0, 0, 0, 180)
	if math.Abs(d-12451) > 50 {
		t.Errorf("expected ~12451 miles, got %f", d)
	}
}
