package reporting

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

const StaleThreshold = 5 * time.Minute

var hundred = decimal.NewFromInt(100)

// BranchFreshness represents sync health and inventory count status for a branch.
type BranchFreshness struct {
	BranchID          string    `json:"branch_id"`
	BranchName        string    `json:"branch_name"`
	BranchCode        string    `json:"branch_code"`
	BranchType        string    `json:"branch_type"`
	LastSyncAt        time.Time `json:"last_sync_at"`
	PendingSyncCount  int       `json:"pending_sync_count"`
	IsStale           bool      `json:"is_stale"`
	MissingStockCount bool      `json:"missing_stock_count"`
}

// CollectionsBreakdown tracks tender totals using fixed-precision Decimal.
type CollectionsBreakdown struct {
	Cash   decimal.Decimal `json:"cash"`
	Card   decimal.Decimal `json:"card"`
	Online decimal.Decimal `json:"online"`
	COD    decimal.Decimal `json:"cod"`
}

// KitchenSLAMetrics summarizes kitchen prep speed and SLA adherence.
type KitchenSLAMetrics struct {
	AvgPrepSeconds     decimal.Decimal `json:"avg_prep_seconds"`
	P95PrepSeconds     decimal.Decimal `json:"p95_prep_seconds"`
	OverdueTicketCount int             `json:"overdue_ticket_count"`
	TotalTickets       int             `json:"total_tickets"`
	OnTimeRatePct      decimal.Decimal `json:"on_time_rate_pct"`
}

// GLReconciliation verifies POS net sales against posted General Ledger revenue.
type GLReconciliation struct {
	ReconciledToGL bool            `json:"reconciled_to_gl"`
	POSNetSales    decimal.Decimal `json:"pos_net_sales"`
	GLRevenueTotal decimal.Decimal `json:"gl_revenue_total"`
	GLDiff         decimal.Decimal `json:"gl_diff"`
}

// OwnerSummaryInput holds raw aggregated figures for deterministic metric computation.
type OwnerSummaryInput struct {
	PeriodFrom         string
	PeriodTo           string
	BranchID           string
	GrossSales         decimal.Decimal
	Discounts          decimal.Decimal
	Refunds            decimal.Decimal
	TaxCollected       decimal.Decimal
	OrderCount         int
	Collections        CollectionsBreakdown
	FoodCostTotal      decimal.Decimal
	WasteTotal         decimal.Decimal
	StockVarianceTotal decimal.Decimal
	GLRevenueTotal     decimal.Decimal
	PrepDurationsSec   []int
	OverdueTicketCount int
	Branches           []BranchFreshness
}

// OwnerSummary is the executive reporting payload returned by the Owner API.
type OwnerSummary struct {
	PeriodFrom                string               `json:"period_from"`
	PeriodTo                  string               `json:"period_to"`
	BranchID                  string               `json:"branch_id,omitempty"`
	GrossSales                decimal.Decimal      `json:"gross_sales"`
	Discounts                 decimal.Decimal      `json:"discounts"`
	Refunds                   decimal.Decimal      `json:"refunds"`
	NetSales                  decimal.Decimal      `json:"net_sales"`
	TaxCollected              decimal.Decimal      `json:"tax_collected"`
	OrderCount                int                  `json:"order_count"`
	AverageOrderValue         decimal.Decimal      `json:"average_order_value"`
	Collections               CollectionsBreakdown `json:"collections"`
	FoodCostTotal             decimal.Decimal      `json:"food_cost_total"`
	FoodCostPct               decimal.Decimal      `json:"food_cost_pct"`
	WasteTotal                decimal.Decimal      `json:"waste_total"`
	WastePct                  decimal.Decimal      `json:"waste_pct"`
	StockVarianceTotal        decimal.Decimal      `json:"stock_variance_total"`
	StockVariancePct          decimal.Decimal      `json:"stock_variance_pct"`
	GrossMarginTotal          decimal.Decimal      `json:"gross_margin_total"`
	GrossMarginPct            decimal.Decimal      `json:"gross_margin_pct"`
	KitchenSLA                KitchenSLAMetrics    `json:"kitchen_sla"`
	GLReconciliation          GLReconciliation     `json:"gl_reconciliation"`
	StaleBranchIDs            []string             `json:"stale_branch_ids"`
	TotalPendingOfflineOrders int                  `json:"total_pending_offline_orders"`
	MissingStockCountBranches []string             `json:"missing_stock_count_branches"`
	IsStale                   bool                 `json:"is_stale"`
	BranchFreshness           []BranchFreshness    `json:"branch_freshness"`
	GeneratedAt               time.Time            `json:"generated_at"`
}

