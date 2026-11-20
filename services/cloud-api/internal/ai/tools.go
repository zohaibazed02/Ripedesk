package ai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

var (
	// ErrForbiddenBranchScope is returned when an actor attempts to query a branch outside their allowed scope.
	ErrForbiddenBranchScope = errors.New("forbidden: requested branch is outside actor scope")
	// ErrUnapprovedTool is returned when an unregistered or mutating tool is requested.
	ErrUnapprovedTool = errors.New("unapproved or mutating tool rejected by read-only allowlist")
	// ErrInvalidTenantScope is returned when tenant_id is missing.
	ErrInvalidTenantScope = errors.New("invalid tenant scope: tenant_id is required")
)

// Approved read-only tool names. Any other tool name is rejected.
const (
	ToolGetMarginDrivers          = "get_margin_drivers"
	ToolGetBranchFreshnessAndGaps = "get_branch_freshness_and_gaps"
	ToolGetWasteAndVariance       = "get_waste_and_variance_hotspots"
	ToolGetReorderSuggestions     = "get_reorder_suggestions"
	ToolGetSalesAndRefundSummary  = "get_sales_and_refund_summary"
)

var approvedToolSet = map[string]bool{
	ToolGetMarginDrivers:          true,
	ToolGetBranchFreshnessAndGaps: true,
	ToolGetWasteAndVariance:       true,
	ToolGetReorderSuggestions:     true,
	ToolGetSalesAndRefundSummary:  true,
}

// ToolExecutionContext carries strict tenant, actor, and branch scope for every tool call.
type ToolExecutionContext struct {
	TenantID           string
	ActorUserID        string
	ActorRole          string
	AllowedBranchIDs   []string
	RequestedBranchIDs []string
	FromDate           string
	ToDate             string
}

// ValidateScope enforces tenant presence and strict branch-level RBAC before any query runs.
func ValidateScope(execCtx ToolExecutionContext) ([]string, error) {
	if execCtx.TenantID == "" {
		return nil, ErrInvalidTenantScope
	}
	if execCtx.ActorRole == "branch_manager" {
		if len(execCtx.AllowedBranchIDs) == 0 {
			return nil, ErrForbiddenBranchScope
		}
		if len(execCtx.RequestedBranchIDs) == 0 {
			out := make([]string, len(execCtx.AllowedBranchIDs))
			copy(out, execCtx.AllowedBranchIDs)
			return out, nil
		}
		allowed := make(map[string]bool, len(execCtx.AllowedBranchIDs))
		for _, id := range execCtx.AllowedBranchIDs {
			allowed[id] = true
		}
		for _, reqID := range execCtx.RequestedBranchIDs {
			if !allowed[reqID] {
				return nil, fmt.Errorf("%w: %s", ErrForbiddenBranchScope, reqID)
			}
		}
		return execCtx.RequestedBranchIDs, nil
	}
	return execCtx.RequestedBranchIDs, nil
}

// MarginDriver represents a quantified contributor to margin movement.
type MarginDriver struct {
	ItemOrIngredient string          `json:"item_or_ingredient"`
	BranchName       string          `json:"branch_name"`
	ImpactPKR        decimal.Decimal `json:"impact_pkr"`
	ImpactPct        decimal.Decimal `json:"impact_pct"`
	Category         string          `json:"category"` // ingredient_cost_spike | stock_variance | waste | discount_mix
}

// BranchFreshnessGap reports sync lag, pending offline orders, or missing stock counts.
type BranchFreshnessGap struct {
	BranchID          string    `json:"branch_id"`
	BranchName        string    `json:"branch_name"`
	LastSyncAt        time.Time `json:"last_sync_at"`
	PendingSyncCount  int       `json:"pending_sync_count"`
	IsStale           bool      `json:"is_stale"`
	MissingStockCount bool      `json:"missing_stock_count"`
}

// WasteVarianceHotspot ranks an ingredient by unexplained variance and waste.
type WasteVarianceHotspot struct {
	IngredientName string          `json:"ingredient_name"`
	BranchName     string          `json:"branch_name"`
	WastePKR       decimal.Decimal `json:"waste_pkr"`
	VariancePKR    decimal.Decimal `json:"variance_pkr"`
	BaseQuantity   decimal.Decimal `json:"base_quantity"`
	BaseUnit       string          `json:"base_unit"`
}

// ReorderSuggestion is a strictly advisory reorder proposal for human review.
type ReorderSuggestion struct {
	IngredientName      string          `json:"ingredient_name"`
	BranchName          string          `json:"branch_name"`
	CurrentStock        decimal.Decimal `json:"current_stock"`
	SafetyStock         decimal.Decimal `json:"safety_stock"`
	OpenInboundPOQty    decimal.Decimal `json:"open_inbound_po_qty"`
	SuggestedReorderQty decimal.Decimal `json:"suggested_reorder_qty"`
	Unit                string          `json:"unit"`
	EstimatedCostPKR    decimal.Decimal `json:"estimated_cost_pkr"`
}

