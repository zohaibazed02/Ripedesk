package accounting

import (
	"net/http"
	"sort"

	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type TrialBalanceRow struct {
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	Debit       decimal.Decimal `json:"debit"`
	Credit      decimal.Decimal `json:"credit"`
}

type TrialBalanceReport struct {
	Rows         []TrialBalanceRow `json:"rows"`
	TotalDebits  decimal.Decimal   `json:"total_debits"`
	TotalCredits decimal.Decimal   `json:"total_credits"`
	IsBalanced   bool              `json:"is_balanced"`
}

func ComputeTrialBalance(lines []JournalLineInput) TrialBalanceReport {
	coaMap := map[string]AccountDefinition{}
	for _, a := range DefaultHospitalityCOA() {
		coaMap[a.Code] = a
	}

	netByAcct := map[string]decimal.Decimal{}
	for _, l := range lines {
		netByAcct[l.AccountCode] = netByAcct[l.AccountCode].Add(l.Debit).Sub(l.Credit)
	}

	codes := make([]string, 0, len(netByAcct))
	for c := range netByAcct {
		codes = append(codes, c)
	}
	sort.Strings(codes)

	rows := make([]TrialBalanceRow, 0, len(codes))
	totDr, totCr := decimal.Zero, decimal.Zero
	for _, c := range codes {
		net := netByAcct[c].Round(4)
		if net.IsZero() {
			continue
		}
		dr, cr := decimal.Zero, decimal.Zero
		if net.GreaterThan(decimal.Zero) {
			dr = net
			totDr = totDr.Add(dr)
		} else {
			cr = net.Abs()
			totCr = totCr.Add(cr)
		}
		rows = append(rows, TrialBalanceRow{
			AccountCode: c,
			AccountName: coaMap[c].Name,
			Debit:       dr,
			Credit:      cr,
		})
	}
	return TrialBalanceReport{
		Rows:         rows,
		TotalDebits:  totDr.Round(4),
		TotalCredits: totCr.Round(4),
		IsBalanced:   totDr.Equal(totCr),
	}
}

type ProfitAndLossReport struct {
	GrossRevenue    decimal.Decimal `json:"gross_revenue"`
	ContraRevenue   decimal.Decimal `json:"contra_revenue"`
	NetRevenue      decimal.Decimal `json:"net_revenue"`
	DirectCosts     decimal.Decimal `json:"direct_costs"`
	GrossProfit     decimal.Decimal `json:"gross_profit"`
	FoodCostPct     decimal.Decimal `json:"food_cost_pct"`
	GrossMarginPct  decimal.Decimal `json:"gross_margin_pct"`
	OperatingExp    decimal.Decimal `json:"operating_expenses"`
	NetIncome       decimal.Decimal `json:"net_income"`
	NetMarginPct    decimal.Decimal `json:"net_margin_pct"`
}

func ComputeProfitAndLoss(lines []JournalLineInput) ProfitAndLossReport {
	hundred := decimal.NewFromInt(100)
	grossRev, contraRev, directCosts, opEx := decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero

	for _, l := range lines {
		switch l.AccountCode {
		case AcctFnBSales, AcctDeliveryRevenue:
			grossRev = grossRev.Add(l.Credit).Sub(l.Debit)
		case AcctSalesDiscounts, AcctSalesRefunds:
			contraRev = contraRev.Add(l.Debit).Sub(l.Credit)
		case AcctCOGS, AcctKitchenWaste:
			directCosts = directCosts.Add(l.Debit).Sub(l.Credit)
		case AcctCashOverShort, AcctCardFees, AcctFreightInbound:
			opEx = opEx.Add(l.Debit).Sub(l.Credit)
		}
	}

	netRev := grossRev.Sub(contraRev).Round(4)
	grossProfit := netRev.Sub(directCosts).Round(4)
	netIncome := grossProfit.Sub(opEx).Round(4)

	foodCostPct, grossMarginPct, netMarginPct := decimal.Zero, decimal.Zero, decimal.Zero
	if netRev.GreaterThan(decimal.Zero) {
		foodCostPct = directCosts.Div(netRev).Mul(hundred).Round(2)
		grossMarginPct = grossProfit.Div(netRev).Mul(hundred).Round(2)
		netMarginPct = netIncome.Div(netRev).Mul(hundred).Round(2)
	}

	return ProfitAndLossReport{
		GrossRevenue:   grossRev.Round(4),
		ContraRevenue:  contraRev.Round(4),
		NetRevenue:     netRev,
		DirectCosts:    directCosts.Round(4),
		GrossProfit:    grossProfit,
		FoodCostPct:    foodCostPct,
		GrossMarginPct: grossMarginPct,
		OperatingExp:   opEx.Round(4),
		NetIncome:      netIncome,
		NetMarginPct:   netMarginPct,
	}
}

type BalanceSheetReport struct {
	TotalAssets            decimal.Decimal `json:"total_assets"`
	TotalLiabilities       decimal.Decimal `json:"total_liabilities"`
	RetainedAndNetEquity   decimal.Decimal `json:"total_equity"`
	IsEquationBalanced     bool            `json:"is_equation_balanced"`
}

func ComputeBalanceSheet(lines []JournalLineInput) BalanceSheetReport {
	assets, liabilities, equity := decimal.Zero, decimal.Zero, decimal.Zero
	for _, l := range lines {
		if len(l.AccountCode) == 0 {
			continue
		}
		switch l.AccountCode[0] {
		case '1':
			assets = assets.Add(l.Debit).Sub(l.Credit)
		case '2':
			liabilities = liabilities.Add(l.Credit).Sub(l.Debit)
		case '3':
			equity = equity.Add(l.Credit).Sub(l.Debit)
		}
	}
	pnl := ComputeProfitAndLoss(lines)
	totalEquity := equity.Add(pnl.NetIncome).Round(4)
	totalAssets := assets.Round(4)
	totalLiab := liabilities.Round(4)

	return BalanceSheetReport{
		TotalAssets:          totalAssets,
		TotalLiabilities:     totalLiab,
		RetainedAndNetEquity: totalEquity,
		IsEquationBalanced:   totalAssets.Equal(totalLiab.Add(totalEquity)),
	}
}

func (h *Handler) GetTrialBalance(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, ComputeTrialBalance(nil), nil)
}

func (h *Handler) GetProfitAndLoss(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, ComputeProfitAndLoss(nil), nil)
}

func (h *Handler) GetBalanceSheet(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, ComputeBalanceSheet(nil), nil)
}
