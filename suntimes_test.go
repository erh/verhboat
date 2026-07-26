package verhboat

import (
	"testing"
	"time"

	"go.viam.com/test"
)

func TestSunTimes(t *testing.T) {
	// New York City, 2026-07-26. NOAA-published times (EDT = UTC-4):
	// sunrise ~05:53 EDT = 09:53 UTC, sunset ~20:15 EDT = 00:15 UTC (Jul 27).
	day := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	sunrise, sunset, err := SunTimes(day, 40.7128, -74.0060)
	test.That(t, err, test.ShouldBeNil)

	wantRise := time.Date(2026, 7, 26, 9, 53, 0, 0, time.UTC)
	wantSet := time.Date(2026, 7, 27, 0, 15, 0, 0, time.UTC)

	test.That(t, within(sunrise, wantRise, 10*time.Minute), test.ShouldBeTrue)
	test.That(t, within(sunset, wantSet, 10*time.Minute), test.ShouldBeTrue)

	// Sunset must follow sunrise on the same day.
	test.That(t, sunset.After(sunrise), test.ShouldBeTrue)
}

func TestDesiredOn(t *testing.T) {
	// NYC location.
	lat, lng := 40.7128, -74.0060

	// Mid-afternoon: off.
	afternoon := time.Date(2026, 7, 26, 19, 0, 0, 0, time.UTC) // 15:00 EDT
	on, err := desiredOn(afternoon, lat, lng)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeFalse)

	// Late evening (well after sunset): on.
	evening := time.Date(2026, 7, 27, 2, 0, 0, 0, time.UTC) // 22:00 EDT
	on, err = desiredOn(evening, lat, lng)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeTrue)

	// Pre-dawn (before sunrise): on.
	predawn := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC) // 04:00 EDT
	on, err = desiredOn(predawn, lat, lng)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeTrue)
}

func within(a, b time.Time, tol time.Duration) bool {
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return d <= tol
}