// SalesRefundSummary captures reconciled sales, refunds, and POS-to-GL integrity.
type SalesRefundSummary struct {
	NetSales       decimal.Decimal `json:"net_sales"`
	Refunds        decimal.Decimal `json:"refunds"`
	Voids          decimal.Decimal `json:"voids"`
	ReconciledToGL bool            `json:"reconciled_to_gl"`
	GLDiffPKR      decimal.Decimal `json:"gl_diff_pkr"`
}

// ReadOnlyDataSource abstracts read-only tenant-scoped queries for the approved tools.
type ReadOnlyDataSource interface {
	GetMarginDrivers(ctx context.Context, tenantID string, branchIDs []string, from, to string) ([]MarginDriver, error)
	GetBranchFreshnessAndGaps(ctx context.Context, tenantID string, branchIDs []string) ([]BranchFreshnessGap, error)
	GetWasteAndVarianceHotspots(ctx context.Context, tenantID string, branchIDs []string, from, to string) ([]WasteVarianceHotspot, error)
	GetReorderSuggestions(ctx context.Context, tenantID string, branchIDs []string) ([]ReorderSuggestion, error)
	GetSalesAndRefundSummary(ctx context.Context, tenantID string, branchIDs []string, from, to string) (SalesRefundSummary, error)
}

// ToolRegistry enforces the read-only allowlist and tenant/branch scope before invoking any tool.
type ToolRegistry struct {
	ds ReadOnlyDataSource
}

// NewToolRegistry creates a guarded read-only tool registry.
func NewToolRegistry(ds ReadOnlyDataSource) *ToolRegistry {
	return &ToolRegistry{ds: ds}
}

// Execute validates the tool allowlist and tenant/branch scope, then runs the read-only tool.
func (r *ToolRegistry) Execute(ctx context.Context, execCtx ToolExecutionContext, toolName string) (any, error) {
	if !approvedToolSet[toolName] {
		return nil, fmt.Errorf("%w: %s", ErrUnapprovedTool, toolName)
	}
	effectiveBranches, err := ValidateScope(execCtx)
	if err != nil {
		return nil, err
	}

	switch toolName {
	case ToolGetMarginDrivers:
		return r.ds.GetMarginDrivers(ctx, execCtx.TenantID, effectiveBranches, execCtx.FromDate, execCtx.ToDate)
	case ToolGetBranchFreshnessAndGaps:
		return r.ds.GetBranchFreshnessAndGaps(ctx, execCtx.TenantID, effectiveBranches)
	case ToolGetWasteAndVariance:
		return r.ds.GetWasteAndVarianceHotspots(ctx, execCtx.TenantID, effectiveBranches, execCtx.FromDate, execCtx.ToDate)
	case ToolGetReorderSuggestions:
		return r.ds.GetReorderSuggestions(ctx, execCtx.TenantID, effectiveBranches)
	case ToolGetSalesAndRefundSummary:
		return r.ds.GetSalesAndRefundSummary(ctx, execCtx.TenantID, effectiveBranches, execCtx.FromDate, execCtx.ToDate)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnapprovedTool, toolName)
	}
}

// PostgresReadOnlyDataSource executes strictly read-only SELECT queries via pool.WithTenantTx.
type PostgresReadOnlyDataSource struct {
	pool *db.Pool
}

// NewPostgresReadOnlyDataSource creates a tenant-isolated PostgreSQL read-only data source.
func NewPostgresReadOnlyDataSource(pool *db.Pool) *PostgresReadOnlyDataSource {
	return &PostgresReadOnlyDataSource{pool: pool}
}

func (p *PostgresReadOnlyDataSource) GetMarginDrivers(ctx context.Context, tenantID string, branchIDs []string, from, to string) ([]MarginDriver, error) {
	var out []MarginDriver
	err := p.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT i.name, b.name, COALESCE(ABS(SUM(sm.total_cost)), 0)
			FROM stock_movements sm
			JOIN ingredients i ON i.id = sm.ingredient_id
			JOIN branches b ON b.id = sm.branch_id
			WHERE sm.movement_type IN ('waste', 'adjustment')
			  AND ($1::text[] IS NULL OR cardinality($1::text[]) = 0 OR b.id::text = ANY($1::text[]))
			GROUP BY i.name, b.name
			ORDER BY COALESCE(ABS(SUM(sm.total_cost)), 0) DESC
			LIMIT 5
		`, branchIDs)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d MarginDriver
			if err := rows.Scan(&d.ItemOrIngredient, &d.BranchName, &d.ImpactPKR); err != nil {
				return err
			}
			d.ImpactPct = decimal.Zero.Round(2)
			d.Category = "stock_variance"
			out = append(out, d)
		}
		return nil
	})
	return out, err
}

func (p *PostgresReadOnlyDataSource) GetBranchFreshnessAndGaps(ctx context.Context, tenantID string, branchIDs []string) ([]BranchFreshnessGap, error) {
	var out []BranchFreshnessGap
	now := time.Now().UTC()
	err := p.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, name, COALESCE(last_sync_at, updated_at), pending_sync_count
			FROM branches
			WHERE ($1::text[] IS NULL OR cardinality($1::text[]) = 0 OR id::text = ANY($1::text[]))
			ORDER BY name ASC
		`, branchIDs)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var g BranchFreshnessGap
			if err := rows.Scan(&g.BranchID, &g.BranchName, &g.LastSyncAt, &g.PendingSyncCount); err != nil {
				return err
			}
			g.IsStale = g.PendingSyncCount > 0 || g.LastSyncAt.IsZero() || now.Sub(g.LastSyncAt) > 5*time.Minute
			out = append(out, g)
		}
		return nil
	})
	return out, err
}

