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

func TestHaversineKMSamePoint(t *testing.T) {
	d := HaversineKM(51.5074, -0.1278, 51.5074, -0.1278)
	if d != 0 {
		t.Errorf("expected 0, got %f", d)
	}
}

func TestHaversineKMLondonToParis(t *testing.T) {
	// London (51.5074, -0.1278) to Paris (48.8566, 2.3522) ~ 343 km
	d := HaversineKM(51.5074, -0.1278, 48.8566, 2.3522)
	if math.Abs(d-343) > 10 {
		t.Errorf("expected ~343 km, got %f", d)
	}
}

func TestHaversineKMLondonToManchester(t *testing.T) {
	// London (51.5074, -0.1278) to Manchester (53.4808, -2.2426) ~ 263 km
	d := HaversineKM(51.5074, -0.1278, 53.4808, -2.2426)
	if math.Abs(d-263) > 10 {
		t.Errorf("expected ~263 km, got %f", d)
	}
}

func TestHaversineKMAntipodal(t *testing.T) {
	// From (0,0) to (0,180) ~ half circumference ~ 20,004 km
	d := HaversineKM(0, 0, 0, 180)
	if math.Abs(d-20004) > 100 {
		t.Errorf("expected ~20004 km, got %f", d)
	}
}

func TestHaversineKMShortDistance(t *testing.T) {
	// Test short distance: ~1 km apart
	// Coordinates approximately 1 km apart in London
	d := HaversineKM(51.5074, -0.1278, 51.5174, -0.1278)
	if d < 0.9 || d > 1.5 {
		t.Errorf("expected ~1.1 km, got %f", d)
	}
}
