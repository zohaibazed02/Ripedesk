package accounting

type AccountDefinition struct {
	Code string `json:"account_code"`
	Name string `json:"name"`
	Type string `json:"account_type"` // asset | liability | equity | revenue | expense
}

const (
	AcctCashOnHand       = "1010"
	AcctRiderCOD         = "1020"
	AcctCardClearing     = "1030"
	AcctOperatingBank    = "1040"
	AcctInventory        = "1200"
	AcctGRNIClearing     = "1210"
	AcctAccountsPayable  = "2010"
	AcctSalesTaxPayable  = "2100"
	AcctOwnerEquity      = "3010"
	AcctFnBSales         = "4010"
	AcctDeliveryRevenue  = "4020"
	AcctSalesDiscounts   = "4090"
	AcctSalesRefunds     = "4095"
	AcctCOGS             = "5010"
	AcctKitchenWaste     = "5020"
	AcctCashOverShort    = "5030"
	AcctCardFees         = "5040"
	AcctFreightInbound   = "5050"
)

func DefaultHospitalityCOA() []AccountDefinition {
	return []AccountDefinition{
		{Code: AcctCashOnHand, Name: "Cash on Hand (Register)", Type: "asset"},
		{Code: AcctRiderCOD, Name: "Rider COD Receivable", Type: "asset"},
		{Code: AcctCardClearing, Name: "Undeposited Card Clearing", Type: "asset"},
		{Code: AcctOperatingBank, Name: "Main Operating Bank", Type: "asset"},
		{Code: AcctInventory, Name: "Ingredient Inventory", Type: "asset"},
		{Code: AcctGRNIClearing, Name: "GRNI Clearing", Type: "asset"},
		{Code: AcctAccountsPayable, Name: "Accounts Payable — Suppliers", Type: "liability"},
		{Code: AcctSalesTaxPayable, Name: "Sales Tax / GST Payable", Type: "liability"},
		{Code: AcctOwnerEquity, Name: "Owner Equity / Retained Earnings", Type: "equity"},
		{Code: AcctFnBSales, Name: "Food & Beverage Sales", Type: "revenue"},
		{Code: AcctDeliveryRevenue, Name: "Delivery Fee Revenue", Type: "revenue"},
		{Code: AcctSalesDiscounts, Name: "Sales Discounts (Contra)", Type: "revenue"},
		{Code: AcctSalesRefunds, Name: "Sales Refunds & Returns (Contra)", Type: "revenue"},
		{Code: AcctCOGS, Name: "Cost of Goods Sold (COGS)", Type: "expense"},
		{Code: AcctKitchenWaste, Name: "Kitchen Waste & Spoilage", Type: "expense"},
		{Code: AcctCashOverShort, Name: "Cash Over / Short Variance", Type: "expense"},
		{Code: AcctCardFees, Name: "Card Processing Fees", Type: "expense"},
		{Code: AcctFreightInbound, Name: "Freight & Delivery Inbound", Type: "expense"},
	}
}
