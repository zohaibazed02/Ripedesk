package delivery_test

import (
	"errors"
	"testing"

	"github.com/hospitality-os/cloud-api/internal/delivery"
	"github.com/shopspring/decimal"
)

func TestTripTransitionsPODEnforcementAndCODReconciliation(t *testing.T) {
	// 1. Valid transitions
	if err := delivery.ValidateTripTransition(delivery.TripAssigned, delivery.TripPickedUp); err != nil {
		t.Fatalf("expected valid assigned -> picked_up, got %v", err)
	}
	if err := delivery.ValidateTripTransition(delivery.TripOutForDelivery, delivery.TripDelivered); err != nil {
		t.Fatalf("expected valid out_for_delivery -> delivered, got %v", err)
	}
	// Illegal skip
	if err := delivery.ValidateTripTransition(delivery.TripAssigned, delivery.TripDelivered); !errors.Is(err, delivery.ErrInvalidTripTransition) {
		t.Fatalf("expected ErrInvalidTripTransition for assigned -> delivered, got %v", err)
	}

	// 2. POD enforcement on delivered
	if err := delivery.ValidatePOD("", ""); !errors.Is(err, delivery.ErrPODRequired) {
		t.Fatalf("expected ErrPODRequired when both photo and recipient are empty")
	}
	if err := delivery.ValidatePOD("https://cdn.example/pod-1.jpg", "Ali"); err != nil {
		t.Fatalf("expected valid POD to pass, got %v", err)
	}

	// 3. Exact Decimal COD reconciliation across 2 trips with -50.00 short variance
	rec := delivery.ComputeCODReconciliation(
		[]decimal.Decimal{decimal.RequireFromString("2480.00"), decimal.RequireFromString("1870.00")},
		[]decimal.Decimal{decimal.RequireFromString("2480.00"), decimal.RequireFromString("1870.00")},
		decimal.RequireFromString("4300.00"),
	)
	if !rec.TotalExpected.Equal(decimal.RequireFromString("4350.0000")) ||
		!rec.Variance.Equal(decimal.RequireFromString("-50.0000")) {
		t.Fatalf("expected total 4350.0000 and variance -50.0000, got %+v", rec)
	}
}
