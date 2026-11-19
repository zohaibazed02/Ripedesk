package storefront_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hospitality-os/cloud-api/internal/payments"
	"github.com/hospitality-os/cloud-api/internal/storefront"
	"github.com/shopspring/decimal"
)

func TestHeartbeatGuardWebhookSignatureAndInterruptedHandoffReversal(t *testing.T) {
	now := time.Now().UTC()
	expired := now.Add(-30 * time.Second)

	// 1. Expired branch heartbeat rejects checkout with ErrBranchOffline (503)
	if err := storefront.CheckBranchHeartbeat(&expired, now); !errors.Is(err, storefront.ErrBranchOffline) {
		t.Fatalf("expected ErrBranchOffline, got %v", err)
	}

	// 2. Signed HMAC-SHA256 webhook verification
	gw := payments.NewSimulatedGatewayProvider("super-secret")
	payload := []byte(`{"provider_event_id":"ev-1","order_id":"ord-1","event_type":"payment.authorized"}`)
	validSig := gw.SignPayload(payload)

	if err := gw.VerifyWebhookSignature(payload, validSig); err != nil {
		t.Fatalf("expected valid signature to pass, got %v", err)
	}
	if err := gw.VerifyWebhookSignature(payload, "tampered-sig"); !errors.Is(err, payments.ErrInvalidWebhookSignature) {
		t.Fatalf("expected ErrInvalidWebhookSignature for tampered signature, got %v", err)
	}

	// 3. Interrupted handoff timeout triggers automatic ReverseAuthorization
	res, err := storefront.ReconcileStaleWebHandoff(
		context.Background(),
		gw,
		storefront.PendingWebOrder{
			OrderID:         "ord-stuck-1",
			PaymentStatus:   "authorized",
			ProviderRef:     "sim_auth_ord-stuck-1",
			Amount:          decimal.RequireFromString("1357.20"),
			PendingSinceUTC: now.Add(-15 * time.Minute),
		},
		now,
		10*time.Minute,
	)
	if err != nil {
		t.Fatalf("ReconcileStaleWebHandoff failed: %v", err)
	}
	if res.ActionTaken != "cancelled_reversed" || len(gw.ReversedRefs) != 1 {
		t.Fatalf("expected cancelled_reversed with 1 gateway reversal, got %+v", res)
	}
}
