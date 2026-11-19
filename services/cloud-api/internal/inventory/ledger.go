package inventory

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type MovementInput struct {
	TenantID        string          `json:"tenant_id"`
	BranchID        string          `json:"branch_id"`
	StockLocationID string          `json:"stock_location_id"`
	IngredientID    string          `json:"ingredient_id"`
	MovementType    string          `json:"movement_type"` // receipt | prep_consumption | waste | count_adjustment | transfer_out | transfer_in | production_out | production_in
	QuantityDelta   decimal.Decimal `json:"quantity_delta"`
	UnitCost        decimal.Decimal `json:"unit_cost"`
	SourceDocType   string          `json:"source_doc_type"`
	SourceDocID     string          `json:"source_doc_id"`
	IdempotencyKey  string          `json:"idempotency_key"`
	LotNumber       string          `json:"lot_number,omitempty"`
	ExpiresAt       *time.Time      `json:"expires_at,omitempty"`
}

// ComputeWeightedAvgCost calculates the new weighted average unit cost on positive stock inflow.
func ComputeWeightedAvgCost(
	currentQty decimal.Decimal,
	currentAvgCost decimal.Decimal,
	inflowQty decimal.Decimal,
	inflowUnitCost decimal.Decimal,
) decimal.Decimal {
	if inflowQty.LessThanOrEqual(decimal.Zero) {
		return currentAvgCost.Round(4)
	}
	if currentQty.LessThanOrEqual(decimal.Zero) {
		return inflowUnitCost.Round(4)
	}
	totalQty := currentQty.Add(inflowQty)
	totalVal := currentQty.Mul(currentAvgCost).Add(inflowQty.Mul(inflowUnitCost))
	return totalVal.Div(totalQty).Round(4)
}

// PostStockMovementTx atomically appends a stock_movements row guarded by UNIQUE(tenant_id, idempotency_key)
// and updates stock_balances in the same PostgreSQL RLS transaction.
// Returns (true, nil) if applied, or (false, nil) if already applied (idempotent duplicate).
func PostStockMovementTx(ctx context.Context, tx pgx.Tx, in MovementInput) (bool, error) {
	if in.QuantityDelta.IsZero() {
		return false, ErrInvalidQuantity
	}
	if in.IdempotencyKey == "" {
		return false, fmt.Errorf("idempotency_key is required for stock movement")
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO stock_movements (
			tenant_id, branch_id, stock_location_id, ingredient_id,
			movement_type, quantity_delta, unit_cost,
			source_doc_type, source_doc_id, idempotency_key, lot_number, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6::numeric, $7::numeric, $8, $9, $10, NULLIF($11, ''), $12)
		ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
	`,
		in.TenantID, in.BranchID, in.StockLocationID, in.IngredientID,
		in.MovementType, in.QuantityDelta.StringFixed(6), in.UnitCost.StringFixed(4),
		in.SourceDocType, in.SourceDocID, in.IdempotencyKey, in.LotNumber, in.ExpiresAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert stock_movements: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Idempotent replay — already posted, zero double-deduction
		return false, nil
	}

	// Lock & read current balance (if any) to update on_hand_qty and weighted average cost
	var curQtyStr, curCostStr string
	err = tx.QueryRow(ctx, `
		SELECT on_hand_qty::text, avg_unit_cost::text
		FROM stock_balances
		WHERE stock_location_id = $1 AND ingredient_id = $2
		FOR UPDATE
	`, in.StockLocationID, in.IngredientID).Scan(&curQtyStr, &curCostStr)

	if err == pgx.ErrNoRows {
		newCost := in.UnitCost.Round(4)
		_, err = tx.Exec(ctx, `
			INSERT INTO stock_balances (tenant_id, branch_id, stock_location_id, ingredient_id, on_hand_qty, avg_unit_cost)
			VALUES ($1, $2, $3, $4, $5::numeric, $6::numeric)
		`, in.TenantID, in.BranchID, in.StockLocationID, in.IngredientID, in.QuantityDelta.StringFixed(6), newCost.StringFixed(4))
		return err == nil, err
	}
	if err != nil {
		return false, err
	}

	curQty, _ := decimal.NewFromString(curQtyStr)
	curCost, _ := decimal.NewFromString(curCostStr)
	nextQty := curQty.Add(in.QuantityDelta)
	nextCost := curCost
	if in.QuantityDelta.GreaterThan(decimal.Zero) && in.UnitCost.GreaterThan(decimal.Zero) {
		nextCost = ComputeWeightedAvgCost(curQty, curCost, in.QuantityDelta, in.UnitCost)
	}

	_, err = tx.Exec(ctx, `
		UPDATE stock_balances
		SET on_hand_qty = $3::numeric, avg_unit_cost = $4::numeric
		WHERE stock_location_id = $1 AND ingredient_id = $2
	`, in.StockLocationID, in.IngredientID, nextQty.StringFixed(6), nextCost.StringFixed(4))
	return err == nil, err
}

// PostOrderPrepConsumption deducts exact base-unit ingredients for a preparation-confirmed order item
// using deterministic idempotency keys: prep:<order_id>:<order_item_id>:<ingredient_id>.
func PostOrderPrepConsumption(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, branchID, locationID, orderID, orderItemID string,
	explosion OrderLineExplosion,
) (int, error) {
	posted := 0
	for _, c := range explosion.Consumptions {
		applied, err := PostStockMovementTx(ctx, tx, MovementInput{
			TenantID:        tenantID,
			BranchID:        branchID,
			StockLocationID: locationID,
			IngredientID:    c.IngredientID,
			MovementType:    "prep_consumption",
			QuantityDelta:   c.TotalBaseQty.Neg(),
			UnitCost:        c.UnitCost,
			SourceDocType:   "order_item",
			SourceDocID:     orderItemID,
			IdempotencyKey:  fmt.Sprintf("prep:%s:%s:%s", orderID, orderItemID, c.IngredientID),
		})
		if err != nil {
			return posted, err
		}
		if applied {
			posted++
		}
	}
	return posted, nil
}
