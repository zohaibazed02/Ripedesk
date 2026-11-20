package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// HTTPPaymentProvider implements OnlinePaymentProvider against a live external PSP REST API.
type HTTPPaymentProvider struct {
	BaseURL       string
	APIKey        string
	WebhookSecret string
	Client        *http.Client
}

// NewProviderFromEnv returns HTTPPaymentProvider when PAYMENT_GATEWAY_URL and PAYMENT_API_KEY
// are configured, otherwise falls back to SimulatedGatewayProvider.
func NewProviderFromEnv() OnlinePaymentProvider {
	baseURL := strings.TrimRight(os.Getenv("PAYMENT_GATEWAY_URL"), "/")
	apiKey := os.Getenv("PAYMENT_API_KEY")
	webhookSecret := os.Getenv("PAYMENT_WEBHOOK_SECRET")
	if baseURL != "" && apiKey != "" {
		return &HTTPPaymentProvider{
			BaseURL:       baseURL,
			APIKey:        apiKey,
			WebhookSecret: webhookSecret,
			Client:        &http.Client{Timeout: 15 * time.Second},
		}
	}
	return NewSimulatedGatewayProvider(webhookSecret)
}

func (p *HTTPPaymentProvider) Name() string { return "http_payment_gateway" }

func (p *HTTPPaymentProvider) VerifyWebhookSignature(rawBody []byte, signatureHeader string) error {
	mac := hmac.New(sha256.New, []byte(p.WebhookSecret))
	_, _ = mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	actual := strings.TrimPrefix(strings.TrimSpace(signatureHeader), "sha256=")
	if !hmac.Equal([]byte(expected), []byte(actual)) {
		return ErrInvalidWebhookSignature
	}
	return nil
}

func (p *HTTPPaymentProvider) postJSON(ctx context.Context, path, idemKey string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("payment gateway %s returned HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (p *HTTPPaymentProvider) CreateAuthorization(ctx context.Context, req AuthRequest) (AuthResult, error) {
	var res AuthResult
	err := p.postJSON(ctx, "/authorize", req.IdempotencyKey, map[string]any{
		"order_id":      req.OrderID,
		"amount":        req.Amount.StringFixed(2),
		"currency_code": req.CurrencyCode,
	}, &res)
	return res, err
}

func (p *HTTPPaymentProvider) CaptureAuthorization(ctx context.Context, providerRef string, amount decimal.Decimal) (CaptureResult, error) {
	var res CaptureResult
	err := p.postJSON(ctx, "/capture", "cap-"+providerRef, map[string]any{
		"provider_ref": providerRef,
		"amount":       amount.StringFixed(2),
	}, &res)
	return res, err
}

func (p *HTTPPaymentProvider) ReverseAuthorization(ctx context.Context, providerRef string, reason string) (ReversalResult, error) {
	var res ReversalResult
	err := p.postJSON(ctx, "/reverse", "rev-"+providerRef, map[string]any{
		"provider_ref": providerRef,
		"reason":       reason,
	}, &res)
	return res, err
}

func (p *HTTPPaymentProvider) RefundCapturedPayment(ctx context.Context, providerRef string, amount decimal.Decimal, reason string) (RefundResult, error) {
	var res RefundResult
	err := p.postJSON(ctx, "/refund", "ref-"+providerRef, map[string]any{
		"provider_ref": providerRef,
		"amount":       amount.StringFixed(2),
		"reason":       reason,
	}, &res)
	return res, err
}
