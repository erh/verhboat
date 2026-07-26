package verhboat

// Sunrise/sunset computation using the standard "sunrise equation"
// (https://en.wikipedia.org/wiki/Sunrise_equation). Accurate to about a
// minute, which is plenty for scheduling a sign around dusk and dawn.

import (
	"errors"
	"math"
	"time"
)

const sunDegRad = math.Pi / 180

// ErrSunAlwaysDown means the sun never rises on the given day/location (polar night).
var ErrSunAlwaysDown = errors.New("sun never rises on this day at this location")

// ErrSunAlwaysUp means the sun never sets on the given day/location (polar day).
var ErrSunAlwaysUp = errors.New("sun never sets on this day at this location")

func julianDate(t time.Time) float64 {
	return float64(t.Unix())/86400.0 + 2440587.5
}

func julianToTime(jd float64) time.Time {
	return time.Unix(int64(math.Round((jd-2440587.5)*86400.0)), 0).UTC()
}

// SunTimes returns the sunrise and sunset times (in UTC) for the calendar day
// containing `day`, at the given latitude and longitude (degrees, longitude
// positive east). Sunset is the sunset following that day's solar noon, so it
// may land in the early UTC hours of the next date.
//
// At high latitudes the sun may not rise or set at all; in that case it
// returns ErrSunAlwaysDown or ErrSunAlwaysUp.
func SunTimes(day time.Time, lat, lng float64) (sunrise, sunset time.Time, err error) {
	// Number of days since J2000.0, for the given day.
	n := math.Round(julianDate(day) - 2451545.0 + 0.0008)

	// Mean solar time. The equation uses west longitude (positive west); with
	// lng positive east that is -lng, so J* = n - (-lng)/360 = n - lng/360.
	jStar := n - lng/360.0

	// Solar mean anomaly.
	m := math.Mod(357.5291+0.98560028*jStar, 360)
	mRad := m * sunDegRad

	// Equation of the center.
	c := 1.9148*math.Sin(mRad) + 0.0200*math.Sin(2*mRad) + 0.0003*math.Sin(3*mRad)

	// Ecliptic longitude.
	lambda := math.Mod(m+c+180+102.9372, 360)
	lRad := lambda * sunDegRad

	// Solar transit (Julian date of solar noon).
	jTransit := 2451545.0 + jStar + 0.0053*math.Sin(mRad) - 0.0069*math.Sin(2*lRad)

	// Declination of the sun.
	sinDec := math.Sin(lRad) * math.Sin(23.4397*sunDegRad)
	cosDec := math.Cos(math.Asin(sinDec))

	latRad := lat * sunDegRad
	// Hour angle for the sun's center at -0.833° (accounts for refraction and radius).
	cosOmega := (math.Sin(-0.833*sunDegRad) - math.Sin(latRad)*sinDec) / (math.Cos(latRad) * cosDec)
	if cosOmega > 1 {
		return time.Time{}, time.Time{}, ErrSunAlwaysDown
	}
	if cosOmega < -1 {
		return time.Time{}, time.Time{}, ErrSunAlwaysUp
	}

	omega := math.Acos(cosOmega) / sunDegRad // degrees

	jRise := jTransit - omega/360.0
	jSet := jTransit + omega/360.0

	return julianToTime(jRise), julianToTime(jSet), nil
}
