package procurement

import (
	"fmt"
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type SplitSourcingResult struct {
	RequestedQty       decimal.Decimal `json:"requested_qty"`
	WarehouseAllocated decimal.Decimal `json:"warehouse_allocated_qty"`
	ConfirmedInbound   decimal.Decimal `json:"confirmed_inbound_qty"`
	UncoveredShortage  decimal.Decimal `json:"uncovered_shortage_qty"`
	POOrderQty         decimal.Decimal `json:"po_order_qty"`
}

// RoundUpToPackAndMOQ rounds a positive quantity UP to exact packSize multiples
// and enforces at least minOrderQty (MOQ), using exact shopspring/decimal math.
func RoundUpToPackAndMOQ(qty, packSize, moq decimal.Decimal) decimal.Decimal {
	if qty.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}
	target := qty
	if moq.GreaterThan(target) {
		target = moq
	}
	if packSize.GreaterThan(decimal.Zero) {
		packs := target.Div(packSize).Ceil()
		return packs.Mul(packSize).Round(6)
	}
	return target.Round(6)
}

// EvaluateSplitSourcing splits a branch requisition line between Central Warehouse
// available stock and a supplier Purchase Order draft rounded to pack size & MOQ.
func EvaluateSplitSourcing(
	requestedQty decimal.Decimal,
	warehouseAvailable decimal.Decimal,
	confirmedInbound decimal.Decimal,
	packSize decimal.Decimal,
	moq decimal.Decimal,
) SplitSourcingResult {
	if requestedQty.LessThanOrEqual(decimal.Zero) {
		return SplitSourcingResult{}
	}
	whAvail := warehouseAvailable
	if whAvail.IsNegative() {
		whAvail = decimal.Zero
	}
	inbound := confirmedInbound
	if inbound.IsNegative() {
		inbound = decimal.Zero
	}

	allocated := requestedQty
	if whAvail.LessThan(allocated) {
		allocated = whAvail
	}

	uncovered := requestedQty.Sub(allocated).Sub(inbound)
	if uncovered.IsNegative() {
		uncovered = decimal.Zero
	}

	poQty := RoundUpToPackAndMOQ(uncovered, packSize, moq)

	return SplitSourcingResult{
		RequestedQty:       requestedQty.Round(6),
		WarehouseAllocated: allocated.Round(6),
		ConfirmedInbound:   inbound.Round(6),
		UncoveredShortage:  uncovered.Round(6),
		POOrderQty:         poQty,
	}
}

type Handler struct {
	pool *db.Pool
}

func NewHandler(pool *db.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) ListRequisitions(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	type reqRow struct {
		ID           string `json:"id"`
		BranchID     string `json:"branch_id"`
		Status       string `json:"status"`
		TemplateName string `json:"template_name,omitempty"`
	}
	items := []reqRow{}
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT id, branch_id, status, COALESCE(template_name, '')
			FROM kitchen_requisitions
			ORDER BY created_at DESC
			LIMIT 100
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row reqRow
			if err := rows.Scan(&row.ID, &row.BranchID, &row.Status, &row.TemplateName); err != nil {
				return err
			}
			items = append(items, row)
		}
		return rows.Err()
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "REQUISITIONS_LIST_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items, map[string]int{"count": len(items)})
}

func (h *Handler) SubmitRequisition(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in struct {
		ID           string `json:"id"`
		BranchID     string `json:"branch_id"`
		NeededBy     string `json:"needed_by"`
		TemplateName string `json:"template_name"`
		Notes        string `json:"notes"`
		Lines        []struct {
			IngredientID string          `json:"ingredient_id"`
			RequestedQty decimal.Decimal `json:"requested_qty"`
		} `json:"lines"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}

	var reqID string
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		// Idempotent insert on client-generated stable UUID
		if in.ID != "" {
			if err := tx.QueryRow(r.Context(), `
				INSERT INTO kitchen_requisitions (id, tenant_id, branch_id, requested_by, status, needed_by, template_name, notes)
				VALUES ($1, $2, $3, $4, 'submitted', COALESCE(NULLIF($5, '')::timestamptz, NOW() + interval '4 hours'), NULLIF($6, ''), NULLIF($7, ''))
				ON CONFLICT (id) DO UPDATE SET status = kitchen_requisitions.status
				RETURNING id
			`, in.ID, claims.TenantID, in.BranchID, claims.UserID, in.NeededBy, in.TemplateName, in.Notes).Scan(&reqID); err != nil {
				return err
			}
		} else {
			if err := tx.QueryRow(r.Context(), `
				INSERT INTO kitchen_requisitions (tenant_id, branch_id, requested_by, status, needed_by, template_name, notes)
				VALUES ($1, $2, $3, 'submitted', COALESCE(NULLIF($4, '')::timestamptz, NOW() + interval '4 hours'), NULLIF($5, ''), NULLIF($6, ''))
				RETURNING id
			`, claims.TenantID, in.BranchID, claims.UserID, in.NeededBy, in.TemplateName, in.Notes).Scan(&reqID); err != nil {
				return err
			}
		}

		for _, l := range in.Lines {
			_, err := tx.Exec(r.Context(), `
				INSERT INTO requisition_lines (tenant_id, requisition_id, ingredient_id, requested_qty)
				VALUES ($1, $2, $3, $4::numeric)
			`, claims.TenantID, reqID, l.IngredientID, l.RequestedQty.StringFixed(6))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "REQUISITION_SUBMIT_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"id": reqID, "status": "submitted"}, nil)
}

func (h *Handler) ApproveRequisition(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	reqID := r.PathValue("id")
	var status string
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(), `SELECT status FROM kitchen_requisitions WHERE id = $1 FOR UPDATE`, reqID).Scan(&status); err != nil {
			return err
		}
		if status == "approved" || status == "fulfilled" {
			return nil // Idempotent approval replay
		}
		_, err := tx.Exec(r.Context(), `
			UPDATE kitchen_requisitions SET status = 'approved', approved_by = $2 WHERE id = $1
		`, reqID, claims.UserID)
		status = "approved"
		return err
	})
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "REQUISITION_APPROVE_FAILED", fmt.Sprintf("approve requisition: %v", err), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"id": reqID, "status": status}, nil)
}
