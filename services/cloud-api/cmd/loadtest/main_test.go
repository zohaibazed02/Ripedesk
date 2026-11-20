package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hospitality-os/cloud-api/internal/qualification"
	"github.com/shopspring/decimal"
)

func TestRunLiveHTTPWorkers_AgainstTestServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	latencies, errCount := runLiveHTTPWorkers(srv.URL, 40, 8, 10, 0.8)
	if errCount != 0 {
		t.Fatalf("expected 0 HTTP errors, got %d", errCount)
	}
	if len(latencies) != 40 {
		t.Fatalf("expected 40 latency samples, got %d", len(latencies))
	}

	rep := qualification.EvaluateGates(qualification.GateInput{
		ProfileName:                    "ci_smoke",
		SaleAckLatenciesMs:             []int64{10, 15},
		KDSDeliveryLatenciesMs:         []int64{25, 40},
		CloudAcceptLatenciesMs:         latencies,
		TotalOrders:                    40,
		DurationSeconds:                1,
		LostAcknowledgedOrders:         errCount,
		DuplicatePaymentOrStockEffects: 0,
		TotalInfraCostPKR:              decimal.RequireFromString("0.80"),
	})
	if !rep.AllGatesPassed {
		t.Fatalf("expected all qualification gates to pass against local test server, got %+v", rep.Gates)
	}
}
