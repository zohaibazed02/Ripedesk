package inventory

import (
	"context"
	"fmt"

	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type Repository struct {
	pool *db.Pool
}

func NewRepository(pool *db.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListIngredients(ctx context.Context, tenantID string) ([]Ingredient, error) {
	items := []Ingredient{}
	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, sku, name, base_unit, purchasing_unit, conversion_factor::text,
			       track_lot_expiry, low_stock_threshold::text, valuation_cost::text, is_active
			FROM ingredients
			ORDER BY name ASC
		`)
		if err != nil {
			return err
		}
		idxMap := map[string]int{}
		for rows.Next() {
			var ing Ingredient
			var cfStr, lowStr, costStr string
			if err := rows.Scan(
				&ing.ID, &ing.SKU, &ing.Name, &ing.BaseUnit, &ing.PurchasingUnit,
				&cfStr, &ing.TrackLotExpiry, &lowStr, &costStr, &ing.IsActive,
			); err != nil {
				rows.Close()
				return err
			}
			ing.ConversionFactor, _ = decimal.NewFromString(cfStr)
			ing.LowStockThreshold, _ = decimal.NewFromString(lowStr)
			ing.ValuationCost, _ = decimal.NewFromString(costStr)
			ing.Conversions = []UnitConversion{}
			idxMap[ing.ID] = len(items)
			items = append(items, ing)
		}
		rows.Close()

		cRows, err := tx.Query(ctx, `
			SELECT id, ingredient_id, from_unit, to_unit, multiplier::text
			FROM unit_conversions
			ORDER BY from_unit ASC
		`)
		if err != nil {
			return err
		}
		defer cRows.Close()
		for cRows.Next() {
			var uc UnitConversion
			var mStr string
			if err := cRows.Scan(&uc.ID, &uc.IngredientID, &uc.FromUnit, &uc.ToUnit, &mStr); err != nil {
				return err
			}
			uc.Multiplier, _ = decimal.NewFromString(mStr)
			if idx, ok := idxMap[uc.IngredientID]; ok {
				items[idx].Conversions = append(items[idx].Conversions, uc)
			}
		}
		return cRows.Err()
	})
	return items, err
}

func (r *Repository) UpsertIngredient(ctx context.Context, tenantID string, in Ingredient) (*Ingredient, error) {
	var out Ingredient
	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var cfStr, lowStr, costStr string
		if err := tx.QueryRow(ctx, `
			INSERT INTO ingredients (
				tenant_id, sku, name, base_unit, purchasing_unit, conversion_factor,
				track_lot_expiry, low_stock_threshold, valuation_cost, is_active
			)
			VALUES ($1, $2, $3, $4, $5, $6::numeric, $7, $8::numeric, $9::numeric, $10)
			ON CONFLICT (tenant_id, sku)
			DO UPDATE SET
				name = EXCLUDED.name,
				base_unit = EXCLUDED.base_unit,
				purchasing_unit = EXCLUDED.purchasing_unit,
				conversion_factor = EXCLUDED.conversion_factor,
				track_lot_expiry = EXCLUDED.track_lot_expiry,
				low_stock_threshold = EXCLUDED.low_stock_threshold,
				valuation_cost = EXCLUDED.valuation_cost,
				is_active = EXCLUDED.is_active
			RETURNING id, sku, name, base_unit, purchasing_unit, conversion_factor::text,
			          track_lot_expiry, low_stock_threshold::text, valuation_cost::text, is_active
		`,
			tenantID, in.SKU, in.Name, in.BaseUnit, in.PurchasingUnit,
			in.ConversionFactor.StringFixed(6), in.TrackLotExpiry,
			in.LowStockThreshold.StringFixed(6), in.ValuationCost.StringFixed(4), in.IsActive,
		).Scan(
			&out.ID, &out.SKU, &out.Name, &out.BaseUnit, &out.PurchasingUnit,
			&cfStr, &out.TrackLotExpiry, &lowStr, &costStr, &out.IsActive,
		); err != nil {
			return err
		}
		out.ConversionFactor, _ = decimal.NewFromString(cfStr)
		out.LowStockThreshold, _ = decimal.NewFromString(lowStr)
		out.ValuationCost, _ = decimal.NewFromString(costStr)

		for _, uc := range in.Conversions {
			_, err := tx.Exec(ctx, `
				INSERT INTO unit_conversions (tenant_id, ingredient_id, from_unit, to_unit, multiplier)
				VALUES ($1, $2, $3, $4, $5::numeric)
				ON CONFLICT (tenant_id, ingredient_id, from_unit, to_unit)
				DO UPDATE SET multiplier = EXCLUDED.multiplier
			`, tenantID, out.ID, uc.FromUnit, uc.ToUnit, uc.Multiplier.StringFixed(6))
			if err != nil {
				return err
			}
		}
		out.Conversions = in.Conversions
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) PublishRecipeVersion(ctx context.Context, tenantID string, in RecipeVersion) (*RecipeVersion, error) {
	if (in.MenuItemID == nil && in.ModifierID == nil) || (in.MenuItemID != nil && in.ModifierID != nil) {
		return nil, fmt.Errorf("exactly one of menu_item_id or modifier_id must be provided")
	}
	var out RecipeVersion
	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var maxVer int
		if in.MenuItemID != nil {
			_ = tx.QueryRow(ctx, `SELECT COALESCE(MAX(version_number), 0) FROM recipe_versions WHERE menu_item_id = $1`, *in.MenuItemID).Scan(&maxVer)
			_, _ = tx.Exec(ctx, `UPDATE recipe_versions SET is_active = FALSE WHERE menu_item_id = $1 AND is_active = TRUE`, *in.MenuItemID)
		} else {
			_ = tx.QueryRow(ctx, `SELECT COALESCE(MAX(version_number), 0) FROM recipe_versions WHERE modifier_id = $1`, *in.ModifierID).Scan(&maxVer)
			_, _ = tx.Exec(ctx, `UPDATE recipe_versions SET is_active = FALSE WHERE modifier_id = $1 AND is_active = TRUE`, *in.ModifierID)
		}

		nextVer := maxVer + 1
		yield := in.YieldQuantity
		if yield.LessThanOrEqual(decimal.Zero) {
			yield = decimal.NewFromInt(1)
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO recipe_versions (tenant_id, menu_item_id, modifier_id, version_number, is_active, yield_quantity)
			VALUES ($1, $2, $3, $4, TRUE, $5::numeric)
			RETURNING id, menu_item_id, modifier_id, version_number, is_active, effective_from
		`, tenantID, in.MenuItemID, in.ModifierID, nextVer, yield.StringFixed(6)).Scan(
			&out.ID, &out.MenuItemID, &out.ModifierID, &out.VersionNumber, &out.IsActive, &out.EffectiveFrom,
		); err != nil {
			return err
		}
		out.YieldQuantity = yield

		for _, line := range in.Lines {
			_, err := tx.Exec(ctx, `
				INSERT INTO recipe_lines (tenant_id, recipe_version_id, ingredient_id, quantity_base_unit, waste_factor_pct)
				VALUES ($1, $2, $3, $4::numeric, $5::numeric)
			`, tenantID, out.ID, line.IngredientID, line.QuantityBaseUnit.StringFixed(6), line.WasteFactorPct.StringFixed(3))
			if err != nil {
				return err
			}
		}
		out.Lines = in.Lines
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Repository) ListRecipeVersions(ctx context.Context, tenantID, targetType, targetID string) ([]RecipeVersion, error) {
	versions := []RecipeVersion{}
	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		col := "menu_item_id"
		if targetType == "modifier" {
			col = "modifier_id"
		}
		rows, err := tx.Query(ctx, fmt.Sprintf(`
			SELECT id, menu_item_id, modifier_id, version_number, is_active, yield_quantity::text, effective_from
			FROM recipe_versions
			WHERE %s = $1
			ORDER BY version_number DESC
		`, col), targetID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var rv RecipeVersion
			var yStr string
			if err := rows.Scan(&rv.ID, &rv.MenuItemID, &rv.ModifierID, &rv.VersionNumber, &rv.IsActive, &yStr, &rv.EffectiveFrom); err != nil {
				rows.Close()
				return err
			}
			rv.YieldQuantity, _ = decimal.NewFromString(yStr)
			rv.Lines = []RecipeLine{}
			versions = append(versions, rv)
		}
		rows.Close()

		for i := range versions {
			lRows, err := tx.Query(ctx, `
				SELECT rl.id, rl.ingredient_id, ing.name, ing.base_unit, rl.quantity_base_unit::text, rl.waste_factor_pct::text
				FROM recipe_lines rl
				JOIN ingredients ing ON ing.tenant_id = rl.tenant_id AND ing.id = rl.ingredient_id
				WHERE rl.recipe_version_id = $1
			`, versions[i].ID)
			if err != nil {
				return err
			}
			for lRows.Next() {
				var l RecipeLine
				var qStr, wStr string
				if err := lRows.Scan(&l.ID, &l.IngredientID, &l.IngredientName, &l.BaseUnit, &qStr, &wStr); err != nil {
					lRows.Close()
					return err
				}
				l.QuantityBaseUnit, _ = decimal.NewFromString(qStr)
				l.WasteFactorPct, _ = decimal.NewFromString(wStr)
				versions[i].Lines = append(versions[i].Lines, l)
			}
			lRows.Close()
		}
		return nil
	})
	return versions, err
}
