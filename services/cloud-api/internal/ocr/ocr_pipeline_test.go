package ocr_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/hospitality-os/cloud-api/internal/ocr"
	"github.com/shopspring/decimal"
)

func encodeSyntheticPNG(w, h int, sharp bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if sharp {
				// Crisp high-contrast checkerboard text-like pattern
				if (x/4+y/4)%2 == 0 {
					img.Set(x, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
				} else {
					img.Set(x, y, color.RGBA{R: 235, G: 235, B: 235, A: 255})
				}
			} else {
				// Flat low-variance blurred gray surface
				img.Set(x, y, color.RGBA{R: 130, G: 130, B: 130, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestBlurDetectionSharpVsBlurry(t *testing.T) {
	sharpReport, err := ocr.AnalyzeImageQuality(encodeSyntheticPNG(700, 900, true))
	if err != nil {
		t.Fatalf("AnalyzeImageQuality sharp failed: %v", err)
	}
	if !sharpReport.Passed || sharpReport.RetakeRecommended || sharpReport.IsBlurry {
		t.Fatalf("expected sharp image to pass, got %+v", sharpReport)
	}

	blurryReport, err := ocr.AnalyzeImageQuality(encodeSyntheticPNG(700, 900, false))
	if err != nil {
		t.Fatalf("AnalyzeImageQuality blurry failed: %v", err)
	}
	if blurryReport.Passed || !blurryReport.RetakeRecommended || !blurryReport.IsBlurry {
		t.Fatalf("expected blurry image to trigger retake_recommended, got %+v", blurryReport)
	}
}

func TestInvoiceNormalizationAndSHA256Fingerprint(t *testing.T) {
	n1 := ocr.NormalizeInvoiceNumber("INV-2026 / 0042")
	n2 := ocr.NormalizeInvoiceNumber("inv20260042")
	if n1 != n2 || n1 != "INV20260042" {
		t.Fatalf("expected normalized INV20260042, got %q vs %q", n1, n2)
	}

	h1 := ocr.ComputeDocumentSHA256([]byte("receipt-bytes-v1"))
	h2 := ocr.ComputeDocumentSHA256([]byte("receipt-bytes-v1"))
	if h1 != h2 || len(h1) != 64 {
		t.Fatalf("expected deterministic 64-char SHA-256, got %q", h1)
	}
}

func TestMathValidationUrduExtractionAndSanitization(t *testing.T) {
	ext := ocr.NewPilotBenchmarkExtractor()
	bill, err := ext.Extract(context.Background(), "bill.png", nil)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if !bill.MathValid {
		t.Fatalf("expected valid math on pilot bill, delta=%s", bill.MathDelta)
	}
	if !strings.Contains(bill.Lines[0].RawDescription, "اوٹ ملک") {
		t.Fatalf("expected UTF-8 Urdu description preserved, got %q", bill.Lines[0].RawDescription)
	}

	// Introduce OCR misread mismatch
	bill.GrandTotal = decimal.RequireFromString("10500.00")
	valid, delta := ocr.ValidateExtractedMath(bill)
	if valid || !delta.Equal(decimal.RequireFromString("300.0000")) {
		t.Fatalf("expected math mismatch delta 300.0000, got valid=%v delta=%s", valid, delta)
	}

	// Untrusted text sanitization
	dirty := "<script>alert(1)</script>Oat Milk\x00\x07 — اوٹ ملک"
	clean := ocr.SanitizeExtractedText(dirty)
	if strings.Contains(clean, "<script>") || strings.Contains(clean, "\x00") {
		t.Fatalf("sanitization failed: %q", clean)
	}
}
