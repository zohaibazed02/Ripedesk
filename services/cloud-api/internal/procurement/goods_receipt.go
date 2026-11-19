package procurement

import (
	"fmt"
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/inventory"
	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type GoodsReceiptLineInput struct {
	LineID       string          `json:"line_id"`
	IngredientID string          `json:"ingredient_id"`
	BaseQty      decimal.Decimal `json:"base_qty"`
	UnitCost     decimal.Decimal `json:"unit_cost"`
	LotNumber    string          `json:"lot_number,omitempty"`
}

type CreateGoodsReceiptInput struct {
	ReceiptID          string                  `json:"receipt_id"`
	BranchID           string                  `json:"branch_id"`
	StockLocationID    string                  `json:"stock_location_id"`
	PurchaseOrderID    string                  `json:"purchase_order_id,omitempty"`
	SupplierID         string                  `json:"supplier_id"`
	ReceiptNumber      string                  `json:"receipt_number"`
	IsDirectPurchase   bool                    `json:"is_direct_purchase"`
	AuthorizedByUserID string                  `json:"authorized_by_user_id,omitempty"`
	Lines              []GoodsReceiptLineInput `json:"lines"`
}

// DeterministicGRNIdempotencyKey ensures replaying a GRN never double-credits stock.
func DeterministicGRNIdempotencyKey(grnID, lineID string) string {
	return fmt.Sprintf("receipt:%s:%s", grnID, lineID)
}

func (h *Handler) CreateGoodsReceipt(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in CreateGoodsReceiptInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	if in.IsDirectPurchase && in.AuthorizedByUserID == "" {
		httpx.WriteError(w, http.StatusForbidden, "DIRECT_PURCHASE_AUTH_REQUIRED", "Authorized direct purchase requires authorized_by_user_id", nil)
		return
	}

	var grnID string
	postedLines := 0
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		var poID any = nil
		if in.PurchaseOrderID != "" {
			poID = in.PurchaseOrderID
		}
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO goods_receipts (tenant_id, branch_id, purchase_order_id, supplier_id, receipt_number, received_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (tenant_id, receipt_number) DO UPDATE SET receipt_number = EXCLUDED.receipt_number
			RETURNING id
		`, claims.TenantID, in.BranchID, poID, in.SupplierID, in.ReceiptNumber, claims.UserID).Scan(&grnID); err != nil {
			return err
		}

		for idx, l := range in.Lines {
			lineKey := l.LineID
			if lineKey == "" {
				lineKey = fmt.Sprintf("%d-%s", idx+1, l.IngredientID)
			}
			applied, err := inventory.PostStockMovementTx(r.Context(), tx, inventory.MovementInput{
				TenantID:        claims.TenantID,
				BranchID:        in.BranchID,
				StockLocationID: in.StockLocationID,
				IngredientID:    l.IngredientID,
				MovementType:    "receipt",
				QuantityDelta:   l.BaseQty,
				UnitCost:        l.UnitCost,
				SourceDocType:   "goods_receipt",
				SourceDocID:     grnID,
				IdempotencyKey:  DeterministicGRNIdempotencyKey(grnID, lineKey),
				LotNumber:       l.LotNumber,
			})
			if err != nil {
				return err
			}
			if applied {
				postedLines++
			}
		}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "GRN_CREATE_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":           grnID,
		"posted_lines": postedLines,
	}, nil)
}
