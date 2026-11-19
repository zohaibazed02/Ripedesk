package accounting

import (
	"math"
	"net/http"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type BankLine struct {
	ID          string          `json:"id"`
	Date        time.Time       `json:"date"`
	Description string          `json:"description"`
	Reference   string          `json:"reference"`
	Amount      decimal.Decimal `json:"amount"`
}

type BankMatchPair struct {
	StatementLineID string          `json:"statement_line_id"`
	BookLineID      string          `json:"book_line_id"`
	Amount          decimal.Decimal `json:"amount"`
}

type BankReconciliationResult struct {
	MatchedPairs            []BankMatchPair `json:"matched_pairs"`
	UnmatchedStatementLines []BankLine      `json:"unmatched_statement_lines"`
	UnmatchedBookLines      []BankLine      `json:"unmatched_book_lines"`
	StatementClosingBalance decimal.Decimal `json:"statement_closing_balance"`
	ClearedBalance          decimal.Decimal `json:"cleared_balance"`
	UnreconciledDiff        decimal.Decimal `json:"unreconciled_diff"`
	CanFinalize             bool            `json:"can_finalize"`
}

// EvaluateBankReconciliation auto-matches bank statement lines against GL 1040 book lines
// by exact amount within a ±3 day window and computes unreconciled_diff.
func EvaluateBankReconciliation(
	statementLines []BankLine,
	bookLines []BankLine,
	openingBalance decimal.Decimal,
	statementClosingBalance decimal.Decimal,
) BankReconciliationResult {
	usedBook := map[string]bool{}
	var matched []BankMatchPair
	var unmatchedStmt []BankLine
	cleared := openingBalance.Round(4)

	for _, st := range statementLines {
		foundIdx := -1
		for i, bk := range bookLines {
			if usedBook[bk.ID] {
				continue
			}
			dayDiff := math.Abs(st.Date.Sub(bk.Date).Hours() / 24.0)
			if st.Amount.Round(4).Equal(bk.Amount.Round(4)) && dayDiff <= 3.0 {
				foundIdx = i
				break
			}
		}
		if foundIdx >= 0 {
			bk := bookLines[foundIdx]
			usedBook[bk.ID] = true
			matched = append(matched, BankMatchPair{
				StatementLineID: st.ID,
				BookLineID:      bk.ID,
				Amount:          st.Amount.Round(4),
			})
			cleared = cleared.Add(st.Amount.Round(4))
		} else {
			unmatchedStmt = append(unmatchedStmt, st)
		}
	}

	var unmatchedBook []BankLine
	for _, bk := range bookLines {
		if !usedBook[bk.ID] {
			unmatchedBook = append(unmatchedBook, bk)
		}
	}

	diff := statementClosingBalance.Round(4).Sub(cleared).Round(4)
	return BankReconciliationResult{
		MatchedPairs:            matched,
		UnmatchedStatementLines: unmatchedStmt,
		UnmatchedBookLines:      unmatchedBook,
		StatementClosingBalance: statementClosingBalance.Round(4),
		ClearedBalance:          cleared,
		UnreconciledDiff:        diff,
		CanFinalize:             diff.IsZero(),
	}
}

func (h *Handler) EvaluateBankRecEndpoint(w http.ResponseWriter, r *http.Request) {
	var in struct {
		StatementLines          []BankLine      `json:"statement_lines"`
		BookLines               []BankLine      `json:"book_lines"`
		OpeningBalance          decimal.Decimal `json:"opening_balance"`
		StatementClosingBalance decimal.Decimal `json:"statement_closing_balance"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	res := EvaluateBankReconciliation(in.StatementLines, in.BookLines, in.OpeningBalance, in.StatementClosingBalance)
	httpx.WriteJSON(w, http.StatusOK, res, nil)
}

func (h *Handler) FinalizeBankRecEndpoint(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UnreconciledDiff decimal.Decimal `json:"unreconciled_diff"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	if !in.UnreconciledDiff.IsZero() {
		httpx.WriteError(w, http.StatusConflict, "BANK_REC_UNBALANCED", "Cannot finalize bank reconciliation while unreconciled_diff != 0", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "reconciled_finalized"}, nil)
}
