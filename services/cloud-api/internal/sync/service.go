package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/jackc/pgx/v5"
)

const MaxBatchSize = 100

var ErrBatchTooLarge = errors.New("sync batch exceeds maximum size of 100 commands")

type PushCommand struct {
	EventID          string          `json:"event_id"`
	CommandType      string          `json:"command_type"`
	AggregateType    string          `json:"aggregate_type"`
	AggregateID      string          `json:"aggregate_id"`
	IdempotencyKey   string          `json:"idempotency_key"`
	Payload          json.RawMessage `json:"payload"`
	CreatedOfflineAt time.Time       `json:"created_offline_at"`
}

type PushBatchRequest struct {
	PendingSyncCount  int           `json:"pending_sync_count"`
	OldestPendingTxAt *time.Time    `json:"oldest_pending_tx_at,omitempty"`
	Commands          []PushCommand `json:"commands"`
}

type CommandAck struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"` // acked | duplicate_acked | rejected
	Reason  string `json:"reason,omitempty"`
}

type PushBatchResponse struct {
	Acks     []CommandAck `json:"acks"`
	CloudSeq string       `json:"cloud_seq"`
}

type PullEvent struct {
	ID            string          `json:"id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
	CreatedAt     time.Time       `json:"created_at"`
}

type PullResponse struct {
	Events     []PullEvent `json:"events"`
	NextCursor string      `json:"next_cursor"`
}

type ReviewQueueItem struct {
	ID             string          `json:"id"`
	BranchID       string          `json:"branch_id"`
	CommandID      string          `json:"command_id"`
	CommandType    string          `json:"command_type"`
	Payload        json.RawMessage `json:"payload"`
	ConflictReason string          `json:"conflict_reason"`
	Status         string          `json:"status"`
	CreatedAt      time.Time       `json:"created_at"`
}

type ResolveReviewInput struct {
	Status string `json:"status"` // resolved | discarded
}

type Service struct {
	pool *db.Pool
}

func NewService(pool *db.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) Name() string {
	return "sync"
}

func (s *Service) PushBatch(ctx context.Context, tenantID, branchID string, req PushBatchRequest) (*PushBatchResponse, error) {
	if len(req.Commands) > MaxBatchSize {
		return nil, ErrBatchTooLarge
	}

	acks := make([]CommandAck, 0, len(req.Commands))

	for _, cmd := range req.Commands {
		ack, err := s.processSingleCommand(ctx, tenantID, branchID, cmd)
		if err != nil {
			return nil, err
		}
		acks = append(acks, ack)
	}

	// Update branch sync metadata
	_ = s.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE branches
			SET last_sync_at = NOW(),
			    pending_sync_count = $2,
			    oldest_pending_tx_at = $3
			WHERE id = $1
		`, branchID, req.PendingSyncCount, req.OldestPendingTxAt)
		return err
	})

	return &PushBatchResponse{
		Acks:     acks,
		CloudSeq: time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func (s *Service) processSingleCommand(ctx context.Context, tenantID, branchID string, cmd PushCommand) (CommandAck, error) {
	var ack CommandAck
	ack.EventID = cmd.EventID

	err := s.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		// 1. Idempotency check in processed_commands
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM processed_commands WHERE command_id = $1)
		`, cmd.EventID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			ack.Status = "duplicate_acked"
			return nil
		}

		// 2. Apply domain state transition
		conflictReason, applyErr := applyCommandTx(ctx, tx, tenantID, branchID, cmd)
		if applyErr != nil {
			conflictReason = applyErr.Error()
		}

		if conflictReason != "" {
			// Route poison-pill or version-conflicting command to sync_review_queue
			if _, err := tx.Exec(ctx, `
				INSERT INTO sync_review_queue (tenant_id, branch_id, command_id, command_type, payload, conflict_reason, status)
				VALUES ($1, $2, $3, $4, $5::jsonb, $6, 'pending_review')
			`, tenantID, branchID, cmd.EventID, cmd.CommandType, string(cmd.Payload), conflictReason); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO processed_commands (command_id, tenant_id, branch_id, command_type, result_status)
				VALUES ($1, $2, $3, $4, 'rejected')
				ON CONFLICT (command_id) DO NOTHING
			`, cmd.EventID, tenantID, branchID, cmd.CommandType); err != nil {
				return err
			}
			ack.Status = "rejected"
			ack.Reason = conflictReason
			return nil
		}

		// 3. Mark command processed & emit cloud outbox event
		if _, err := tx.Exec(ctx, `
			INSERT INTO processed_commands (command_id, tenant_id, branch_id, command_type, result_status)
			VALUES ($1, $2, $3, $4, 'acked')
		`, cmd.EventID, tenantID, branchID, cmd.CommandType); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO outbox_events (tenant_id, branch_id, aggregate_type, aggregate_id, event_type, payload)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		`, tenantID, branchID, cmd.AggregateType, cmd.AggregateID, cmd.CommandType, string(cmd.Payload)); err != nil {
			return err
		}

		ack.Status = "acked"
		return nil
	})
	if err != nil {
		return ack, err
	}
	return ack, nil
}

func applyCommandTx(ctx context.Context, tx pgx.Tx, tenantID, branchID string, cmd PushCommand) (string, error) {
	switch cmd.CommandType {
	case "order.upserted":
		var p struct {
			OrderID      string `json:"order_id"`
			OrderNumber  string `json:"order_number"`
			OrderMode    string `json:"order_mode"`
			OrderVersion int    `json:"order_version"`
			GrandTotal   string `json:"grand_total"`
		}
		if err := json.Unmarshal(cmd.Payload, &p); err != nil {
			return "invalid order.upserted payload", nil
		}
		if p.OrderID == "" {
			p.OrderID = cmd.AggregateID
		}
		if p.OrderMode == "" {
			p.OrderMode = "quick_sale"
		}
		if p.GrandTotal == "" {
			p.GrandTotal = "0.0000"
		}

		var existingVersion int
		err := tx.QueryRow(ctx, `SELECT order_version FROM orders WHERE id = $1`, p.OrderID).Scan(&existingVersion)
		if err == nil {
			if p.OrderVersion <= existingVersion {
				return fmt.Sprintf("order_version conflict: incoming %d <= current %d", p.OrderVersion, existingVersion), nil
			}
			_, err = tx.Exec(ctx, `
				UPDATE orders
				SET order_version = $2, grand_total = $3::numeric, sync_status = 'synced'
				WHERE id = $1
			`, p.OrderID, p.OrderVersion, p.GrandTotal)
			return "", err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO orders (id, tenant_id, branch_id, order_number, order_mode, order_version, status, grand_total, sync_status, created_offline_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'in_prep', $7::numeric, 'synced', $8)
		`, p.OrderID, tenantID, branchID, p.OrderNumber, p.OrderMode, maxInt(p.OrderVersion, 1), p.GrandTotal, cmd.CreatedOfflineAt)
		return "", err

	case "kds.ticket_updated":
		var p struct {
			TicketID string `json:"ticket_id"`
			Status   string `json:"status"`
		}
		if err := json.Unmarshal(cmd.Payload, &p); err != nil {
			return "invalid kds.ticket_updated payload", nil
		}
		if _, err := tx.Exec(ctx, `UPDATE kitchen_tickets SET status = $2 WHERE id = $1`, p.TicketID, p.Status); err != nil {
			return "", err
		}
		if p.Status == "preparing" {
			if err := ApplyKDSPrepDeductionTx(ctx, tx, tenantID, branchID, cmd.EventID, cmd.Payload); err != nil {
				return "", err
			}
		}
		return "", nil

	case "payment.recorded":
		var p struct {
			PaymentID   string `json:"payment_id"`
			OrderID     string `json:"order_id"`
			CheckNumber int    `json:"check_number"`
			Method      string `json:"method"`
			Amount      string `json:"amount"`
		}
		if err := json.Unmarshal(cmd.Payload, &p); err != nil {
			return "invalid payment.recorded payload", nil
		}
		if p.PaymentID == "" {
			p.PaymentID = cmd.AggregateID
		}
		if p.CheckNumber <= 0 {
			p.CheckNumber = 1
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO payments (id, tenant_id, branch_id, order_id, check_number, method, status, amount)
			VALUES ($1, $2, $3, $4, $5, $6, 'captured', $7::numeric)
			ON CONFLICT (id) DO NOTHING
		`, p.PaymentID, tenantID, branchID, p.OrderID, p.CheckNumber, p.Method, p.Amount)
		return "", err

	case "cash_shift.closed":
		var p struct {
			ShiftID     string `json:"shift_id"`
			CountedCash string `json:"counted_cash"`
		}
		if err := json.Unmarshal(cmd.Payload, &p); err != nil {
			return "invalid cash_shift.closed payload", nil
		}
		_, err := tx.Exec(ctx, `
			UPDATE cash_shifts SET status = 'closed', counted_cash = $2::numeric, closed_at = NOW() WHERE id = $1
		`, p.ShiftID, p.CountedCash)
		return "", err

	case "kitchen_requisition.submitted":
		var p struct {
			RequisitionID string `json:"requisition_id"`
			RequestedBy   string `json:"requested_by"`
			TemplateName  string `json:"template_name"`
		}
		if err := json.Unmarshal(cmd.Payload, &p); err != nil {
			return "invalid kitchen_requisition.submitted payload", nil
		}
		if p.RequisitionID == "" {
			p.RequisitionID = cmd.AggregateID
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO kitchen_requisitions (id, tenant_id, branch_id, requested_by, status, needed_by, template_name)
			VALUES ($1, $2, $3, $4, 'submitted', NOW() + interval '4 hours', NULLIF($5, ''))
			ON CONFLICT (id) DO NOTHING
		`, p.RequisitionID, tenantID, branchID, p.RequestedBy, p.TemplateName)
		return "", err

	default:
		return fmt.Sprintf("unsupported command_type %q", cmd.CommandType), nil
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *Service) PullEvents(ctx context.Context, tenantID, branchID, sinceCursor string) (*PullResponse, error) {
	sinceTime := time.Unix(0, 0).UTC()
	if sinceCursor != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, sinceCursor); err == nil {
			sinceTime = parsed
		}
	}

	events := []PullEvent{}
	nextCursor := sinceCursor
	err := s.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at
			FROM outbox_events
			WHERE (branch_id IS NULL OR branch_id = $1) AND created_at > $2
			ORDER BY created_at ASC
			LIMIT 200
		`, branchID, sinceTime)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ev PullEvent
			if err := rows.Scan(&ev.ID, &ev.AggregateType, &ev.AggregateID, &ev.EventType, &ev.Payload, &ev.CreatedAt); err != nil {
				return err
			}
			events = append(events, ev)
			nextCursor = ev.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return &PullResponse{Events: events, NextCursor: nextCursor}, nil
}

func (s *Service) ListReviewQueue(ctx context.Context, tenantID, branchID string) ([]ReviewQueueItem, error) {
	items := []ReviewQueueItem{}
	err := s.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, branch_id, command_id, command_type, payload, conflict_reason, status, created_at
			FROM sync_review_queue
			WHERE branch_id = $1
			ORDER BY created_at DESC
			LIMIT 100
		`, branchID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var it ReviewQueueItem
			if err := rows.Scan(&it.ID, &it.BranchID, &it.CommandID, &it.CommandType, &it.Payload, &it.ConflictReason, &it.Status, &it.CreatedAt); err != nil {
				return err
			}
			items = append(items, it)
		}
		return rows.Err()
	})
	return items, err
}

func (s *Service) ResolveReviewItem(ctx context.Context, tenantID, branchID, itemID, resolvedBy, status string) error {
	if status != "resolved" && status != "discarded" {
		return errors.New("status must be 'resolved' or 'discarded'")
	}
	return s.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE sync_review_queue
			SET status = $3, resolved_by = $4, resolved_at = NOW()
			WHERE id = $1 AND branch_id = $2
		`, itemID, branchID, status, resolvedBy)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
}
