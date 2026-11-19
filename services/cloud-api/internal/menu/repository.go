package menu

import (
	"context"

	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type Category struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type MenuItem struct {
	ID              string          `json:"id"`
	BranchID        *string         `json:"branch_id,omitempty"`
	CategoryID      string          `json:"category_id"`
	SKU             string          `json:"sku"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	BasePrice       decimal.Decimal `json:"base_price"`
	TaxRatePercent  decimal.Decimal `json:"tax_rate_percent"`
	PrepStation     string          `json:"prep_station"`
	IsAvailable     bool            `json:"is_available"`
	ModifierGroupID []string        `json:"modifier_group_ids"`
}

type ModifierGroup struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	MinSelections int        `json:"min_selections"`
	MaxSelections int        `json:"max_selections"`
	Modifiers     []Modifier `json:"modifiers"`
}

type Modifier struct {
	ID              string          `json:"id"`
	ModifierGroupID string          `json:"modifier_group_id"`
	Name            string          `json:"name"`
	PriceDelta      decimal.Decimal `json:"price_delta"`
	IsAvailable     bool            `json:"is_available"`
}

type Floor struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	SortOrder int           `json:"sort_order"`
	Tables    []DiningTable `json:"tables"`
}

type DiningTable struct {
	ID        string `json:"id"`
	FloorID   string `json:"floor_id"`
	TableCode string `json:"table_code"`
	Capacity  int    `json:"capacity"`
	PosX      int    `json:"pos_x"`
	PosY      int    `json:"pos_y"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Shape     string `json:"shape"`
	Status    string `json:"status"`
}

type BranchCatalog struct {
	BranchID       string          `json:"branch_id"`
	Categories     []Category      `json:"categories"`
	MenuItems      []MenuItem      `json:"menu_items"`
	ModifierGroups []ModifierGroup `json:"modifier_groups"`
	Floors         []Floor         `json:"floors"`
	ETag           string          `json:"etag"`
}

type UpsertMenuItemInput struct {
	CategoryID     string          `json:"category_id"`
	SKU            string          `json:"sku"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	BasePrice      decimal.Decimal `json:"base_price"`
	TaxRatePercent decimal.Decimal `json:"tax_rate_percent"`
	PrepStation    string          `json:"prep_station"`
	IsAvailable    bool            `json:"is_available"`
}

type Repository struct {
	pool *db.Pool
}

func NewRepository(pool *db.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetBranchCatalog(ctx context.Context, tenantID, branchID string) (*BranchCatalog, error) {
	cat := &BranchCatalog{
		BranchID:       branchID,
		Categories:     []Category{},
		MenuItems:      []MenuItem{},
		ModifierGroups: []ModifierGroup{},
		Floors:         []Floor{},
	}

	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		// 1. Categories
		cRows, err := tx.Query(ctx, `
			SELECT id, name, sort_order
			FROM categories
			WHERE is_active = TRUE
			ORDER BY sort_order ASC, name ASC
		`)
		if err != nil {
			return err
		}
		for cRows.Next() {
			var c Category
			if err := cRows.Scan(&c.ID, &c.Name, &c.SortOrder); err != nil {
				cRows.Close()
				return err
			}
			cat.Categories = append(cat.Categories, c)
		}
		cRows.Close()

		// 2. Menu items (tenant-wide or branch-specific override)
		mRows, err := tx.Query(ctx, `
			SELECT id, branch_id, category_id, sku, name, COALESCE(description, ''),
			       base_price::text, tax_rate_percent::text, prep_station, is_available
			FROM menu_items
			WHERE (branch_id IS NULL OR branch_id = $1)
			ORDER BY name ASC
		`, branchID)
		if err != nil {
			return err
		}
		itemIndex := map[string]int{}
		for mRows.Next() {
			var item MenuItem
			var priceStr, taxStr string
			if err := mRows.Scan(
				&item.ID, &item.BranchID, &item.CategoryID, &item.SKU, &item.Name,
				&item.Description, &priceStr, &taxStr, &item.PrepStation, &item.IsAvailable,
			); err != nil {
				mRows.Close()
				return err
			}
			item.BasePrice, _ = decimal.NewFromString(priceStr)
			item.TaxRatePercent, _ = decimal.NewFromString(taxStr)
			item.ModifierGroupID = []string{}
			itemIndex[item.ID] = len(cat.MenuItems)
			cat.MenuItems = append(cat.MenuItems, item)
		}
		mRows.Close()

		// 3. Item -> Modifier Group links
		lRows, err := tx.Query(ctx, `
			SELECT menu_item_id, modifier_group_id
			FROM item_modifier_groups
			ORDER BY sort_order ASC
		`)
		if err != nil {
			return err
		}
		for lRows.Next() {
			var itemID, groupID string
			if err := lRows.Scan(&itemID, &groupID); err != nil {
				lRows.Close()
				return err
			}
			if idx, ok := itemIndex[itemID]; ok {
				cat.MenuItems[idx].ModifierGroupID = append(cat.MenuItems[idx].ModifierGroupID, groupID)
			}
		}
		lRows.Close()

		// 4. Modifier groups & modifiers
		gRows, err := tx.Query(ctx, `
			SELECT id, name, min_selections, max_selections
			FROM modifier_groups
			ORDER BY name ASC
		`)
		if err != nil {
			return err
		}
		groupIndex := map[string]int{}
		for gRows.Next() {
			var g ModifierGroup
			if err := gRows.Scan(&g.ID, &g.Name, &g.MinSelections, &g.MaxSelections); err != nil {
				gRows.Close()
				return err
			}
			g.Modifiers = []Modifier{}
			groupIndex[g.ID] = len(cat.ModifierGroups)
			cat.ModifierGroups = append(cat.ModifierGroups, g)
		}
		gRows.Close()

		modRows, err := tx.Query(ctx, `
			SELECT id, modifier_group_id, name, price_delta::text, is_available
			FROM modifiers
			ORDER BY name ASC
		`)
		if err != nil {
			return err
		}
		for modRows.Next() {
			var mod Modifier
			var deltaStr string
			if err := modRows.Scan(&mod.ID, &mod.ModifierGroupID, &mod.Name, &deltaStr, &mod.IsAvailable); err != nil {
				modRows.Close()
				return err
			}
			mod.PriceDelta, _ = decimal.NewFromString(deltaStr)
			if idx, ok := groupIndex[mod.ModifierGroupID]; ok {
				cat.ModifierGroups[idx].Modifiers = append(cat.ModifierGroups[idx].Modifiers, mod)
			}
		}
		modRows.Close()

		// 5. Floors & Dining Tables for the branch
		fRows, err := tx.Query(ctx, `
			SELECT id, name, sort_order
			FROM floors
			WHERE branch_id = $1
			ORDER BY sort_order ASC, name ASC
		`, branchID)
		if err != nil {
			return err
		}
		floorIndex := map[string]int{}
		for fRows.Next() {
			var f Floor
			if err := fRows.Scan(&f.ID, &f.Name, &f.SortOrder); err != nil {
				fRows.Close()
				return err
			}
			f.Tables = []DiningTable{}
			floorIndex[f.ID] = len(cat.Floors)
			cat.Floors = append(cat.Floors, f)
		}
		fRows.Close()

		tRows, err := tx.Query(ctx, `
			SELECT id, floor_id, table_code, capacity, pos_x, pos_y, width, height, shape, status
			FROM dining_tables
			WHERE branch_id = $1
			ORDER BY table_code ASC
		`, branchID)
		if err != nil {
			return err
		}
		for tRows.Next() {
			var t DiningTable
			if err := tRows.Scan(
				&t.ID, &t.FloorID, &t.TableCode, &t.Capacity,
				&t.PosX, &t.PosY, &t.Width, &t.Height, &t.Shape, &t.Status,
			); err != nil {
				tRows.Close()
				return err
			}
			if idx, ok := floorIndex[t.FloorID]; ok {
				cat.Floors[idx].Tables = append(cat.Floors[idx].Tables, t)
			}
		}
		tRows.Close()

		return nil
	})
	if err != nil {
		return nil, err
	}
	return cat, nil
}

func (r *Repository) UpsertMenuItem(ctx context.Context, tenantID, branchID string, in UpsertMenuItemInput) (*MenuItem, error) {
	var item MenuItem
	var priceStr, taxStr string
	err := r.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO menu_items (
				tenant_id, branch_id, category_id, sku, name, description,
				base_price, tax_rate_percent, prep_station, is_available
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7::numeric, $8::numeric, $9, $10)
			ON CONFLICT (tenant_id, sku)
			DO UPDATE SET
				branch_id = EXCLUDED.branch_id,
				category_id = EXCLUDED.category_id,
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				base_price = EXCLUDED.base_price,
				tax_rate_percent = EXCLUDED.tax_rate_percent,
				prep_station = EXCLUDED.prep_station,
				is_available = EXCLUDED.is_available
			RETURNING id, branch_id, category_id, sku, name, COALESCE(description, ''),
			          base_price::text, tax_rate_percent::text, prep_station, is_available
		`,
			tenantID, branchID, in.CategoryID, in.SKU, in.Name, in.Description,
			in.BasePrice.StringFixed(4), in.TaxRatePercent.StringFixed(3), in.PrepStation, in.IsAvailable,
		).Scan(
			&item.ID, &item.BranchID, &item.CategoryID, &item.SKU, &item.Name,
			&item.Description, &priceStr, &taxStr, &item.PrepStation, &item.IsAvailable,
		)
	})
	if err != nil {
		return nil, err
	}
	item.BasePrice, _ = decimal.NewFromString(priceStr)
	item.TaxRatePercent, _ = decimal.NewFromString(taxStr)
	item.ModifierGroupID = []string{}
	return &item, nil
}
