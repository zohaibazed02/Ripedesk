package telemetry

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

const MaxTrackingTTL = 4 * time.Hour

var (
	ErrTrackingTokenExpired = errors.New("tracking link has expired for privacy")
	ErrInvalidTrackingToken = errors.New("invalid tracking token signature or format")
)

type TrackingClaims struct {
	TenantID  string `json:"tid"`
	OrderID   string `json:"oid"`
	TripID    string `json:"trp"`
	ExpiresAt int64  `json:"exp"`
}

func signBytes(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func GenerateTrackingToken(secret, tenantID, orderID, tripID string, ttl time.Duration, now time.Time) (string, error) {
	if ttl <= 0 || ttl > MaxTrackingTTL {
		ttl = MaxTrackingTTL
	}
	claims := TrackingClaims{
		TenantID:  tenantID,
		OrderID:   orderID,
		TripID:    tripID,
		ExpiresAt: now.UTC().Add(ttl).Unix(),
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	sig := signBytes(secret, []byte(encoded))
	return encoded + "." + sig, nil
}

func VerifyTrackingToken(secret, token string, now time.Time) (*TrackingClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidTrackingToken
	}
	expectedSig := signBytes(secret, []byte(parts[0]))
	if !hmac.Equal([]byte(expectedSig), []byte(parts[1])) {
		return nil, ErrInvalidTrackingToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidTrackingToken
	}
	var claims TrackingClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, ErrInvalidTrackingToken
	}
	if now.UTC().Unix() > claims.ExpiresAt {
		return nil, ErrTrackingTokenExpired
	}
	return &claims, nil
}

// HaversineKm calculates great-circle distance in kilometers between two WGS84 coordinates.
func HaversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLng := (lng2 - lng1) * math.Pi / 180.0
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

// EstimateETAMinutes estimates urban rider ETA assuming 24 km/h average city speed (minimum 2 mins).
func EstimateETAMinutes(distanceKm float64) int {
	mins := int(math.Ceil((distanceKm / 24.0) * 60.0))
	if mins < 2 {
		return 2
	}
	return mins
}
