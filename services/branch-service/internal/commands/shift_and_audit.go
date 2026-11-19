package commands

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hospitality-os/branch-service/internal/hardware/escpos"
)

type OpenShiftInput struct {
	ShiftID        string `json:"shift_id"`
	OpenedBy       string `json:"opened_by"`
	DeviceID       string `json:"device_id"`
	OpeningFloat   string `json:"opening_float"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (p *Processor) OpenCashShift(ctx context.Context, in OpenShiftInput) (string, error) {
	if in.ShiftID == "" {
		in.ShiftID = uuid.NewString()
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = "shift-open-" + in.ShiftID
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	conn, err := p.store.BeginImmediate(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		_ = conn.Close()
	}()

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_cash_shifts (id, opened_by, device_id, opening_float, expected_cash, status, opened_at)
		VALUES (?, ?, ?, ?, ?, 'open', ?)
	`, in.ShiftID, in.OpenedBy, in.DeviceID, in.OpeningFloat, in.OpeningFloat, now); err != nil {
		return "", fmt.Errorf("insert local_cash_shifts: %w", err)
	}

	payloadBytes, _ := json.Marshal(in)
	payloadStr := string(payloadBytes)

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_outbox (event_id, command_type, aggregate_type, aggregate_id, payload_json, idempotency_key, status, created_at)
		VALUES (?, 'cash_shift.opened', 'cash_shift', ?, ?, ?, 'pending', ?)
	`, uuid.NewString(), in.ShiftID, payloadStr, in.IdempotencyKey, now); err != nil {
		return "", err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return "", err
	}
	return in.ShiftID, nil
}

type CashMovementInput struct {
	MovementID     string `json:"movement_id"`
	ShiftID        string `json:"shift_id"`
	TenantID       string `json:"tenant_id"`
	BranchID       string `json:"branch_id"`
	MovementType   string `json:"movement_type"` // paid_in | paid_out | safe_drop
	Amount         string `json:"amount"`
	Reason         string `json:"reason"`
	ActorUserID    string `json:"actor_user_id"`
	ApprovedBy     string `json:"approved_by"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (p *Processor) RecordCashMovement(ctx context.Context, in CashMovementInput) (string, error) {
	if in.MovementID == "" {
		in.MovementID = uuid.NewString()
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = "shift-mov-" + in.MovementID
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	conn, err := p.store.BeginImmediate(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		_ = conn.Close()
	}()

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_cash_movements (id, shift_id, movement_type, amount, reason, approved_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, in.MovementID, in.ShiftID, in.MovementType, in.Amount, in.Reason, in.ApprovedBy, now); err != nil {
		return "", fmt.Errorf("insert local_cash_movements: %w", err)
	}

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_audit_logs (id, tenant_id, branch_id, actor_user_id, approver_user_id, action_type, reason_code, target_type, target_id, amount, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'cash_shift', ?, ?, '{}', ?)
	`, uuid.NewString(), in.TenantID, in.BranchID, in.ActorUserID, in.ApprovedBy, "cash_shift."+in.MovementType, in.Reason, in.ShiftID, in.Amount, now); err != nil {
		return "", err
	}

	payloadBytes, _ := json.Marshal(in)
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_outbox (event_id, command_type, aggregate_type, aggregate_id, payload_json, idempotency_key, status, created_at)
		VALUES (?, 'cash_shift.movement', 'cash_shift', ?, ?, ?, 'pending', ?)
	`, uuid.NewString(), in.ShiftID, string(payloadBytes), in.IdempotencyKey, now); err != nil {
		return "", err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return "", err
	}
	return in.MovementID, nil
}

type CloseShiftInput struct {
	ShiftID           string          `json:"shift_id"`
	ClosedBy          string          `json:"closed_by"`
	CountedCash       string          `json:"counted_cash"`
	DenominationsJSON json.RawMessage `json:"denominations_json"`
	IdempotencyKey    string          `json:"idempotency_key"`
}