func (p *PostgresReadOnlyDataSource) GetWasteAndVarianceHotspots(ctx context.Context, tenantID string, branchIDs []string, from, to string) ([]WasteVarianceHotspot, error) {
	var out []WasteVarianceHotspot
	err := p.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT i.name, b.name,
			       COALESCE(ABS(SUM(CASE WHEN sm.movement_type = 'waste' THEN sm.total_cost ELSE 0 END)), 0),
			       COALESCE(ABS(SUM(CASE WHEN sm.movement_type = 'adjustment' THEN sm.total_cost ELSE 0 END)), 0),
			       COALESCE(ABS(SUM(sm.quantity_base)), 0),
			       i.base_unit
			FROM stock_movements sm
			JOIN ingredients i ON i.id = sm.ingredient_id
			JOIN branches b ON b.id = sm.branch_id
			WHERE sm.movement_type IN ('waste', 'adjustment')
			  AND ($1::text[] IS NULL OR cardinality($1::text[]) = 0 OR b.id::text = ANY($1::text[]))
			GROUP BY i.name, b.name, i.base_unit
			ORDER BY 3 DESC, 4 DESC
			LIMIT 5
		`, branchIDs)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var h WasteVarianceHotspot
			if err := rows.Scan(&h.IngredientName, &h.BranchName, &h.WastePKR, &h.VariancePKR, &h.BaseQuantity, &h.BaseUnit); err != nil {
				return err
			}
			out = append(out, h)
		}
		return nil
	})
	return out, err
}

func (p *PostgresReadOnlyDataSource) GetReorderSuggestions(ctx context.Context, tenantID string, branchIDs []string) ([]ReorderSuggestion, error) {
	var out []ReorderSuggestion
	err := p.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT i.name, b.name, COALESCE(bs.current_qty_base, 0), COALESCE(bs.reorder_level_base, 0), i.base_unit
			FROM branch_stock bs
			JOIN ingredients i ON i.id = bs.ingredient_id
			JOIN branches b ON b.id = bs.branch_id
			WHERE bs.current_qty_base <= bs.reorder_level_base
			  AND ($1::text[] IS NULL OR cardinality($1::text[]) = 0 OR b.id::text = ANY($1::text[]))
			ORDER BY (bs.reorder_level_base - bs.current_qty_base) DESC
			LIMIT 5
		`, branchIDs)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s ReorderSuggestion
			if err := rows.Scan(&s.IngredientName, &s.BranchName, &s.CurrentStock, &s.SafetyStock, &s.Unit); err != nil {
				return err
			}
			deficit := s.SafetyStock.Sub(s.CurrentStock)
			if deficit.IsPositive() {
				s.SuggestedReorderQty = deficit.Mul(decimal.NewFromInt(2)).Round(2)
			}
			out = append(out, s)
		}
		return nil
	})
	return out, err
}

func (p *PostgresReadOnlyDataSource) GetSalesAndRefundSummary(ctx context.Context, tenantID string, branchIDs []string, from, to string) (SalesRefundSummary, error) {
	var res SalesRefundSummary
	err := p.pool.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var glRev decimal.Decimal
		err := tx.QueryRow(ctx, `
			SELECT
				COALESCE(SUM(CASE WHEN status NOT IN ('cancelled','voided') THEN subtotal - discount_total ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN status = 'refunded' THEN grand_total ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN status = 'voided' THEN grand_total ELSE 0 END), 0)
			FROM orders
			WHERE ($1::text[] IS NULL OR cardinality($1::text[]) = 0 OR branch_id::text = ANY($1::text[]))
		`, branchIDs).Scan(&res.NetSales, &res.Refunds, &res.Voids)
		if err != nil {
			return err
		}
		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(jl.credit - jl.debit), 0)
			FROM journal_lines jl
			JOIN journal_entries je ON je.id = jl.journal_entry_id
			JOIN chart_of_accounts coa ON coa.id = jl.account_id
			WHERE je.status = 'posted' AND coa.code = '4000'
			  AND ($1::text[] IS NULL OR cardinality($1::text[]) = 0 OR je.branch_id::text = ANY($1::text[]))
		`, branchIDs).Scan(&glRev)
		res.GLDiffPKR = res.NetSales.Sub(glRev).Round(2)
		res.ReconciledToGL = res.GLDiffPKR.IsZero()
		return nil
	})
	return res, err
}
