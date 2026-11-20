package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// HTTPVisionExtractor calls an external Vision/Document-AI HTTP API and sanitizes all extracted text.
type HTTPVisionExtractor struct {
	APIURL string
	APIKey string
	Client *http.Client
}

// NewExtractorFromEnv returns HTTPVisionExtractor when OCR_API_URL and OCR_API_KEY are set,
// otherwise falls back explicitly to PilotBenchmarkExtractor.
func NewExtractorFromEnv() Extractor {
	url := os.Getenv("OCR_API_URL")
	key := os.Getenv("OCR_API_KEY")
	if url != "" && key != "" {
		return &HTTPVisionExtractor{
			APIURL: url,
			APIKey: key,
			Client: &http.Client{Timeout: 15 * time.Second},
		}
	}
	return NewPilotBenchmarkExtractor()
}

func (e *HTTPVisionExtractor) Extract(ctx context.Context, filename string, content []byte) (*ExtractedBill, error) {
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = http.DetectContentType(content)
	}

	reqBody, err := json.Marshal(map[string]any{
		"filename":       filename,
		"mime_type":      mimeType,
		"content_base64": base64.StdEncoding.EncodeToString(content),
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.APIURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.APIKey)

	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ocr http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ocr provider returned HTTP %d", resp.StatusCode)
	}

	var bill ExtractedBill
	if err := json.NewDecoder(resp.Body).Decode(&bill); err != nil {
		return nil, fmt.Errorf("decode ocr response: %w", err)
	}

	// Strictly sanitize all extracted strings as untrusted passive data
	bill.SupplierName.Value = SanitizeExtractedText(bill.SupplierName.Value)
	bill.InvoiceNumber.Value = SanitizeExtractedText(bill.InvoiceNumber.Value)
	bill.POReference.Value = SanitizeExtractedText(bill.POReference.Value)
	for i := range bill.Lines {
		bill.Lines[i].RawDescription = SanitizeExtractedText(bill.Lines[i].RawDescription)
		bill.Lines[i].SupplierSKU = SanitizeExtractedText(bill.Lines[i].SupplierSKU)
	}

	valid, delta := ValidateExtractedMath(&bill)
	bill.MathValid = valid
	bill.MathDelta = delta
	return &bill, nil
}
