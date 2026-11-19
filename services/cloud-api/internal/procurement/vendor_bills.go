package procurement

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/hospitality-os/cloud-api/internal/ocr"
	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/hospitality-os/cloud-api/internal/storage"
	"github.com/jackc/pgx/v5"
)

const MaxVendorBillUploadBytes = 20 << 20 // 20 MB

func (h *Handler) UploadVendorBill(store storage.ObjectStore, extractor ocr.Extractor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
			return
		}

		if err := r.ParseMultipartForm(MaxVendorBillUploadBytes); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "INVALID_MULTIPART", "Expected multipart/form-data up to 20MB", err.Error())
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "MISSING_FILE", "Form field 'file' is required", err.Error())
			return
		}
		defer file.Close()

		rawBytes, err := io.ReadAll(io.LimitReader(file, MaxVendorBillUploadBytes+1))
		if err != nil || len(rawBytes) > MaxVendorBillUploadBytes {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "File exceeds 20MB limit", nil)
			return
		}

		forceUpload := strings.EqualFold(r.FormValue("force_upload"), "true")
		ext := strings.ToLower(filepath.Ext(header.Filename))

		// 1. Pure-Go blur & resolution check for images
		var quality *ocr.QualityReport
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			q, qErr := ocr.AnalyzeImageQuality(rawBytes)
			if qErr == nil {
				quality = q
				if q.RetakeRecommended && !forceUpload {
					httpx.WriteJSON(w, http.StatusOK, map[string]any{
						"status":  "retake_required",
						"quality": q,
					}, nil)
					return
				}
			}
		}

		// 2. SHA-256 document fingerprint duplicate check
		sha := ocr.ComputeDocumentSHA256(rawBytes)
		var existingDocID string
		_ = h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(r.Context(), `
				SELECT id FROM vendor_invoice_documents WHERE sha256_fingerprint = $1 LIMIT 1
			`, sha).Scan(&existingDocID)
		})
		if existingDocID != "" {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"status":             "duplicate_flagged",
				"duplicate_of_id":    existingDocID,
				"sha256_fingerprint": sha,
			}, nil)
			return
		}

		// 3. Store raw file in tenant-scoped ObjectStore (7-year fiscal retention)
		key, err := storage.BuildTenantVendorBillKey(claims.TenantID, sha[:16], ext, time.Now().UTC())
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "INVALID_OBJECT_KEY", err.Error(), nil)
			return
		}
		if err := store.PutObject(r.Context(), key, rawBytes, storage.PutOptions{
			ContentType:   header.Header.Get("Content-Type"),
			RetentionDays: storage.DefaultRetentionDays,
			Encryption:    "AES256",
		}); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "STORAGE_WRITE_FAILED", err.Error(), nil)
			return
		}

		// 4. Run Pluggable OCR Extraction
		extracted, err := extractor.Extract(r.Context(), header.Filename, rawBytes)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "OCR_EXTRACTION_FAILED", err.Error(), nil)
			return
		}
		normInvNo := ocr.NormalizeInvoiceNumber(extracted.InvoiceNumber.Value)

		httpx.WriteJSON(w, http.StatusCreated, map[string]any{
			"status":                    "extracted_needs_review",
			"storage_object_key":        key,
			"sha256_fingerprint":        sha,
			"normalized_invoice_number": normInvNo,
			"quality":                   quality,
			"extracted_bill":            extracted,
		}, nil)
	}
}

func (h *Handler) ListVendorBills(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	type billRow struct {
		ID                      string `json:"id"`
		InvoiceNumber           string `json:"invoice_number"`
		NormalizedInvoiceNumber string `json:"normalized_invoice_number"`
		Status                  string `json:"status"`
		GrandTotal              string `json:"grand_total"`
	}
	items := []billRow{}
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT id, invoice_number, normalized_invoice_number, status, grand_total::text
			FROM vendor_invoices
			ORDER BY created_at DESC
			LIMIT 100
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var b billRow
			if err := rows.Scan(&b.ID, &b.InvoiceNumber, &b.NormalizedInvoiceNumber, &b.Status, &b.GrandTotal); err != nil {
				return err
			}
			items = append(items, b)
		}
		return rows.Err()
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "VENDOR_BILLS_LIST_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items, map[string]int{"count": len(items)})
}
