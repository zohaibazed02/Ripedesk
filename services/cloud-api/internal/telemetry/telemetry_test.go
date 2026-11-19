package telemetry_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hospitality-os/cloud-api/internal/telemetry"
)

func TestPrivacyGuardOutOfOrderAndExpiringTrackingTokens(t *testing.T) {
	// 1. Active-Trip Privacy Guard
	if !telemetry.IsTripTrackable("picked_up") || !telemetry.IsTripTrackable("out_for_delivery") {
		t.Fatalf("expected picked_up and out_for_delivery to be trackable")
	}
	if telemetry.IsTripTrackable("unassigned") || telemetry.IsTripTrackable("delivered") || telemetry.IsTripTrackable("failed_delivery") {
		t.Fatalf("privacy violation: inactive trips must NOT be trackable")
	}

	// 2. Coordinate bounds validation
	if err := telemetry.ValidateCoordinate(95.0, 73.0); !errors.Is(err, telemetry.ErrInvalidCoordinates) {
		t.Fatalf("expected ErrInvalidCoordinates for lat=95")
	}

	// 3. Out-of-order buffered ping protection
	now := time.Now().UTC()
	older := now.Add(-2 * time.Minute)
	if telemetry.ShouldUpdateLastKnown(&now, older) {
		t.Fatalf("older buffered ping must NOT overwrite fresher coordinate")
	}
	if !telemetry.ShouldUpdateLastKnown(&older, now) {
		t.Fatalf("fresher ping should update last known coordinate")
	}

	// 4. Expiring HMAC-SHA256 tracking token -> 410 Gone after TTL
	tok, err := telemetry.GenerateTrackingToken("sec", "tn-1", "ord-1", "trp-1", 10*time.Minute, now)
	if err != nil {
		t.Fatalf("GenerateTrackingToken failed: %v", err)
	}
	if _, err := telemetry.VerifyTrackingToken("sec", tok, now.Add(5*time.Minute)); err != nil {
		t.Fatalf("expected valid token within TTL, got %v", err)
	}
	if _, err := telemetry.VerifyTrackingToken("sec", tok, now.Add(15*time.Minute)); !errors.Is(err, telemetry.ErrTrackingTokenExpired) {
		t.Fatalf("expected ErrTrackingTokenExpired after TTL, got %v", err)
	}
}
