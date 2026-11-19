package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

var ErrInvalidWebhookSignature = errors.New("invalid webhook HMAC signature")

type AuthRequest struct {
	OrderID        string
	Amount         decimal.Decimal
	CurrencyCode   string
	IdempotencyKey string
}

type AuthResult struct {
	ProviderRef string `json:"provider_ref"`
	Status      string `json:"status"` // authorized | captured | declined
}

type CaptureResult struct {
	ProviderRef string `json:"provider_ref"`
	Status      string `json:"status"`
}

type ReversalResult struct {
	ReversalRef string `json:"reversal_ref"`
	Status      string `json:"status"`
}

type RefundResult struct {
	RefundRef string `json:"refund_ref"`
	Status    string `json:"status"`
}

type OnlinePaymentProvider interface {
	Name() string
	CreateAuthorization(ctx context.Context, req AuthRequest) (AuthResult, error)
	VerifyWebhookSignature(rawBody []byte, signatureHeader string) error
	CaptureAuthorization(ctx context.Context, providerRef string, amount decimal.Decimal) (CaptureResult, error)
	ReverseAuthorization(ctx context.Context, providerRef string, reason string) (ReversalResult, error)
	RefundCapturedPayment(ctx context.Context, providerRef string, amount decimal.Decimal, reason string) (RefundResult, error)
}

// SimulatedGatewayProvider provides deterministic HMAC-SHA256 webhooks and auth/capture/reverse/refund calls.
type SimulatedGatewayProvider struct {
	WebhookSecret string
	ReversedRefs  []string
	RefundedRefs  []string
	CapturedRefs  []string
}

func NewSimulatedGatewayProvider(secret string) *SimulatedGatewayProvider {
	if secret == "" {
		secret = "hospitality-dev-webhook-secret"
	}
	return &SimulatedGatewayProvider{WebhookSecret: secret}
}

func (p *SimulatedGatewayProvider) Name() string { return "simulated_gateway" }

func (p *SimulatedGatewayProvider) SignPayload(rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(p.WebhookSecret))
	_, _ = mac.Write(rawBody)
	return hex.EncodeToString(mac.Sum(nil))
}

func (p *SimulatedGatewayProvider) VerifyWebhookSignature(rawBody []byte, signatureHeader string) error {
	expected := p.SignPayload(rawBody)
	actual := strings.TrimPrefix(strings.TrimSpace(signatureHeader), "sha256=")
	if !hmac.Equal([]byte(expected), []byte(actual)) {
		return ErrInvalidWebhookSignature
	}
	return nil
}

func (p *SimulatedGatewayProvider) CreateAuthorization(_ context.Context, req AuthRequest) (AuthResult, error) {
	return AuthResult{
		ProviderRef: fmt.Sprintf("sim_auth_%s", req.OrderID),
		Status:      "authorized",
	}, nil
}

func (p *SimulatedGatewayProvider) CaptureAuthorization(_ context.Context, providerRef string, _ decimal.Decimal) (CaptureResult, error) {
	p.CapturedRefs = append(p.CapturedRefs, providerRef)
	return CaptureResult{ProviderRef: providerRef, Status: "captured"}, nil
}

func (p *SimulatedGatewayProvider) ReverseAuthorization(_ context.Context, providerRef string, _ string) (ReversalResult, error) {
	p.ReversedRefs = append(p.ReversedRefs, providerRef)
	return ReversalResult{ReversalRef: "rev_" + providerRef, Status: "reversed"}, nil
}

func (p *SimulatedGatewayProvider) RefundCapturedPayment(_ context.Context, providerRef string, _ decimal.Decimal, _ string) (RefundResult, error) {
	p.RefundedRefs = append(p.RefundedRefs, providerRef)
	return RefundResult{RefundRef: "ref_" + providerRef, Status: "refunded"}, nil
}
