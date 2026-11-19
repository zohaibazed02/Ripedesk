package qualification

import (
	"sort"

	"github.com/shopspring/decimal"
)

const DownstreamRecordsPerOrder = 12

// ScaleProfile defines target load parameters for qualification runs.
type ScaleProfile struct {
	Name                     string `json:"name"`
	DurationSeconds          int    `json:"duration_seconds"`
	TargetOrdersPerSec       int    `json:"target_orders_per_sec"`
	BurstOrdersPerSec        int    `json:"burst_orders_per_sec"`
	ExpectedDownstreamPerSec int    `json:"expected_downstream_per_sec"`
	Description              string `json:"description"`
}

// Profiles returns the 3 standard qualification presets.
func Profiles() map[string]ScaleProfile {
	return map[string]ScaleProfile{
		"ci_smoke": {
			Name:                     "ci_smoke",
			DurationSeconds:          3,
			TargetOrdersPerSec:       50,
			BurstOrdersPerSec:        100,
			ExpectedDownstreamPerSec: 50 * DownstreamRecordsPerOrder,
			Description:              "Fast 3-second local CI verification run",
		},
		"tenant_steady": {
			Name:                     "tenant_steady",
			DurationSeconds:          1800,
			TargetOrdersPerSec:       50,
			BurstOrdersPerSec:        100,
			ExpectedDownstreamPerSec: 50 * DownstreamRecordsPerOrder,
			Description:              "50 orders/sec per tenant for 30m + 100/sec burst",
		},
		"fleet_scale": {
			Name:                     "fleet_scale",
			DurationSeconds:          3600,
			TargetOrdersPerSec:       16667, // ~1,000,000 orders/min
			BurstOrdersPerSec:        20000,
			ExpectedDownstreamPerSec: 16667 * DownstreamRecordsPerOrder, // ~200,004 records/sec
			Description:              "1,000,000 orders/min across skewed tenants (~200k downstream records/sec)",
		},
	}
}

// GateInput holds measured metrics from a chaos/load run.
type GateInput struct {
	ProfileName                    string
	SaleAckLatenciesMs             []int64
	KDSDeliveryLatenciesMs         []int64
	CloudAcceptLatenciesMs         []int64
	TotalOrders                    int64
	DurationSeconds                int64
	QueueLag                       int64
	RecoveryTimeMs                 int64
	LostAcknowledgedOrders         int64
	DuplicatePaymentOrStockEffects int64
	TotalInfraCostPKR              decimal.Decimal
}

// GateResult represents one mandatory release gate evaluation.
type GateResult struct {
	Name      string `json:"name"`
	Threshold string `json:"threshold"`
	Observed  string `json:"observed"`
	Passed    bool   `json:"passed"`
}

// QualificationReport summarizes latencies, throughput, cost, and all 5 release gates.
type QualificationReport struct {
	ProfileName           string          `json:"profile_name"`
	LocalSaleAckP50Ms     int64           `json:"local_sale_ack_p50_ms"`
	LocalSaleAckP95Ms     int64           `json:"local_sale_ack_p95_ms"`
	LocalSaleAckP99Ms     int64           `json:"local_sale_ack_p99_ms"`
	LocalKDSDeliveryP95Ms int64           `json:"local_kds_delivery_p95_ms"`
	CloudAcceptP95Ms      int64           `json:"cloud_accept_p95_ms"`
	OrdersPerSec          decimal.Decimal `json:"orders_per_sec"`
	DownstreamRecsPerSec  decimal.Decimal `json:"downstream_records_per_sec"`
	QueueLag              int64           `json:"queue_lag"`
	RecoveryTimeMs        int64           `json:"recovery_time_ms"`
	CostPer1KOrdersPKR    decimal.Decimal `json:"cost_per_1k_orders_pkr"`
	AllGatesPassed        bool            `json:"all_gates_passed"`
	Gates                 []GateResult    `json:"gates"`
}

// PercentileMs computes nearest-rank percentile in milliseconds.
func PercentileMs(samples []int64, pct int) int64 {
	if len(samples) == 0 {
		return 0
	}
	cp := make([]int64, len(samples))
	copy(cp, samples)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := (pct*len(cp) + 99) / 100
	if idx > 0 {
		idx--
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

// EvaluateGates evaluates all 5 mandatory release qualification gates.
func EvaluateGates(in GateInput) QualificationReport {
	saleP50 := PercentileMs(in.SaleAckLatenciesMs, 50)
	saleP95 := PercentileMs(in.SaleAckLatenciesMs, 95)
	saleP99 := PercentileMs(in.SaleAckLatenciesMs, 99)
	kdsP95 := PercentileMs(in.KDSDeliveryLatenciesMs, 95)
	cloudP95 := PercentileMs(in.CloudAcceptLatenciesMs, 95)

	dur := in.DurationSeconds
	if dur <= 0 {
		dur = 1
	}
	ordersPerSec := decimal.NewFromInt(in.TotalOrders).Div(decimal.NewFromInt(dur)).Round(2)
	downstreamPerSec := ordersPerSec.Mul(decimal.NewFromInt(DownstreamRecordsPerOrder)).Round(2)

	costPer1K := decimal.Zero.Round(4)
	if in.TotalOrders > 0 {
		costPer1K = in.TotalInfraCostPKR.Mul(decimal.NewFromInt(1000)).Div(decimal.NewFromInt(in.TotalOrders)).Round(4)
	}

	gates := []GateResult{
		{
			Name:      "local_sale_ack_p95_ms",
			Threshold: "< 200ms",
			Observed:  decimal.NewFromInt(saleP95).String() + "ms",
			Passed:    saleP95 < 200,
		},
		{
			Name:      "local_kds_delivery_p95_ms",
			Threshold: "< 1000ms",
			Observed:  decimal.NewFromInt(kdsP95).String() + "ms",
			Passed:    kdsP95 < 1000,
		},
		{
			Name:      "regional_cloud_accept_p95_ms",
			Threshold: "< 500ms",
			Observed:  decimal.NewFromInt(cloudP95).String() + "ms",
			Passed:    cloudP95 < 500,
		},
		{
			Name:      "lost_acknowledged_orders",
			Threshold: "== 0",
			Observed:  decimal.NewFromInt(in.LostAcknowledgedOrders).String(),
			Passed:    in.LostAcknowledgedOrders == 0,
		},
		{
			Name:      "duplicate_payment_or_stock_effects",
			Threshold: "== 0",
			Observed:  decimal.NewFromInt(in.DuplicatePaymentOrStockEffects).String(),
			Passed:    in.DuplicatePaymentOrStockEffects == 0,
		},
	}

	allPassed := true
	for _, g := range gates {
		if !g.Passed {
			allPassed = false
			break
		}
	}

	return QualificationReport{
		ProfileName:           in.ProfileName,
		LocalSaleAckP50Ms:     saleP50,
		LocalSaleAckP95Ms:     saleP95,
		LocalSaleAckP99Ms:     saleP99,
		LocalKDSDeliveryP95Ms: kdsP95,
		CloudAcceptP95Ms:      cloudP95,
		OrdersPerSec:          ordersPerSec,
		DownstreamRecsPerSec:  downstreamPerSec,
		QueueLag:              in.QueueLag,
		RecoveryTimeMs:        in.RecoveryTimeMs,
		CostPer1KOrdersPKR:    costPer1K,
		AllGatesPassed:        allPassed,
		Gates:                 gates,
	}
}
