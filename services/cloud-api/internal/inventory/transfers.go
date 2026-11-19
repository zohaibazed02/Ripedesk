package inventory

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

var (
	ErrInvalidTransferTransition = errors.New("invalid warehouse transfer state transition")
	ErrInsufficientAvailable     = errors.New("insufficient available warehouse stock to reserve")
	ErrReceiptExceedsDispatched  = errors.New("received good + damaged quantity exceeds dispatched quantity")
)

const (
	TransferDraft             = "draft"
	TransferApproved          = "approved"
	TransferReserved          = "reserved"
	TransferInTransit         = "in_transit"
	TransferPartiallyReceived = "partially_received"
	TransferReceived          = "received"
	TransferCancelled         = "cancelled"
)

var allowedTransferTransitions = map[string]map[string]bool{
	TransferDraft:             {TransferApproved: true, TransferCancelled: true},
	TransferApproved:          {TransferReserved: true, TransferCancelled: true},
	TransferReserved:          {TransferInTransit: true, TransferCancelled: true},
	TransferInTransit:         {TransferPartiallyReceived: true, TransferReceived: true},
	TransferPartiallyReceived: {TransferReceived: true},
}

// ValidateTransferTransition enforces the strict warehouse transfer lifecycle state machine.
func ValidateTransferTransition(from, to string) error {
	if nexts, ok := allowedTransferTransitions[from]; ok && nexts[to] {
		return nil
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransferTransition, from, to)
}

type TransferReceiptEvaluation struct {
	IngredientID   string          `json:"ingredient_id"`
	DispatchedQty  decimal.Decimal `json:"dispatched_qty"`
	GoodQty        decimal.Decimal `json:"received_good_qty"`
	DamagedQty     decimal.Decimal `json:"damaged_qty"`
	ShortageQty    decimal.Decimal `json:"shortage_qty"`
	PODraftQty     decimal.Decimal `json:"po_draft_qty"`
	NeedsPODraft   bool            `json:"needs_po_draft"`
}

// EvaluateTransferReceiptLine computes exact Decimal shortage and shortage-PO-draft quantities.
func EvaluateTransferReceiptLine(
	ingredientID string,
	dispatchedQty, goodQty, damagedQty decimal.Decimal,
	createPODraftForShortage bool,
) (TransferReceiptEvaluation, error) {
	if goodQty.IsNegative() || damagedQty.IsNegative() || dispatchedQty.LessThanOrEqual(decimal.Zero) {
		return TransferReceiptEvaluation{}, ErrInvalidQuantity
	}
	if goodQty.Add(damagedQty).GreaterThan(dispatchedQty) {
		return TransferReceiptEvaluation{}, ErrReceiptExceedsDispatched
	}
	shortage := dispatchedQty.Sub(goodQty).Sub(damagedQty).Round(6)
	needsPO := createPODraftForShortage && shortage.GreaterThan(decimal.Zero)
	poQty := decimal.Zero
	if needsPO {
		poQty = shortage
	}
	return TransferReceiptEvaluation{
		IngredientID:  ingredientID,
		DispatchedQty: dispatchedQty.Round(6),
		GoodQty:       goodQty.Round(6),
		DamagedQty:    damagedQty.Round(6),
		ShortageQty:   shortage,
		PODraftQty:    poQty,
		NeedsPODraft:  needsPO,
	}, nil
}

// DeterministicTransferIdempotencyKey guarantees replay safety for dispatch/receipt movements.
func DeterministicTransferIdempotencyKey(action, transferID, ingredientID, clientKey string) string {
	return fmt.Sprintf("tr:%s:%s:%s:%s", action, transferID, ingredientID, clientKey)
}

type TransferRecord struct {
	ID             string `json:"id"`
	FromBranchID   string `json:"from_branch_id"`
	ToBranchID     string `json:"to_branch_id"`
	FromLocationID string `json:"from_location_id"`
	ToLocationID   string `json:"to_location_id"`
	Status         string `json:"status"`
}

func (h *Handler) ListTransfers(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	items := []TransferRecord{}
	err := h.repo.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT id, from_branch_id, to_branch_id, from_location_id, to_location_id, status
			FROM warehouse_transfers
			ORDER BY created_at DESC
			LIMIT 100
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var tr TransferRecord
			if err := rows.Scan(&tr.ID, &tr.FromBranchID, &tr.ToBranchID, &tr.FromLocationID, &tr.ToLocationID, &tr.Status); err != nil {
				return err
			}
			items = append(items, tr)
		}
		return rows.Err()
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "TRANSFERS_LIST_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items, map[string]int{"count": len(items)})
}

