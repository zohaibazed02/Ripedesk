package qualification

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestEvaluateGates_PassAndThroughputMath(t *testing.T) {
	rep := EvaluateGates(GateInput{
		ProfileName:                    "fleet_scale",
		SaleAckLatenciesMs:             []int64{12, 18, 25, 42, 85},
		KDSDeliveryLatenciesMs:         []int64{45, 90, 120, 210, 340},
		CloudAcceptLatenciesMs:         []int64{60, 110, 180, 290, 410},
		TotalOrders:                    16667,
		DurationSeconds:                1,
		QueueLag:                       0,
		RecoveryTimeMs:                 420,
		LostAcknowledgedOrders:         0,
		DuplicatePaymentOrStockEffects: 0,
		TotalInfraCostPKR:              decimal.RequireFromString("333.34"),
	})

	if !rep.AllGatesPassed {
		t.Fatalf("expected all 5 gates to pass, got %+v", rep.Gates)
	}
	if !rep.DownstreamRecsPerSec.Equal(decimal.RequireFromString("200004.00")) {
		t.Fatalf("expected 200004.00 downstream records/sec, got %s", rep.DownstreamRecsPerSec)
	}
	if rep.LocalSaleAckP95Ms != 85 {
		t.Fatalf("expected sale p95 85ms, got %d", rep.LocalSaleAckP95Ms)
	}
}

func TestEvaluateGates_FailsOnBreachOrDataLoss(t *testing.T) {
	rep := EvaluateGates(GateInput{
		ProfileName:                    "ci_smoke",
		SaleAckLatenciesMs:             []int64{50, 250}, // >= 200ms breach
		KDSDeliveryLatenciesMs:         []int64{100},
		CloudAcceptLatenciesMs:         []int64{100},
		TotalOrders:                    100,
		DurationSeconds:                2,
		LostAcknowledgedOrders:         1, // breach
		DuplicatePaymentOrStockEffects: 1, // breach
	})

	if rep.AllGatesPassed {
		t.Fatalf("expected gates to fail on latency breach and data loss")
	}
}
