package sync

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/hospitality-os/branch-service/internal/sqlite"
)

type State string

const (
	StateOnlineSynced     State = "ONLINE_SYNCED"
	StateLANMode          State = "LAN_MODE"
	StateIsolatedTerminal State = "ISOLATED_TERMINAL"
)

const (
	DefaultBatchSize = 50
	MaxBackoff       = 60 * time.Second
)

type StatusBroadcaster interface {
	BroadcastSyncStatus(cloudReachable bool, pendingCount int)
}

type Engine struct {
	store       *sqlite.Store
	httpClient  *http.Client
	broadcaster StatusBroadcaster
	authToken   string
}

func NewEngine(store *sqlite.Store, broadcaster StatusBroadcaster, authToken string) *Engine {
	cleanToken := strings.Trim(strings.TrimSpace(authToken), `"'`)
	return &Engine{
		store: store,
		httpClient: &http.Client{
			Timeout: 25 * time.Second,
		},
		broadcaster: broadcaster,
		authToken:   cleanToken,
	}
}

type outboxRow struct {
	SeqID          int64
	EventID        string
	CommandType    string
	AggregateType  string
	AggregateID    string
	PayloadJSON    string
	IdempotencyKey string
	RetryCount     int
	CreatedAt      string
}

type branchConfig struct {
	TenantID     string
	BranchID     string
	DeviceID     string
	DeviceSecret string
	CloudAPIURL  string
	CatalogETag  string
	Reachable    bool
}

func (e *Engine) loadConfig(ctx context.Context) (*branchConfig, error) {
	var cfg branchConfig
	var reachableInt int
	err := e.store.DB().QueryRowContext(ctx, `
		SELECT tenant_id, branch_id, device_id, device_secret, cloud_api_url, catalog_etag, cloud_reachable
		FROM branch_state WHERE id = 1
	`).Scan(&cfg.TenantID, &cfg.BranchID, &cfg.DeviceID, &cfg.DeviceSecret, &cfg.CloudAPIURL, &cfg.CatalogETag, &reachableInt)
	if err != nil {
		return nil, err
	}
	cfg.Reachable = reachableInt == 1
	return &cfg, nil
}

func (e *Engine) setCloudReachable(ctx context.Context, reachable bool) {
	val := 0
	if reachable {
		val = 1
	}
	if reachable {
		_, _ = e.store.DB().ExecContext(ctx, `
			UPDATE branch_state SET cloud_reachable = ?, last_cloud_sync_at = ? WHERE id = 1
		`, val, time.Now().UTC().Format(time.RFC3339Nano))
	} else {
		_, _ = e.store.DB().ExecContext(ctx, `
			UPDATE branch_state SET cloud_reachable = ? WHERE id = 1
		`, val)
	}
	if e.broadcaster != nil {
		e.broadcaster.BroadcastSyncStatus(reachable, e.store.PendingQueueDepth())
	}
}

func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// ComputeBackoff returns capped exponential backoff (1s, 2s, 4s... up to 60s) with up to 20% jitter.
func ComputeBackoff(retryCount int) time.Duration {
	if retryCount < 0 {
		retryCount = 0
	}
	shift := retryCount
	if shift > 6 {
		shift = 6
	}
	base := time.Duration(1<<shift) * time.Second
	if base > MaxBackoff {
		base = MaxBackoff
	}
	jitter := time.Duration(rand.Int63n(int64(base/5) + 1))
	return base + jitter
}