type CloseShiftResult struct {
	ShiftID      string `json:"shift_id"`
	OpeningFloat string `json:"opening_float"`
	CashSales    string `json:"cash_sales"`
	CashRefunds  string `json:"cash_refunds"`
	PaidIn       string `json:"paid_in"`
	PaidOut      string `json:"paid_out"`
	SafeDrops    string `json:"safe_drops"`
	ExpectedCash string `json:"expected_cash"`
	CountedCash  string `json:"counted_cash"`
	Variance     string `json:"variance"`
}

func parseCents(s string) int64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(v * 100))
}

func formatCents(c int64) string {
	sign := ""
	if c < 0 {
		sign = "-"
		c = -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

func (p *Processor) CloseCashShift(ctx context.Context, in CloseShiftInput) (*CloseShiftResult, error) {
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = "shift-close-" + in.ShiftID
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	conn, err := p.store.BeginImmediate(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		_ = conn.Close()
	}()

	var openingStr string
	if err := conn.QueryRowContext(ctx, `SELECT opening_float FROM local_cash_shifts WHERE id = ? AND status = 'open'`, in.ShiftID).Scan(&openingStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Sum cash sales & refunds tied to this shift
	var salesCents, refundCents int64
	pRows, err := conn.QueryContext(ctx, `SELECT status, amount FROM local_payments WHERE cash_shift_id = ? AND method = 'cash'`, in.ShiftID)
	if err != nil {
		return nil, err
	}
	for pRows.Next() {
		var st, amt string
		if err := pRows.Scan(&st, &amt); err == nil {
			if st == "captured" {
				salesCents += parseCents(amt)
			} else if st == "refunded" {
				refundCents += parseCents(amt)
			}
		}
	}
	pRows.Close()

	// Sum paid_in, paid_out, safe_drop
	var paidInCents, paidOutCents, safeDropCents int64
	mRows, err := conn.QueryContext(ctx, `SELECT movement_type, amount FROM local_cash_movements WHERE shift_id = ?`, in.ShiftID)
	if err != nil {
		return nil, err
	}
	for mRows.Next() {
		var mt, amt string
		if err := mRows.Scan(&mt, &amt); err == nil {
			switch mt {
			case "paid_in":
				paidInCents += parseCents(amt)
			case "paid_out":
				paidOutCents += parseCents(amt)
			case "safe_drop":
				safeDropCents += parseCents(amt)
			}
		}
	}
	mRows.Close()

	openingCents := parseCents(openingStr)
	expectedCents := openingCents + salesCents - refundCents + paidInCents - paidOutCents - safeDropCents
	countedCents := parseCents(in.CountedCash)
	varianceCents := countedCents - expectedCents

	res := &CloseShiftResult{
		ShiftID:      in.ShiftID,
		OpeningFloat: formatCents(openingCents),
		CashSales:    formatCents(salesCents),
		CashRefunds:  formatCents(refundCents),
		PaidIn:       formatCents(paidInCents),
		PaidOut:      formatCents(paidOutCents),
		SafeDrops:    formatCents(safeDropCents),
		ExpectedCash: formatCents(expectedCents),
		CountedCash:  formatCents(countedCents),
		Variance:     formatCents(varianceCents),
	}

	denomStr := "{}"
	if len(in.DenominationsJSON) > 0 {
		denomStr = string(in.DenominationsJSON)
	}

	if _, err := conn.ExecContext(ctx, `
		UPDATE local_cash_shifts
		SET status = 'closed', closed_by = ?, expected_cash = ?, counted_cash = ?, variance = ?, denominations_json = ?, closed_at = ?
		WHERE id = ?
	`, in.ClosedBy, res.ExpectedCash, res.CountedCash, res.Variance, denomStr, now, in.ShiftID); err != nil {
		return nil, err
	}

	payloadBytes, _ := json.Marshal(res)
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_outbox (event_id, command_type, aggregate_type, aggregate_id, payload_json, idempotency_key, status, created_at)
		VALUES (?, 'cash_shift.closed', 'cash_shift', ?, ?, ?, 'pending', ?)
	`, uuid.NewString(), in.ShiftID, string(payloadBytes), in.IdempotencyKey, now); err != nil {
		return nil, err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return nil, err
	}

	// Enqueue Z-Report ESC/POS print job if spooler is configured
	if p.spooler != nil {
		frame := escpos.RenderShiftZReport(escpos.ShiftZReportData{
			PaperWidth:   escpos.Paper80mm,
			BranchName:   "Hospitality OS Branch",
			ShiftID:      res.ShiftID,
			ClosedBy:     in.ClosedBy,
			ClosedAt:     now,
			OpeningFloat: res.OpeningFloat,
			CashSales:    res.CashSales,
			CashRefunds:  res.CashRefunds,
			PaidIn:       res.PaidIn,
			PaidOut:      res.PaidOut,
			SafeDrops:    res.SafeDrops,
			ExpectedCash: res.ExpectedCash,
			CountedCash:  res.CountedCash,
			Variance:     res.Variance,
		})
		_, _ = p.spooler.EnqueueForStation(ctx, "receipt_counter", "customer_receipt", "zrep-"+in.ShiftID, frame)
	}

	return res, nil
}

type VoidRefundInput struct {
	TenantID               string `json:"tenant_id"`
	BranchID               string `json:"branch_id"`
	OrderID                string `json:"order_id"`
	ActionType             string `json:"action_type"` // void | refund
	Amount                 string `json:"amount"`
	PreserveInventoryUsage bool   `json:"preserve_inventory_usage"`
	RecordWaste            bool   `json:"record_waste"`
	ReasonCode             string `json:"reason_code"`
	ActorUserID            string `json:"actor_user_id"`
	ApproverUserID         string `json:"approver_user_id"`
	IdempotencyKey         string `json:"idempotency_key"`
}

func (p *Processor) VoidOrRefundOrder(ctx context.Context, in VoidRefundInput) (string, error) {
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = "vr-" + uuid.NewString()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	conn, err := p.store.BeginImmediate(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		_ = conn.Close()
	}()

	// Determine if any round was already confirmed in prep -> enforce preserve_inventory_usage = true
	var prepCount int
	_ = conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM local_order_rounds WHERE order_id = ? AND prep_confirmed_at IS NOT NULL`, in.OrderID).Scan(&prepCount)
	if prepCount > 0 {
		in.PreserveInventoryUsage = true
	}

	newStatus := "voided"
	if in.ActionType == "refund" {
		newStatus = "refunded"
	}
	res, err := conn.ExecContext(ctx, `UPDATE local_orders SET status = ?, updated_at = ? WHERE id = ?`, newStatus, now, in.OrderID)
	if err != nil {
		return "", err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return "", ErrNotFound
	}
	if in.ActionType == "refund" {
		_, _ = conn.ExecContext(ctx, `UPDATE local_payments SET status = 'refunded' WHERE order_id = ? AND status = 'captured'`, in.OrderID)
	}

	auditID := uuid.NewString()
	metaBytes, _ := json.Marshal(map[string]any{
		"preserve_inventory_usage": in.PreserveInventoryUsage,
		"record_waste":             in.RecordWaste,
	})
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_audit_logs (id, tenant_id, branch_id, actor_user_id, approver_user_id, action_type, reason_code, target_type, target_id, amount, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'order', ?, ?, ?, ?)
	`, auditID, in.TenantID, in.BranchID, in.ActorUserID, in.ApproverUserID, "order."+newStatus, in.ReasonCode, in.OrderID, in.Amount, string(metaBytes), now); err != nil {
		return "", err
	}

	payloadBytes, _ := json.Marshal(in)
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_outbox (event_id, command_type, aggregate_type, aggregate_id, payload_json, idempotency_key, status, created_at)
		VALUES (?, 'order.voided_or_refunded', 'order', ?, ?, ?, 'pending', ?)
	`, uuid.NewString(), in.OrderID, string(payloadBytes), in.IdempotencyKey, now); err != nil {
		return "", err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return "", err
	}
	return auditID, nil
}