func (h *Handler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in struct {
		FromBranchID   string `json:"from_branch_id"`
		ToBranchID     string `json:"to_branch_id"`
		FromLocationID string `json:"from_location_id"`
		ToLocationID   string `json:"to_location_id"`
		Lines          []struct {
			IngredientID string          `json:"ingredient_id"`
			Quantity     decimal.Decimal `json:"quantity"`
		} `json:"lines"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}

	var transferID string
	err := h.repo.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO warehouse_transfers (tenant_id, from_branch_id, to_branch_id, from_location_id, to_location_id, status)
			VALUES ($1, $2, $3, $4, $5, 'approved')
			RETURNING id
		`, claims.TenantID, in.FromBranchID, in.ToBranchID, in.FromLocationID, in.ToLocationID).Scan(&transferID); err != nil {
			return err
		}
		for _, line := range in.Lines {
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO transfer_lines (tenant_id, transfer_id, ingredient_id, dispatched_qty)
				VALUES ($1, $2, $3, $4::numeric)
			`, claims.TenantID, transferID, line.IngredientID, line.Quantity.StringFixed(6)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "TRANSFER_CREATE_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"id": transferID, "status": TransferApproved}, nil)
}

func (h *Handler) ReserveTransfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	transferID := r.PathValue("id")
	err := h.repo.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		var status, fromLoc string
		if err := tx.QueryRow(r.Context(), `SELECT status, from_location_id FROM warehouse_transfers WHERE id = $1 FOR UPDATE`, transferID).Scan(&status, &fromLoc); err != nil {
			return err
		}
		if err := ValidateTransferTransition(status, TransferReserved); err != nil {
			return err
		}
		rows, err := tx.Query(r.Context(), `SELECT ingredient_id, dispatched_qty::text FROM transfer_lines WHERE transfer_id = $1`, transferID)
		if err != nil {
			return err
		}
		type lineQty struct {
			ingID string
			qty   decimal.Decimal
		}
		var lines []lineQty
		for rows.Next() {
			var l lineQty
			var qStr string
			if err := rows.Scan(&l.ingID, &qStr); err == nil {
				l.qty, _ = decimal.NewFromString(qStr)
				lines = append(lines, l)
			}
		}
		rows.Close()

		for _, l := range lines {
			var onHandStr, resStr string
			if err := tx.QueryRow(r.Context(), `
				SELECT on_hand_qty::text, reserved_qty::text
				FROM stock_balances
				WHERE stock_location_id = $1 AND ingredient_id = $2
				FOR UPDATE
			`, fromLoc, l.ingID).Scan(&onHandStr, &resStr); err != nil {
				return ErrInsufficientAvailable
			}
			onHand, _ := decimal.NewFromString(onHandStr)
			reserved, _ := decimal.NewFromString(resStr)
			if onHand.Sub(reserved).LessThan(l.qty) {
				return ErrInsufficientAvailable
			}
			if _, err := tx.Exec(r.Context(), `
				UPDATE stock_balances SET reserved_qty = reserved_qty + $3::numeric
				WHERE stock_location_id = $1 AND ingredient_id = $2
			`, fromLoc, l.ingID, l.qty.StringFixed(6)); err != nil {
				return err
			}
		}

		_, err = tx.Exec(r.Context(), `UPDATE warehouse_transfers SET status = 'reserved' WHERE id = $1`, transferID)
		return err
	})
	if err != nil {
		statusCode := http.StatusBadRequest
		if errors.Is(err, ErrInvalidTransferTransition) || errors.Is(err, ErrInsufficientAvailable) {
			statusCode = http.StatusConflict
		}
		httpx.WriteError(w, statusCode, "TRANSFER_RESERVE_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"id": transferID, "status": TransferReserved}, nil)
}

func (h *Handler) DispatchTransfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	transferID := r.PathValue("id")
	var in struct {
		IdempotencyKey string `json:"idempotency_key"`
		LotNumber      string `json:"lot_number"`
	}
	_ = httpx.DecodeJSON(r, &in)

	err := h.repo.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		var status, fromBranch, fromLoc string
		if err := tx.QueryRow(r.Context(), `
			SELECT status, from_branch_id, from_location_id FROM warehouse_transfers WHERE id = $1 FOR UPDATE
		`, transferID).Scan(&status, &fromBranch, &fromLoc); err != nil {
			return err
		}
		if err := ValidateTransferTransition(status, TransferInTransit); err != nil {
			return err
		}
		rows, err := tx.Query(r.Context(), `SELECT ingredient_id, dispatched_qty::text FROM transfer_lines WHERE transfer_id = $1`, transferID)
		if err != nil {
			return err
		}
		type lineQty struct {
			ingID string
			qty   decimal.Decimal
		}
		var lines []lineQty
		for rows.Next() {
			var l lineQty
			var qStr string
			if err := rows.Scan(&l.ingID, &qStr); err == nil {
				l.qty, _ = decimal.NewFromString(qStr)
				lines = append(lines, l)
			}
		}
		rows.Close()

		for _, l := range lines {
			_, _ = tx.Exec(r.Context(), `
				UPDATE stock_balances SET reserved_qty = GREATEST(0, reserved_qty - $3::numeric)
				WHERE stock_location_id = $1 AND ingredient_id = $2
			`, fromLoc, l.ingID, l.qty.StringFixed(6))
			if _, err := PostStockMovementTx(r.Context(), tx, MovementInput{
				TenantID:        claims.TenantID,
				BranchID:        fromBranch,
				StockLocationID: fromLoc,
				IngredientID:    l.ingID,
				MovementType:    "transfer_out",
				QuantityDelta:   l.qty.Neg(),
				UnitCost:        decimal.Zero,
				SourceDocType:   "warehouse_transfer",
				SourceDocID:     transferID,
				IdempotencyKey:  DeterministicTransferIdempotencyKey("dispatch", transferID, l.ingID, in.IdempotencyKey),
				LotNumber:       in.LotNumber,
			}); err != nil {
				return err
			}
		}

		_, err = tx.Exec(r.Context(), `UPDATE warehouse_transfers SET status = 'in_transit', dispatched_at = NOW() WHERE id = $1`, transferID)
		return err
	})
	if err != nil {
		statusCode := http.StatusBadRequest
		if errors.Is(err, ErrInvalidTransferTransition) {
			statusCode = http.StatusConflict
		}
		httpx.WriteError(w, statusCode, "TRANSFER_DISPATCH_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"id": transferID, "status": TransferInTransit}, nil)
}

func (h *Handler) ReceiveTransfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	transferID := r.PathValue("id")
	var in struct {
		IdempotencyKey           string `json:"idempotency_key"`
		CreatePODraftForShortage bool   `json:"create_po_draft_for_shortage"`
		Lines                    []struct {
			IngredientID    string          `json:"ingredient_id"`
			ReceivedGoodQty decimal.Decimal `json:"received_good_qty"`
			DamagedQty      decimal.Decimal `json:"damaged_qty"`
		} `json:"lines"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}

	var evals []TransferReceiptEvaluation
	err := h.repo.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		var status, toBranch, toLoc string
		if err := tx.QueryRow(r.Context(), `
			SELECT status, to_branch_id, to_location_id FROM warehouse_transfers WHERE id = $1 FOR UPDATE
		`, transferID).Scan(&status, &toBranch, &toLoc); err != nil {
			return err
		}
		if err := ValidateTransferTransition(status, TransferReceived); err != nil {
			return err
		}

		for _, line := range in.Lines {
			var dispStr string
			if err := tx.QueryRow(r.Context(), `
				SELECT dispatched_qty::text FROM transfer_lines WHERE transfer_id = $1 AND ingredient_id = $2
			`, transferID, line.IngredientID).Scan(&dispStr); err != nil {
				return err
			}
			dispQty, _ := decimal.NewFromString(dispStr)
			ev, err := EvaluateTransferReceiptLine(line.IngredientID, dispQty, line.ReceivedGoodQty, line.DamagedQty, in.CreatePODraftForShortage)
			if err != nil {
				return err
			}
			evals = append(evals, ev)

			if ev.GoodQty.GreaterThan(decimal.Zero) {
				if _, err := PostStockMovementTx(r.Context(), tx, MovementInput{
					TenantID:        claims.TenantID,
					BranchID:        toBranch,
					StockLocationID: toLoc,
					IngredientID:    line.IngredientID,
					MovementType:    "transfer_in",
					QuantityDelta:   ev.GoodQty,
					SourceDocType:   "warehouse_transfer",
					SourceDocID:     transferID,
					IdempotencyKey:  DeterministicTransferIdempotencyKey("receive_good", transferID, line.IngredientID, in.IdempotencyKey),
				}); err != nil {
					return err
				}
			}
			if ev.DamagedQty.GreaterThan(decimal.Zero) {
				if _, err := PostStockMovementTx(r.Context(), tx, MovementInput{
					TenantID:        claims.TenantID,
					BranchID:        toBranch,
					StockLocationID: toLoc,
					IngredientID:    line.IngredientID,
					MovementType:    "waste",
					QuantityDelta:   ev.DamagedQty.Neg(),
					SourceDocType:   "transit_damage",
					SourceDocID:     transferID,
					IdempotencyKey:  DeterministicTransferIdempotencyKey("receive_dmg", transferID, line.IngredientID, in.IdempotencyKey),
				}); err != nil {
					return err
				}
			}
			_, _ = tx.Exec(r.Context(), `
				UPDATE transfer_lines
				SET received_qty = $3::numeric, damaged_qty = $4::numeric, shortage_qty = $5::numeric
				WHERE transfer_id = $1 AND ingredient_id = $2
			`, transferID, line.IngredientID, ev.GoodQty.StringFixed(6), ev.DamagedQty.StringFixed(6), ev.ShortageQty.StringFixed(6))
		}

		_, err := tx.Exec(r.Context(), `UPDATE warehouse_transfers SET status = 'received', received_at = NOW() WHERE id = $1`, transferID)
		return err
	})
	if err != nil {
		statusCode := http.StatusBadRequest
		if errors.Is(err, ErrInvalidTransferTransition) {
			statusCode = http.StatusConflict
		} else if errors.Is(err, ErrReceiptExceedsDispatched) {
			statusCode = http.StatusUnprocessableEntity
		}
		httpx.WriteError(w, statusCode, "TRANSFER_RECEIVE_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":          transferID,
		"status":      TransferReceived,
		"evaluations": evals,
	}, nil)
}
