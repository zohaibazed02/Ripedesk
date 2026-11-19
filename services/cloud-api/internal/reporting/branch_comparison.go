package reporting

import (
	"net/http"
	"sort"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// BranchComparisonInput supplies per-branch current and prior period figures.
type BranchComparisonInput struct {
	BranchID           string
	BranchName         string
	BranchCode         string
	BranchType         string
	CurrentNetSales    decimal.Decimal
	PriorNetSales      decimal.Decimal
	OrderCount         int
	FoodCostTotal      decimal.Decimal
	WasteTotal         decimal.Decimal
	PrepDurationsSec   []int
	OverdueTicketCount int
	LastSyncAt         time.Time
	PendingSyncCount   int
}

// BranchComparisonRow is a normalized side-by-side branch performance row.
type BranchComparisonRow struct {
	BranchID          string          `json:"branch_id"`
	BranchName        string          `json:"branch_name"`
	BranchCode        string          `json:"branch_code"`
	BranchType        string          `json:"branch_type"`
	NetSales          decimal.Decimal `json:"net_sales"`
	PriorNetSales     decimal.Decimal `json:"prior_net_sales"`
	NetSalesDeltaPct  decimal.Decimal `json:"net_sales_delta_pct"`
	OrderCount        int             `json:"order_count"`
	AverageOrderValue decimal.Decimal `json:"average_order_value"`
	FoodCostPct       decimal.Decimal `json:"food_cost_pct"`
	WastePct          decimal.Decimal `json:"waste_pct"`
	GrossMarginPct    decimal.Decimal `json:"gross_margin_pct"`
	AvgPrepSeconds    decimal.Decimal `json:"avg_prep_seconds"`
	OnTimeRatePct     decimal.Decimal `json:"on_time_rate_pct"`
	SalesRank         int             `json:"sales_rank"`
	MarginRank        int             `json:"margin_rank"`
	SLARank           int             `json:"sla_rank"`
	IsStale           bool            `json:"is_stale"`
	PendingSyncCount  int             `json:"pending_sync_count"`
}

// BranchComparisonResult aggregates side-by-side branch rows and tenant totals.
type BranchComparisonResult struct {
	Branches        []BranchComparisonRow `json:"branches"`
	TotalNetSales   decimal.Decimal       `json:"total_net_sales"`
	TotalOrderCount int                   `json:"total_order_count"`
	GeneratedAt     time.Time             `json:"generated_at"`
}

// ComputeBranchComparison normalizes metrics, computes period-over-period deltas,
// and assigns deterministic rankings across branches.
func ComputeBranchComparison(inputs []BranchComparisonInput, now time.Time) BranchComparisonResult {
	rows := make([]BranchComparisonRow, len(inputs))
	totalSales := decimal.Zero
	totalOrders := 0

	for i, in := range inputs {
		net := in.CurrentNetSales.Round(2)
		prior := in.PriorNetSales.Round(2)
		deltaPct := decimal.Zero.Round(2)
		if !prior.IsZero() {
			deltaPct = net.Sub(prior).Mul(hundred).Div(prior).Round(2)
		}

		aov := safeDiv(net, decimal.NewFromInt(int64(in.OrderCount)), 2)
		foodPct := safePct(in.FoodCostTotal, net)
		wastePct := safePct(in.WasteTotal, net)
		marginPct := safePct(net.Sub(in.FoodCostTotal), net)
		sla := ComputeKitchenSLA(in.PrepDurationsSec, in.OverdueTicketCount)
		isStale := in.PendingSyncCount > 0 || in.LastSyncAt.IsZero() || now.Sub(in.LastSyncAt) > StaleThreshold

		rows[i] = BranchComparisonRow{
			BranchID:          in.BranchID,
			BranchName:        in.BranchName,
			BranchCode:        in.BranchCode,
			BranchType:        in.BranchType,
			NetSales:          net,
			PriorNetSales:     prior,
			NetSalesDeltaPct:  deltaPct,
			OrderCount:        in.OrderCount,
			AverageOrderValue: aov,
			FoodCostPct:       foodPct,
			WastePct:          wastePct,
			GrossMarginPct:    marginPct,
			AvgPrepSeconds:    sla.AvgPrepSeconds,
			OnTimeRatePct:     sla.OnTimeRatePct,
			IsStale:           isStale,
			PendingSyncCount:  in.PendingSyncCount,
		}
		totalSales = totalSales.Add(net)
		totalOrders += in.OrderCount
	}

	// Assign SalesRank (descending NetSales)
	salesIdx := make([]int, len(rows))
	for i := range rows {
		salesIdx[i] = i
	}
	sort.SliceStable(salesIdx, func(i, j int) bool {
		return rows[salesIdx[i]].NetSales.GreaterThan(rows[salesIdx[j]].NetSales)
	})
	for rank, idx := range salesIdx {
		rows[idx].SalesRank = rank + 1
	}

	// Assign MarginRank (descending GrossMarginPct)
	marginIdx := make([]int, len(rows))
	for i := range rows {
		marginIdx[i] = i
	}
	sort.SliceStable(marginIdx, func(i, j int) bool {
		return rows[marginIdx[i]].GrossMarginPct.GreaterThan(rows[marginIdx[j]].GrossMarginPct)
	})
	for rank, idx := range marginIdx {
		rows[idx].MarginRank = rank + 1
	}

	// Assign SLARank (descending OnTimeRatePct, tie-break ascending AvgPrepSeconds)
	slaIdx := make([]int, len(rows))
	for i := range rows {
		slaIdx[i] = i
	}
	sort.SliceStable(slaIdx, func(i, j int) bool {
		a, b := rows[slaIdx[i]], rows[slaIdx[j]]
		if !a.OnTimeRatePct.Equal(b.OnTimeRatePct) {
			return a.OnTimeRatePct.GreaterThan(b.OnTimeRatePct)
		}
		return a.AvgPrepSeconds.LessThan(b.AvgPrepSeconds)
	})
	for rank, idx := range slaIdx {
		rows[idx].SLARank = rank + 1
	}

	return BranchComparisonResult{
		Branches:        rows,
		TotalNetSales:   totalSales.Round(2),
		TotalOrderCount: totalOrders,
		GeneratedAt:     now.UTC(),
	}
}

// GetBranchComparison handles GET /api/v1/reports/owner/branch-comparison?from=&to=
func (h *Handler) GetBranchComparison(w http.ResponseWriter, r *http.Request) {
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

	periodDur := toTime.Sub(fromTime)
	priorFrom := fromTime.Add(-periodDur)
	priorTo := fromTime

	var result BranchComparisonResult
	err = h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT
				b.id::text,
				b.name,
				b.code,
				b.branch_type,
				COALESCE(b.last_heartbeat_at, b.updated_at),
				COALESCE(SUM(CASE WHEN o.created_at >= $1 AND o.created_at < $2 THEN o.subtotal - o.discount_total ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN o.created_at >= $3 AND o.created_at < $4 THEN o.subtotal - o.discount_total ELSE 0 END), 0),
				COUNT(CASE WHEN o.created_at >= $1 AND o.created_at < $2 THEN 1 END)
			FROM branches b
			LEFT JOIN orders o
				ON o.branch_id = b.id
				AND o.status NOT IN ('cancelled', 'voided')
				AND o.created_at >= $3 AND o.created_at < $2
			WHERE b.is_active = true
			GROUP BY b.id, b.name, b.code, b.branch_type, b.last_heartbeat_at, b.updated_at
			ORDER BY b.name ASC
		`, fromTime, toTime, priorFrom, priorTo)
		if err != nil {
			return err
		}
		defer rows.Close()

		var inputs []BranchComparisonInput
		for rows.Next() {
			var in BranchComparisonInput
			if err := rows.Scan(
				&in.BranchID,
				&in.BranchName,
				&in.BranchCode,
				&in.BranchType,
				&in.LastSyncAt,
				&in.CurrentNetSales,
				&in.PriorNetSales,
				&in.OrderCount,
			); err != nil {
				return err
			}
			inputs = append(inputs, in)
		}
		result = ComputeBranchComparison(inputs, time.Now().UTC())
		return nil
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "branch_comparison_failed", err.Error(), nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"period_from": fromStr,
		"period_to":   toStr,
		"comparison":  result,
	}, nil)
}