func safeDiv(num, den decimal.Decimal, scale int32) decimal.Decimal {
	if den.IsZero() {
		return decimal.Zero.Round(scale)
	}
	return num.Div(den).Round(scale)
}

func safePct(part, total decimal.Decimal) decimal.Decimal {
	if total.IsZero() {
		return decimal.Zero.Round(2)
	}
	return part.Mul(hundred).Div(total).Round(2)
}

// ComputeKitchenSLA computes avg, p95, and on-time percentage using exact Decimal math.
func ComputeKitchenSLA(durationsSec []int, overdueCount int) KitchenSLAMetrics {
	total := len(durationsSec)
	if total == 0 {
		return KitchenSLAMetrics{
			AvgPrepSeconds:     decimal.Zero.Round(2),
			P95PrepSeconds:     decimal.Zero.Round(2),
			OverdueTicketCount: overdueCount,
			TotalTickets:       0,
			OnTimeRatePct:      hundred.Round(2),
		}
	}

	sorted := make([]int, total)
	copy(sorted, durationsSec)
	sort.Ints(sorted)

	sum := decimal.Zero
	for _, d := range sorted {
		sum = sum.Add(decimal.NewFromInt(int64(d)))
	}
	avg := sum.Div(decimal.NewFromInt(int64(total))).Round(2)

	// Deterministic nearest-rank P95 index: ceil(0.95 * N) - 1
	p95Idx := (95*total + 99) / 100
	if p95Idx > 0 {
		p95Idx--
	}
	if p95Idx >= total {
		p95Idx = total - 1
	}
	p95 := decimal.NewFromInt(int64(sorted[p95Idx])).Round(2)

	onTime := total - overdueCount
	if onTime < 0 {
		onTime = 0
	}
	onTimePct := safePct(decimal.NewFromInt(int64(onTime)), decimal.NewFromInt(int64(total)))

	return KitchenSLAMetrics{
		AvgPrepSeconds:     avg,
		P95PrepSeconds:     p95,
		OverdueTicketCount: overdueCount,
		TotalTickets:       total,
		OnTimeRatePct:      onTimePct,
	}
}

