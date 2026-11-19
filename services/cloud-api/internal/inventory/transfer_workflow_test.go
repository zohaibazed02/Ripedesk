package inventory_test

import (
	"errors"
	"testing"

	"github.com/hospitality-os/cloud-api/internal/inventory"
	"github.com/shopspring/decimal"
)

func TestTransferStateMachineTransitions(t *testing.T) {
	// Valid happy-path transitions
	validPairs := [][2]string{
		{inventory.TransferApproved, inventory.TransferReserved},
		{inventory.TransferReserved, inventory.TransferInTransit},
		{inventory.TransferInTransit, inventory.TransferPartiallyReceived},
		{inventory.TransferPartiallyReceived, inventory.TransferReceived},
		{inventory.TransferInTransit, inventory.TransferReceived},
	}
	for _, p := range validPairs {
		if err := inventory.ValidateTransferTransition(p[0], p[1]); err != nil {
			t.Fatalf("expected valid transition %s -> %s, got %v", p[0], p[1], err)
		}
	}

	// Illegal transitions (receiving before dispatch, dispatching without reservation)
	illegalPairs := [][2]string{
		{inventory.TransferApproved, inventory.TransferReceived},
		{inventory.TransferReserved, inventory.TransferReceived},
		{inventory.TransferApproved, inventory.TransferInTransit},
	}
	for _, p := range illegalPairs {
		if err := inventory.ValidateTransferTransition(p[0], p[1]); !errors.Is(err, inventory.ErrInvalidTransferTransition) {
			t.Fatalf("expected ErrInvalidTransferTransition for %s -> %s, got %v", p[0], p[1], err)
		}
	}
}

func TestTransferPartialReceiptDamageAndShortagePODraft(t *testing.T) {
	// Dispatch 12 L Oat Milk, receive 8 L good + 1 L damaged -> 3 L shortage + 3 L PO draft
	ev, err := inventory.EvaluateTransferReceiptLine(
		"ing-oat",
		decimal.NewFromInt(12),
		decimal.NewFromInt(8),
		decimal.NewFromInt(1),
		true,
	)
	if err != nil {
		t.Fatalf("EvaluateTransferReceiptLine failed: %v", err)
	}
	if !ev.ShortageQty.Equal(decimal.NewFromInt(3)) {
		t.Fatalf("expected 3 L shortage, got %s", ev.ShortageQty)
	}
	if !ev.NeedsPODraft || !ev.PODraftQty.Equal(decimal.NewFromInt(3)) {
		t.Fatalf("expected 3 L shortage PO draft, got %+v", ev)
	}

	// Exceeding dispatched quantity must be rejected
	_, err = inventory.EvaluateTransferReceiptLine(
		"ing-oat",
		decimal.NewFromInt(12),
		decimal.NewFromInt(10),
		decimal.NewFromInt(3),
		false,
	)
	if !errors.Is(err, inventory.ErrReceiptExceedsDispatched) {
		t.Fatalf("expected ErrReceiptExceedsDispatched, got %v", err)
	}

	// Idempotency key determinism
	k1 := inventory.DeterministicTransferIdempotencyKey("dispatch", "tr-1", "ing-oat", "req-99")
	k2 := inventory.DeterministicTransferIdempotencyKey("dispatch", "tr-1", "ing-oat", "req-99")
	if k1 != k2 {
		t.Fatalf("expected deterministic idempotency keys, got %q vs %q", k1, k2)
	}
}
