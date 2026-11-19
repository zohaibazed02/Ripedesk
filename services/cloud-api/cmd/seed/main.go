package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func hashSecret(salt, raw string) string {
	sum := sha256.Sum256([]byte(salt + ":" + raw))
	return hex.EncodeToString(sum[:])
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://hospitality:hospitality_dev_secret@localhost:5432/hospitality_cloud?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Seed Tenant: Artisan Roast & Bistro
	var tenantID string
	err = tx.QueryRow(ctx, `
		INSERT INTO tenants (name, slug, currency_code, timezone, status, entitlements)
		VALUES ($1, $2, $3, $4, 'active', $5::jsonb)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`,
		"Artisan Roast & Bistro",
		"artisan-roast-bistro",
		"PKR",
		"Asia/Karachi",
		`{"kds":true,"warehouse":true,"ocr_bills":true,"rider_tracking":true,"ai_agent":true}`,
	).Scan(&tenantID)
	if err != nil {
		log.Fatalf("seed tenant: %v", err)
	}

	// Set RLS tenant context for the remainder of the transaction
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant_id', $1, true)`, tenantID); err != nil {
		log.Fatalf("set rls tenant context: %v", err)
	}

	// 2. Seed 2 Pilot Branches
	var coffeeBranchID, dineInBranchID string
	err = tx.QueryRow(ctx, `
		INSERT INTO branches (tenant_id, name, code, branch_type, default_order_mode, config)
		VALUES ($1, 'F-7 Coffee Bar', 'ARB-F7', 'coffee_shop', 'quick_sale', '{"kds_stations":["barista","pastry"]}'::jsonb)
		ON CONFLICT (tenant_id, code) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, tenantID).Scan(&coffeeBranchID)
	if err != nil {
		log.Fatalf("seed F-7 Coffee Bar: %v", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO branches (tenant_id, name, code, branch_type, default_order_mode, config)
		VALUES ($1, 'G-9 Flagship Dine-In', 'ARB-G9', 'dine_in', 'dine_in', '{"kds_stations":["barista","hot_kitchen","cold_prep"]}'::jsonb)
		ON CONFLICT (tenant_id, code) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, tenantID).Scan(&dineInBranchID)
	if err != nil {
		log.Fatalf("seed G-9 Flagship Dine-In: %v", err)
	}

	// 3. Seed Floor & Dining Tables for G-9 Flagship Dine-In
	var mainFloorID string
	err = tx.QueryRow(ctx, `
		INSERT INTO floors (tenant_id, branch_id, name, sort_order)
		VALUES ($1, $2, 'Main Dining Hall', 1)
		RETURNING id
	`, tenantID, dineInBranchID).Scan(&mainFloorID)
	if err != nil {
		log.Fatalf("seed floor: %v", err)
	}

	tables := []struct {
		code  string
		cap   int
		x, y  int
		shape string
	}{
		{"T-01", 2, 40, 40, "circle"},
		{"T-02", 4, 160, 40, "rectangle"},
		{"T-03", 4, 280, 40, "rectangle"},
		{"B-01", 6, 40, 180, "booth"},
	}
	for _, t := range tables {
		_, err = tx.Exec(ctx, `
			INSERT INTO dining_tables (tenant_id, branch_id, floor_id, table_code, capacity, pos_x, pos_y, shape)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (tenant_id, branch_id, table_code) DO NOTHING
		`, tenantID, dineInBranchID, mainFloorID, t.code, t.cap, t.x, t.y, t.shape)
		if err != nil {
			log.Fatalf("seed table %s: %v", t.code, err)
		}
	}

	// 4. Seed Roles & Staff Users with PINs
	roleDefs := []struct {
		code        string
		displayName string
		canOverride bool
	}{
		{"owner", "Owner", true},
		{"branch_manager", "Branch Manager", true},
		{"cashier", "Cashier / Barista", false},
		{"waiter", "Floor Waiter", false},
		{"kitchen_lead", "Kitchen Lead", false},
		{"warehouse_clerk", "Warehouse Clerk", false},
		{"accountant", "Accountant", false},
	}
	roleIDs := make(map[string]string, len(roleDefs))
	for _, r := range roleDefs {
		var roleID string
		err = tx.QueryRow(ctx, `
			INSERT INTO roles (tenant_id, code, display_name, can_override)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_id, code) DO UPDATE SET display_name = EXCLUDED.display_name
			RETURNING id
		`, tenantID, r.code, r.displayName, r.canOverride).Scan(&roleID)
		if err != nil {
			log.Fatalf("seed role %s: %v", r.code, err)
		}
		roleIDs[r.code] = roleID
	}

	staff := []struct {
		name     string
		email    string
		roleCode string
		pin      string
		branchID string
	}{
		{"Zainab Tariq", "owner@artisanroast.pk", "owner", "1111", coffeeBranchID},
		{"Bilal Ahmed", "manager.f7@artisanroast.pk", "branch_manager", "2222", coffeeBranchID},
		{"Ayesha Khan", "barista.f7@artisanroast.pk", "cashier", "3333", coffeeBranchID},
		{"Hamza Raza", "waiter.g9@artisanroast.pk", "waiter", "4444", dineInBranchID},
		{"Chef Usman", "kitchen.g9@artisanroast.pk", "kitchen_lead", "5555", dineInBranchID},
	}
	for _, u := range staff {
		var userID string
		err = tx.QueryRow(ctx, `
			INSERT INTO users (tenant_id, role_id, full_name, email, password_hash, pin_hash)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (tenant_id, email) DO UPDATE SET full_name = EXCLUDED.full_name
			RETURNING id
		`,
			tenantID,
			roleIDs[u.roleCode],
			u.name,
			u.email,
			hashSecret(tenantID, "DevPass#2026"),
			hashSecret(tenantID, u.pin),
		).Scan(&userID)
		if err != nil {
			log.Fatalf("seed user %s: %v", u.email, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO user_branch_assignments (tenant_id, user_id, branch_id, is_primary)
			VALUES ($1, $2, $3, true)
			ON CONFLICT (tenant_id, user_id, branch_id) DO NOTHING
		`, tenantID, userID, u.branchID)
		if err != nil {
			log.Fatalf("assign user %s to branch: %v", u.email, err)
		}
	}

	// 5. Seed Menu Category, "Large Oat Latte", and "Extra Shot" Modifier
	var coffeeCatID string
	err = tx.QueryRow(ctx, `
		INSERT INTO categories (tenant_id, name, sort_order)
		VALUES ($1, 'Specialty Coffee', 1)
		ON CONFLICT (tenant_id, name) DO UPDATE SET sort_order = EXCLUDED.sort_order
		RETURNING id
	`, tenantID).Scan(&coffeeCatID)
	if err != nil {
		log.Fatalf("seed category: %v", err)
	}

	var oatLatteID string
	err = tx.QueryRow(ctx, `
		INSERT INTO menu_items (tenant_id, category_id, sku, name, description, base_price, tax_rate_percent, prep_station)
		VALUES ($1, $2, 'COF-OAT-LG', 'Large Oat Latte', 'Double ristretto espresso with steamed barista oat milk (16oz)', 950.0000, 16.000, 'barista')
		ON CONFLICT (tenant_id, sku) DO UPDATE SET base_price = EXCLUDED.base_price
		RETURNING id
	`, tenantID, coffeeCatID).Scan(&oatLatteID)
	if err != nil {
		log.Fatalf("seed Large Oat Latte: %v", err)
	}

	var modGroupID, extraShotModID string
	err = tx.QueryRow(ctx, `
		INSERT INTO modifier_groups (tenant_id, name, min_selections, max_selections)
		VALUES ($1, 'Espresso Add-Ons', 0, 2)
		RETURNING id
	`, tenantID).Scan(&modGroupID)
	if err != nil {
		log.Fatalf("seed modifier_group: %v", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO modifiers (tenant_id, modifier_group_id, name, price_delta)
		VALUES ($1, $2, 'Extra Shot', 220.0000)
		RETURNING id
	`, tenantID, modGroupID).Scan(&extraShotModID)
	if err != nil {
		log.Fatalf("seed Extra Shot modifier: %v", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO item_modifier_groups (tenant_id, menu_item_id, modifier_group_id, sort_order)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT (tenant_id, menu_item_id, modifier_group_id) DO NOTHING
	`, tenantID, oatLatteID, modGroupID)
	if err != nil {
		log.Fatalf("link modifier group: %v", err)
	}

	// 6. Seed Ingredients, Recipes (Item + Modifier), and Warehouse Stock
	ingDefs := []struct {
		sku       string
		name      string
		baseUnit  string
		purchUnit string
		factor    float64
		threshold float64
		cost      float64
		initQty   float64
	}{
		{"ING-BEANS-ESP", "House Espresso Beans", "g", "kg", 1000, 2000, 4.50, 25000},
		{"ING-MILK-OAT", "Barista Oat Milk", "ml", "L", 1000, 5000, 0.85, 60000},
		{"ING-CUP-16OZ", "16oz Double-Wall Paper Cup", "pcs", "box", 500, 100, 22.00, 1500},
		{"ING-LID-16OZ", "16oz Sip-Through Lid", "pcs", "box", 500, 100, 8.50, 1500},
	}
	ingIDs := make(map[string]string, len(ingDefs))
	for _, ing := range ingDefs {
		var ingID string
		err = tx.QueryRow(ctx, `
			INSERT INTO ingredients (tenant_id, sku, name, base_unit, purchasing_unit, conversion_factor, low_stock_threshold, valuation_cost)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (tenant_id, sku) DO UPDATE SET valuation_cost = EXCLUDED.valuation_cost
			RETURNING id
		`, tenantID, ing.sku, ing.name, ing.baseUnit, ing.purchUnit, ing.factor, ing.threshold, ing.cost).Scan(&ingID)
		if err != nil {
			log.Fatalf("seed ingredient %s: %v", ing.sku, err)
		}
		ingIDs[ing.sku] = ingID
	}

	// Recipe for Large Oat Latte (18g beans, 320ml oat milk, 1 cup, 1 lid)
	var latteRecipeID string
	err = tx.QueryRow(ctx, `
		INSERT INTO recipe_versions (tenant_id, menu_item_id, version_number, is_active, yield_quantity)
		VALUES ($1, $2, 1, true, 1)
		RETURNING id
	`, tenantID, oatLatteID).Scan(&latteRecipeID)
	if err != nil {
		log.Fatalf("seed latte recipe: %v", err)
	}

	latteLines := []struct {
		sku string
		qty float64
	}{
		{"ING-BEANS-ESP", 18},
		{"ING-MILK-OAT", 320},
		{"ING-CUP-16OZ", 1},
		{"ING-LID-16OZ", 1},
	}
	for _, line := range latteLines {
		_, err = tx.Exec(ctx, `
			INSERT INTO recipe_lines (tenant_id, recipe_version_id, ingredient_id, quantity_base_unit)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_id, recipe_version_id, ingredient_id) DO NOTHING
		`, tenantID, latteRecipeID, ingIDs[line.sku], line.qty)
		if err != nil {
			log.Fatalf("seed latte recipe line %s: %v", line.sku, err)
		}
	}

	// Recipe for Extra Shot modifier (+9g beans)
	var extraShotRecipeID string
	err = tx.QueryRow(ctx, `
		INSERT INTO recipe_versions (tenant_id, modifier_id, version_number, is_active, yield_quantity)
		VALUES ($1, $2, 1, true, 1)
		RETURNING id
	`, tenantID, extraShotModID).Scan(&extraShotRecipeID)
	if err != nil {
		log.Fatalf("seed extra shot recipe: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO recipe_lines (tenant_id, recipe_version_id, ingredient_id, quantity_base_unit)
		VALUES ($1, $2, $3, 9)
		ON CONFLICT (tenant_id, recipe_version_id, ingredient_id) DO NOTHING
	`, tenantID, extraShotRecipeID, ingIDs["ING-BEANS-ESP"])
	if err != nil {
		log.Fatalf("seed extra shot recipe line: %v", err)
	}

	// Stock Location & Opening Balances at F-7 Coffee Bar
	var barLocationID string
	err = tx.QueryRow(ctx, `
		INSERT INTO stock_locations (tenant_id, branch_id, name, location_type, is_default)
		VALUES ($1, $2, 'F-7 Barista Store', 'barista_bar', true)
		ON CONFLICT (tenant_id, branch_id, name) DO UPDATE SET is_default = true
		RETURNING id
	`, tenantID, coffeeBranchID).Scan(&barLocationID)
	if err != nil {
		log.Fatalf("seed stock location: %v", err)
	}

	for _, ing := range ingDefs {
		_, err = tx.Exec(ctx, `
			INSERT INTO stock_balances (tenant_id, branch_id, stock_location_id, ingredient_id, on_hand_qty, avg_unit_cost)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (tenant_id, stock_location_id, ingredient_id) DO UPDATE SET on_hand_qty = EXCLUDED.on_hand_qty
		`, tenantID, coffeeBranchID, barLocationID, ingIDs[ing.sku], ing.initQty, ing.cost)
		if err != nil {
			log.Fatalf("seed stock balance %s: %v", ing.sku, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO stock_movements (tenant_id, branch_id, stock_location_id, ingredient_id, movement_type, quantity_delta, unit_cost, source_doc_type, source_doc_id, idempotency_key)
			VALUES ($1, $2, $3, $4, 'receipt', $5, $6, 'opening_seed', $3, $7)
			ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
		`, tenantID, coffeeBranchID, barLocationID, ingIDs[ing.sku], ing.initQty, ing.cost, fmt.Sprintf("seed-opening-%s-%s", coffeeBranchID, ing.sku))
		if err != nil {
			log.Fatalf("seed stock movement %s: %v", ing.sku, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit seed tx: %v", err)
	}

	log.Printf("Seed complete for tenant %q (%s): 2 branches, 4 tables, 5 staff users, Large Oat Latte + Extra Shot recipes, and opening warehouse stock.", "Artisan Roast & Bistro", tenantID)
}