// ComputeOwnerSummary deterministically calculates all executive KPIs, margins,
// kitchen SLA, POS-to-GL reconciliation, and data freshness flags.
func ComputeOwnerSummary(in OwnerSummaryInput, now time.Time) OwnerSummary {
	netSales := in.GrossSales.Sub(in.Discounts).Sub(in.Refunds).Round(2)
	aov := safeDiv(netSales, decimal.NewFromInt(int64(in.OrderCount)), 2)

	grossMargin := netSales.Sub(in.FoodCostTotal).Round(2)
	foodCostPct := safePct(in.FoodCostTotal, netSales)
	wastePct := safePct(in.WasteTotal, netSales)
	stockVariancePct := safePct(in.StockVarianceTotal, netSales)
	grossMarginPct := safePct(grossMargin, netSales)

	glDiff := netSales.Sub(in.GLRevenueTotal).Round(2)
	glRec := GLReconciliation{
		ReconciledToGL: glDiff.IsZero(),
		POSNetSales:    netSales,
		GLRevenueTotal: in.GLRevenueTotal.Round(2),
		GLDiff:         glDiff,
	}

	staleIDs := make([]string, 0)
	missingStockIDs := make([]string, 0)
	totalPending := 0
	freshness := make([]BranchFreshness, len(in.Branches))

	for i, b := range in.Branches {
		isStale := b.PendingSyncCount > 0 || b.LastSyncAt.IsZero() || now.Sub(b.LastSyncAt) > StaleThreshold
		b.IsStale = isStale
		freshness[i] = b
		totalPending += b.PendingSyncCount
		if isStale {
			staleIDs = append(staleIDs, b.BranchID)
		}
		if b.MissingStockCount {
			missingStockIDs = append(missingStockIDs, b.BranchID)
		}
	}

	return OwnerSummary{
		PeriodFrom:        in.PeriodFrom,
		PeriodTo:          in.PeriodTo,
		BranchID:          in.BranchID,
		GrossSales:        in.GrossSales.Round(2),
		Discounts:         in.Discounts.Round(2),
		Refunds:           in.Refunds.Round(2),
		NetSales:          netSales,
		TaxCollected:      in.TaxCollected.Round(2),
		OrderCount:        in.OrderCount,
		AverageOrderValue: aov,
		Collections: CollectionsBreakdown{
			Cash:   in.Collections.Cash.Round(2),
			Card:   in.Collections.Card.Round(2),
			Online: in.Collections.Online.Round(2),
			COD:    in.Collections.COD.Round(2),
		},
		FoodCostTotal:             in.FoodCostTotal.Round(2),
		FoodCostPct:               foodCostPct,
		WasteTotal:                in.WasteTotal.Round(2),
		WastePct:                  wastePct,
		StockVarianceTotal:        in.StockVarianceTotal.Round(2),
		StockVariancePct:          stockVariancePct,
		GrossMarginTotal:          grossMargin,
		GrossMarginPct:            grossMarginPct,
		KitchenSLA:                ComputeKitchenSLA(in.PrepDurationsSec, in.OverdueTicketCount),
		GLReconciliation:          glRec,
		StaleBranchIDs:            staleIDs,
		TotalPendingOfflineOrders: totalPending,
		MissingStockCountBranches: missingStockIDs,
		IsStale:                   len(staleIDs) > 0,
		BranchFreshness:           freshness,
		GeneratedAt:               now.UTC(),
	}
}

// Handler serves tenant-scoped executive owner reporting endpoints.
type Handler struct {
	pool *db.Pool
}

// NewHandler creates a new reporting HTTP handler.
func NewHandler(pool *db.Pool) *Handler {
	return &Handler{pool: pool}
}

func parseDateRange(r *http.Request) (time.Time, time.Time, string, string, error) {
	now := time.Now().UTC()
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var fromTime, toTime time.Time
	var err error

	if fromStr == "" {
		fromTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		fromStr = fromTime.Format("2006-01-02")
	} else {
		fromTime, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("invalid 'from' date (expected YYYY-MM-DD)")
		}
	}

	if toStr == "" {
		toTime = fromTime.Add(24 * time.Hour)
		toStr = fromTime.Format("2006-01-02")
	} else {
		parsedTo, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("invalid 'to' date (expected YYYY-MM-DD)")
		}
		toTime = parsedTo.Add(24 * time.Hour)
	}

	return fromTime, toTime, fromStr, toStr, nil
}

func checkBranchAccess(claims *auth.Claims, requestedBranchID string) (string, error) {
	if claims.Role == "branch_manager" && len(claims.BranchIDs) > 0 {
		if requestedBranchID == "" {
			return claims.BranchIDs[0], nil
		}
		for _, allowed := range claims.BranchIDs {
			if allowed == requestedBranchID {
				return requestedBranchID, nil
			}
		}
		return "", fmt.Errorf("branch_manager cannot access branch %s", requestedBranchID)
	}
	return requestedBranchID, nil
}

