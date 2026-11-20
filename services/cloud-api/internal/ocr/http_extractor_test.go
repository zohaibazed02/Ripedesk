package ocr_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hospitality-os/cloud-api/internal/ocr"
	"github.com/shopspring/decimal"
)

func TestHTTPVisionExtractor_LiveExtractionSanitizationAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-ocr-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := ocr.ExtractedBill{
			ExtractorName: "http_vision_v1",
			ModelVersion:  "docai-2026",
			SupplierName:  ocr.ConfidenceField{Value: "<b>Al-Fatah Dairy</b>", Confidence: 0.98},
			InvoiceNumber: ocr.ConfidenceField{Value: "INV-2026-0099", Confidence: 0.97},
			CurrencyCode:  "PKR",
			Subtotal:      decimal.RequireFromString("7000.00"),
			GrandTotal:    decimal.RequireFromString("7000.00"),
			Lines: []ocr.ExtractedBillLine{
				{
					LineNumber:     1,
					RawDescription: "<script>alert(1)</script>Oat Milk 1L",
					Quantity:       decimal.NewFromInt(10),
					Unit:           "L",
					UnitPrice:      decimal.RequireFromString("700.00"),
					LineAmount:     decimal.RequireFromString("7000.00"),
					Confidence:     0.96,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ext := &ocr.HTTPVisionExtractor{APIURL: srv.URL + "/extract", APIKey: "test-ocr-key"}
	bill, err := ext.Extract(context.Background(), "invoice.pdf", []byte("%PDF-1.4 mock"))
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if strings.Contains(bill.SupplierName.Value, "<") || strings.Contains(bill.Lines[0].RawDescription, "<script>") {
		t.Fatalf("unsanitized HTML remained: supplier=%q desc=%q", bill.SupplierName.Value, bill.Lines[0].RawDescription)
	}
	if !bill.MathValid {
		t.Fatalf("expected valid math, got delta=%s", bill.MathDelta)
	}

	// Verify 500 error propagation
	errExt := &ocr.HTTPVisionExtractor{APIURL: srv.URL + "/fail", APIKey: "test-ocr-key"}
	if _, err := errExt.Extract(context.Background(), "invoice.pdf", []byte("x")); err == nil {
		t.Fatalf("expected error on HTTP 500 from OCR provider")
	}
}