// ShipNextBatch polls up to 50 pending outbox items in strict FIFO seq_id order and pushes them to Cloud API.
func (e *Engine) ShipNextBatch(ctx context.Context) (int, error) {
	cfg, err := e.loadConfig(ctx)
	if err != nil || cfg.CloudAPIURL == "" || cfg.BranchID == "" {
		return 0, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := e.store.DB().QueryContext(ctx, `
		SELECT seq_id, event_id, command_type, aggregate_type, aggregate_id, payload_json, idempotency_key, retry_count, created_at
		FROM local_outbox
		WHERE status = 'pending' AND (next_retry_at IS NULL OR next_retry_at <= ?)
		ORDER BY seq_id ASC
		LIMIT ?
	`, now, DefaultBatchSize)
	if err != nil {
		return 0, err
	}

	var batch []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.SeqID, &r.EventID, &r.CommandType, &r.AggregateType, &r.AggregateID, &r.PayloadJSON, &r.IdempotencyKey, &r.RetryCount, &r.CreatedAt); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, r)
	}
	rows.Close()
	if len(batch) == 0 {
		return 0, nil
	}

	type wireCmd struct {
		EventID          string          `json:"event_id"`
		CommandType      string          `json:"command_type"`
		AggregateType    string          `json:"aggregate_type"`
		AggregateID      string          `json:"aggregate_id"`
		IdempotencyKey   string          `json:"idempotency_key"`
		Payload          json.RawMessage `json:"payload"`
		CreatedOfflineAt string          `json:"created_offline_at"`
	}
	wireCmds := make([]wireCmd, 0, len(batch))
	for _, b := range batch {
		wireCmds = append(wireCmds, wireCmd{
			EventID:          b.EventID,
			CommandType:      b.CommandType,
			AggregateType:    b.AggregateType,
			AggregateID:      b.AggregateID,
			IdempotencyKey:   b.IdempotencyKey,
			Payload:          json.RawMessage(b.PayloadJSON),
			CreatedOfflineAt: b.CreatedAt,
		})
	}

	snap, _ := e.store.HealthSnapshot(ctx)
	pendingCount := 0
	if snap != nil {
		pendingCount = snap.PendingOutboxCount
	}

	reqBody, _ := json.Marshal(map[string]any{
		"pending_sync_count": pendingCount,
		"commands":           wireCmds,
	})

	url := fmt.Sprintf("%s/api/v1/branches/%s/sync/push", cfg.CloudAPIURL, cfg.BranchID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.authToken)
	}
	req.Header.Set("X-Device-ID", cfg.DeviceID)
	req.Header.Set("X-Device-Signature", cfg.DeviceSecret)

	resp, err := e.httpClient.Do(req)
	if err != nil || resp.StatusCode >= 500 {
		detail := ""
		if err != nil {
			detail = fmt.Sprintf("network error: %v", err)
		} else {
			snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
			_ = resp.Body.Close()
			detail = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		}
		// Apply capped exponential backoff with jitter to each item in the failed batch
		for _, item := range batch {
			nextRetry := time.Now().UTC().Add(ComputeBackoff(item.RetryCount)).Format(time.RFC3339Nano)
			_, _ = e.store.DB().ExecContext(ctx, `
				UPDATE local_outbox SET retry_count = retry_count + 1, next_retry_at = ? WHERE seq_id = ?
			`, nextRetry, item.SeqID)
		}
		e.setCloudReachable(ctx, false)
		return 0, fmt.Errorf("cloud sync failed (%s)", detail)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return 0, fmt.Errorf("cloud sync unexpected status: %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var envelope struct {
		Data struct {
			Acks []struct {
				EventID string `json:"event_id"`
				Status  string `json:"status"`
				Reason  string `json:"reason"`
			} `json:"acks"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return 0, err
	}

	processed := 0
	for _, ack := range envelope.Data.Acks {
		switch ack.Status {
		case "acked", "duplicate_acked":
			_, _ = e.store.DB().ExecContext(ctx, `UPDATE local_outbox SET status = 'acked' WHERE event_id = ?`, ack.EventID)
			processed++
		case "rejected":
			_, _ = e.store.DB().ExecContext(ctx, `UPDATE local_outbox SET status = 'rejected' WHERE event_id = ?`, ack.EventID)
			processed++
		}
	}

	e.setCloudReachable(ctx, true)
	return processed, nil
}

// SendHeartbeat reports pending outbox depth and oldest pending tx timestamp to Cloud API.
func (e *Engine) SendHeartbeat(ctx context.Context) error {
	cfg, err := e.loadConfig(ctx)
	if err != nil {
		return fmt.Errorf("heartbeat loadConfig: %w", err)
	}
	if cfg.CloudAPIURL == "" || cfg.BranchID == "" {
		return fmt.Errorf("heartbeat skipped: cloud_api_url=%q branch_id=%q", cfg.CloudAPIURL, cfg.BranchID)
	}
	snap, err := e.store.HealthSnapshot(ctx)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"pending_sync_count": snap.PendingOutboxCount,
	}
	if snap.OldestPendingAt != "" {
		payload["oldest_pending_tx_at"] = snap.OldestPendingAt
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/api/v1/branches/%s/heartbeat", cfg.CloudAPIURL, cfg.BranchID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.authToken)
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		e.setCloudReachable(ctx, false)
		return fmt.Errorf("heartbeat POST %s network error: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		e.setCloudReachable(ctx, false)
		return fmt.Errorf("heartbeat POST %s failed with HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		// Cloud API is reachable, but BRANCH_AUTH_TOKEN is missing, invalid, or out of branch scope
		e.setCloudReachable(ctx, true)
		return fmt.Errorf("heartbeat POST %s returned HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	e.setCloudReachable(ctx, true)
	return nil
}

// RefreshCatalog fetches /api/v1/branches/{branchId}/catalog using If-None-Match ETag caching.
func (e *Engine) RefreshCatalog(ctx context.Context) error {
	cfg, err := e.loadConfig(ctx)
	if err != nil || cfg.CloudAPIURL == "" || cfg.BranchID == "" {
		return err
	}
	url := fmt.Sprintf("%s/api/v1/branches/%s/catalog", cfg.CloudAPIURL, cfg.BranchID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if e.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.authToken)
	}
	if cfg.CatalogETag != "" {
		req.Header.Set("If-None-Match", cfg.CatalogETag)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("catalog refresh status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	etag := resp.Header.Get("ETag")
	_, err = e.store.DB().ExecContext(ctx, `
		UPDATE branch_state SET catalog_etag = ?, catalog_json = ? WHERE id = 1
	`, etag, string(raw))
	return err
}