// GetOwnerSummary handles GET /api/v1/reports/owner/summary?from=&to=&branch_id=
func (h *Handler) GetOwnerSummary(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth claims", nil)
		return
	}

	fromTime, toTime, fromStr, toStr, err := parseDateRange(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_date_range", err.Error(), nil)
		return
	}

	branchID, err := checkBranchAccess(claims, r.URL.Query().Get("branch_id"))
	if err != nil {
		httpx.WriteError(w, http.StatusForbidden, "branch_scope_denied", err.Error(), nil)
		return
	}

	var summary OwnerSummary
	err = h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		in := OwnerSummaryInput{
			PeriodFrom: fromStr,
			PeriodTo:   toStr,
			BranchID:   branchID,
		}

		// 1. Aggregate orders
		orderQuery := `
			SELECT
				COALESCE(SUM(subtotal), 0),
				COALESCE(SUM(discount_total), 0),
				COALESCE(SUM(CASE WHEN status = 'refunded' THEN grand_total ELSE 0 END), 0),
				COALESCE(SUM(tax_total), 0),
				COUNT(*)
			FROM orders
			WHERE created_at >= $1 AND created_at < $2
			  AND status NOT IN ('cancelled', 'voided')
			  AND ($3 = '' OR branch_id::text = $3)
		`
		if err := tx.QueryRow(r.Context(), orderQuery, fromTime, toTime, branchID).Scan(
			&in.GrossSales,
			&in.Discounts,
			&in.Refunds,
			&in.TaxCollected,
			&in.OrderCount,
		); err != nil {
			return err
		}

		// 2. Aggregate GL revenue (account_code = '4000') inside a savepoint so empty/uninitialized COA never aborts tx
		if sp, spErr := tx.Begin(r.Context()); spErr == nil {
			glQuery := `
				SELECT COALESCE(SUM(jl.credit - jl.debit), 0)
				FROM journal_lines jl
				JOIN journal_entries je ON je.id = jl.journal_entry_id
				JOIN chart_of_accounts coa ON coa.id = jl.account_id
				WHERE je.entry_date >= $1::date AND je.entry_date < $2::date
				  AND je.status = 'posted'
				  AND coa.code = '4000'
				  AND ($3 = '' OR je.branch_id::text = $3)
			`
			if err := sp.QueryRow(r.Context(), glQuery, fromTime, toTime, branchID).Scan(&in.GLRevenueTotal); err != nil {
				_ = sp.Rollback(r.Context())
			} else {
				_ = sp.Commit(r.Context())
			}
		}

		// 3. Branch freshness (matches 000001_tenant_and_rbac.up.sql columns)
		branchQuery := `
			SELECT id::text, name, code, branch_type, COALESCE(last_sync_at, updated_at), pending_sync_count
			FROM branches
			WHERE ($1 = '' OR id::text = $1)
			ORDER BY name ASC
		`
		rows, err := tx.Query(r.Context(), branchQuery, branchID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var bf BranchFreshness
			if err := rows.Scan(&bf.BranchID, &bf.BranchName, &bf.BranchCode, &bf.BranchType, &bf.LastSyncAt, &bf.PendingSyncCount); err != nil {
				return err
			}
			in.Branches = append(in.Branches, bf)
		}

		summary = ComputeOwnerSummary(in, time.Now().UTC())
		return nil
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "report_query_failed", err.Error(), nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, summary, nil)
}

// GetKitchenSLA handles GET /api/v1/reports/owner/kitchen-sla?from=&to=&branch_id=
func (h *Handler) GetKitchenSLA(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth claims", nil)
		return
	}

	fromTime, toTime, fromStr, toStr, err := parseDateRange(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_date_range", err.Error(), nil)
		return
	}

	branchID, err := checkBranchAccess(claims, r.URL.Query().Get("branch_id"))
	if err != nil {
		httpx.WriteError(w, http.StatusForbidden, "branch_scope_denied", err.Error(), nil)
		return
	}

	var sla KitchenSLAMetrics
	err = h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT EXTRACT(EPOCH FROM (COALESCE(completed_at, NOW()) - created_at))::int
			FROM orders
			WHERE created_at >= $1 AND created_at < $2
			  AND ($3 = '' OR branch_id::text = $3)
		`, fromTime, toTime, branchID)
		if err != nil {
			return err
		}
		defer rows.Close()

		var durations []int
		overdue := 0
		for rows.Next() {
			var sec int
			if err := rows.Scan(&sec); err != nil {
				return err
			}
			if sec < 0 {
				sec = 0
			}
			durations = append(durations, sec)
			if sec > 900 { // > 15m SLA threshold
				overdue++
			}
		}
		sla = ComputeKitchenSLA(durations, overdue)
		return nil
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "kitchen_sla_failed", err.Error(), nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"period_from": fromStr,
		"period_to":   toStr,
		"branch_id":   branchID,
		"kitchen_sla": sla,
	}, nil)
}
