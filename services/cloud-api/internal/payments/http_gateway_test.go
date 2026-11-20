package payments_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hospitality-os/cloud-api/internal/payments"
	"github.com/shopspring/decimal"
)

func TestHTTPPaymentProvider_AuthorizeCaptureReverseAndWebhook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer live-psp-key" || r.Header.Get("Idempotency-Key") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/authorize":
			_ = json.NewEncoder(w).Encode(payments.AuthResult{ProviderRef: "psp_auth_101", Status: "authorized"})
		case "/capture":
			_ = json.NewEncoder(w).Encode(payments.CaptureResult{ProviderRef: "psp_auth_101", Status: "captured"})
		case "/reverse":
			_ = json.NewEncoder(w).Encode(payments.ReversalResult{ReversalRef: "psp_rev_101", Status: "reversed"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	gw := &payments.HTTPPaymentProvider{
		BaseURL:       srv.URL,
		APIKey:        "live-psp-key",
		WebhookSecret: "whsec_test_123",
	}

	ctx := context.Background()
	authRes, err := gw.CreateAuthorization(ctx, payments.AuthRequest{
		OrderID:        "ord-web-101",
		Amount:         decimal.RequireFromString("1357.20"),
		CurrencyCode:   "PKR",
		IdempotencyKey: "idem-auth-101",
	})
	if err != nil || authRes.ProviderRef != "psp_auth_101" {
		t.Fatalf("CreateAuthorization failed: res=%+v err=%v", authRes, err)
	}

	capRes, err := gw.CaptureAuthorization(ctx, authRes.ProviderRef, decimal.RequireFromString("1357.20"))
	if err != nil || capRes.Status != "captured" {
		t.Fatalf("CaptureAuthorization failed: res=%+v err=%v", capRes, err)
	}

	revRes, err := gw.ReverseAuthorization(ctx, authRes.ProviderRef, "branch_handoff_timeout")
	if err != nil || revRes.Status != "reversed" {
		t.Fatalf("ReverseAuthorization failed: res=%+v err=%v", revRes, err)
	}

	raw := []byte(`{"event":"payment.authorized","provider_ref":"psp_auth_101"}`)
	mac := hmac.New(sha256.New, []byte("whsec_test_123"))
	_, _ = mac.Write(raw)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if err := gw.VerifyWebhookSignature(raw, sig); err != nil {
		t.Fatalf("valid webhook signature rejected: %v", err)
	}
	if err := gw.VerifyWebhookSignature(raw, "sha256=bad"); err == nil {
		t.Fatalf("expected error for invalid webhook signature")
	}
}
