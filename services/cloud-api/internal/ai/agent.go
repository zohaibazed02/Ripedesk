package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const PromptVersion = "owner-analyst-prompt-v1.0"

// DateRange specifies the analyzed reporting window.
type DateRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// HumanReviewSuggestion is an advisory action card requiring human approval.
type HumanReviewSuggestion struct {
	Type                  string          `json:"type"` // reorder_quantity | waste_investigation | stock_count_audit
	Title                 string          `json:"title"`
	RecommendedAction     string          `json:"recommended_action"`
	EstimatedImpactPKR    decimal.Decimal `json:"estimated_impact_pkr"`
	RequiresHumanApproval bool            `json:"requires_human_approval"`
	TargetReportLink      string          `json:"target_report_link"`
}

// ReportLink points to an Admin ERP verification screen.
type ReportLink struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

// AuditMetadata records model version, prompt version, tools invoked, and read-only guarantee.
type AuditMetadata struct {
	ModelVersion              string    `json:"model_version"`
	ExtractionOrPromptVersion string    `json:"extraction_or_prompt_version"`
	ToolsInvoked              []string  `json:"tools_invoked"`
	GeneratedAt               time.Time `json:"generated_at"`
	ReadOnlyEnforced          bool      `json:"read_only_enforced"`
}

// AIReportResponse is the strict grounded response contract returned by the AI Reporting Agent.
type AIReportResponse struct {
	Question               string                  `json:"question"`
	AnswerSummary          string                  `json:"answer_summary"`
	DateRange              DateRange               `json:"date_range"`
	BranchesAnalyzed       []string                `json:"branches_analyzed"`
	ContributingFactors    []MarginDriver          `json:"contributing_factors"`
	DataQualityWarnings    []string                `json:"data_quality_warnings"`
	HumanReviewSuggestions []HumanReviewSuggestion `json:"human_review_suggestions"`
	UnderlyingReportLinks  []ReportLink            `json:"underlying_report_links"`
	AuditMetadata          AuditMetadata           `json:"audit_metadata"`
}

// ToolOutputBundle holds sanitized, structured outputs from approved read-only tools.
type ToolOutputBundle struct {
	MarginDrivers []MarginDriver         `json:"margin_drivers,omitempty"`
	FreshnessGaps []BranchFreshnessGap   `json:"freshness_gaps,omitempty"`
	WasteHotspots []WasteVarianceHotspot `json:"waste_hotspots,omitempty"`
	Reorders      []ReorderSuggestion    `json:"reorders,omitempty"`
	SalesSummary  *SalesRefundSummary    `json:"sales_summary,omitempty"`
}

// LLMExplainer synthesizes a grounded executive explanation from structured tool outputs.
type LLMExplainer interface {
	Explain(ctx context.Context, question string, bundle ToolOutputBundle) (answerSummary string, modelVersion string, err error)
}

// GroundedAnalyticalExplainer deterministically synthesizes answers without external API keys.
type GroundedAnalyticalExplainer struct{}

func (g *GroundedAnalyticalExplainer) Explain(_ context.Context, question string, bundle ToolOutputBundle) (string, string, error) {
	parts := make([]string, 0, 4)

	if len(bundle.MarginDrivers) > 0 {
		top := bundle.MarginDrivers[0]
		parts = append(parts, fmt.Sprintf(
			"Primary margin driver: %s at %s impacted margin by PKR %s (%s%%, category: %s).",
			top.ItemOrIngredient, top.BranchName, top.ImpactPKR.StringFixed(2), top.ImpactPct.StringFixed(2), top.Category,
		))
	}
	if len(bundle.WasteHotspots) > 0 {
		topW := bundle.WasteHotspots[0]
		parts = append(parts, fmt.Sprintf(
			"Top kitchen waste/variance hotspot: %s at %s (waste PKR %s, variance PKR %s).",
			topW.IngredientName, topW.BranchName, topW.WastePKR.StringFixed(2), topW.VariancePKR.StringFixed(2),
		))
	}
	if len(bundle.Reorders) > 0 {
		parts = append(parts, fmt.Sprintf(
			"%d low-stock ingredient(s) flagged for human-review reorder approval.",
			len(bundle.Reorders),
		))
	}
	if bundle.SalesSummary != nil {
		glStatus := "reconciled to GL"
		if !bundle.SalesSummary.ReconciledToGL {
			glStatus = fmt.Sprintf("GL variance of PKR %s", bundle.SalesSummary.GLDiffPKR.StringFixed(2))
		}
		parts = append(parts, fmt.Sprintf(
			"Net sales: PKR %s (refunds PKR %s, %s).",
			bundle.SalesSummary.NetSales.StringFixed(2), bundle.SalesSummary.Refunds.StringFixed(2), glStatus,
		))
	}

	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("Completed read-only analysis for %q with no anomalies detected in the selected window.", question))
	}
	return strings.Join(parts, " "), "grounded-analyst-v1.0", nil
}

// ManagedLLMHTTPAdapter calls a configurable external LLM endpoint with ONLY structured tool outputs.
type ManagedLLMHTTPAdapter struct {
	APIURL       string
	APIKey       string
	ModelVersion string
	Client       *http.Client
	Fallback     LLMExplainer
}

func (m *ManagedLLMHTTPAdapter) Explain(ctx context.Context, question string, bundle ToolOutputBundle) (string, string, error) {
	if m.APIURL == "" || m.APIKey == "" {
		if m.Fallback != nil {
			return m.Fallback.Explain(ctx, question, bundle)
		}
		return (&GroundedAnalyticalExplainer{}).Explain(ctx, question, bundle)
	}

	payload := map[string]any{
		"model":          m.ModelVersion,
		"prompt_version": PromptVersion,
		"question":       question,
		"tool_outputs":   bundle,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.APIURL, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.APIKey)

	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		if m.Fallback != nil {
			return m.Fallback.Explain(ctx, question, bundle)
		}
		return "", "", err
	}
	defer resp.Body.Close()

	var parsed struct {
		AnswerSummary string `json:"answer_summary"`
		ModelVersion  string `json:"model_version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil || parsed.AnswerSummary == "" {
		if m.Fallback != nil {
			return m.Fallback.Explain(ctx, question, bundle)
		}
		return "", "", fmt.Errorf("managed LLM returned invalid response")
	}
	mv := parsed.ModelVersion
	if mv == "" {
		mv = m.ModelVersion
	}
	return parsed.AnswerSummary, mv, nil
}

// Agent orchestrates approved read-only tools and produces grounded executive reports.
type Agent struct {
	registry  *ToolRegistry
	explainer LLMExplainer
}

// NewAgent constructs a read-only AI Reporting Agent.
func NewAgent(registry *ToolRegistry, explainer LLMExplainer) *Agent {
	if explainer == nil {
		explainer = &GroundedAnalyticalExplainer{}
	}
	return &Agent{registry: registry, explainer: explainer}
}

// Ask answers an executive question using only approved read-only tools.
func (a *Agent) Ask(ctx context.Context, execCtx ToolExecutionContext, question string) (AIReportResponse, error) {
	if _, err := ValidateScope(execCtx); err != nil {
		return AIReportResponse{}, err
	}

	q := strings.ToLower(question)
	toolsToRun := []string{ToolGetBranchFreshnessAndGaps}
	if strings.Contains(q, "margin") || strings.Contains(q, "cost") || strings.Contains(q, "drop") {
		toolsToRun = append(toolsToRun, ToolGetMarginDrivers, ToolGetWasteAndVariance, ToolGetReorderSuggestions)
	} else if strings.Contains(q, "reorder") || strings.Contains(q, "stock") {
		toolsToRun = append(toolsToRun, ToolGetReorderSuggestions, ToolGetWasteAndVariance)
	} else if strings.Contains(q, "waste") || strings.Contains(q, "variance") {
		toolsToRun = append(toolsToRun, ToolGetWasteAndVariance, ToolGetMarginDrivers)
	} else {
		toolsToRun = append(toolsToRun, ToolGetSalesAndRefundSummary, ToolGetMarginDrivers, ToolGetReorderSuggestions)
	}

	return a.runPipeline(ctx, execCtx, question, toolsToRun)
}

// DailyBriefing generates the automated morning executive briefing.
func (a *Agent) DailyBriefing(ctx context.Context, execCtx ToolExecutionContext) (AIReportResponse, error) {
	toolsToRun := []string{
		ToolGetBranchFreshnessAndGaps,
		ToolGetSalesAndRefundSummary,
		ToolGetMarginDrivers,
		ToolGetWasteAndVariance,
		ToolGetReorderSuggestions,
	}
	return a.runPipeline(ctx, execCtx, "Daily Executive Briefing", toolsToRun)
}

func (a *Agent) runPipeline(ctx context.Context, execCtx ToolExecutionContext, question string, toolsToRun []string) (AIReportResponse, error) {
	var bundle ToolOutputBundle
	invoked := make([]string, 0, len(toolsToRun))

	for _, tName := range toolsToRun {
		res, err := a.registry.Execute(ctx, execCtx, tName)
		if err != nil {
			return AIReportResponse{}, err
		}
		invoked = append(invoked, tName)
		switch v := res.(type) {
		case []MarginDriver:
			bundle.MarginDrivers = v
		case []BranchFreshnessGap:
			bundle.FreshnessGaps = v
		case []WasteVarianceHotspot:
			bundle.WasteHotspots = v
		case []ReorderSuggestion:
			bundle.Reorders = v
		case SalesRefundSummary:
			cp := v
			bundle.SalesSummary = &cp
		}
	}

	summary, modelVersion, err := a.explainer.Explain(ctx, question, bundle)
	if err != nil {
		return AIReportResponse{}, err
	}

	branchSet := make(map[string]bool)
	branchesAnalyzed := make([]string, 0)
	warnings := make([]string, 0)
	suggestions := make([]HumanReviewSuggestion, 0)

	for _, g := range bundle.FreshnessGaps {
		if !branchSet[g.BranchName] {
			branchSet[g.BranchName] = true
			branchesAnalyzed = append(branchesAnalyzed, g.BranchName)
		}
		if g.IsStale {
			warnings = append(warnings, fmt.Sprintf("Branch %s has stale sync data (%d pending offline orders).", g.BranchName, g.PendingSyncCount))
		}
		if g.MissingStockCount {
			warnings = append(warnings, fmt.Sprintf("Branch %s has a missing/overdue physical stock count.", g.BranchName))
			suggestions = append(suggestions, HumanReviewSuggestion{
				Type:                  "stock_count_audit",
				Title:                 fmt.Sprintf("Schedule physical stock count at %s", g.BranchName),
				RecommendedAction:     "Assign branch manager to complete closing stock count before finalizing weekly margin.",
				EstimatedImpactPKR:    decimal.Zero.Round(2),
				RequiresHumanApproval: true,
				TargetReportLink:      "/inventory/ledger",
			})
		}
	}

	for _, w := range bundle.WasteHotspots {
		totalLoss := w.WastePKR.Add(w.VariancePKR).Round(2)
		suggestions = append(suggestions, HumanReviewSuggestion{
			Type:                  "waste_investigation",
			Title:                 fmt.Sprintf("Investigate %s waste & variance at %s", w.IngredientName, w.BranchName),
			RecommendedAction:     fmt.Sprintf("Audit recipe portioning and prep logs for %s (%s %s unexplained).", w.IngredientName, w.BaseQuantity.StringFixed(2), w.BaseUnit),
			EstimatedImpactPKR:    totalLoss,
			RequiresHumanApproval: true,
			TargetReportLink:      "/inventory/recipes",
		})
	}

	for _, r := range bundle.Reorders {
		suggestions = append(suggestions, HumanReviewSuggestion{
			Type:                  "reorder_quantity",
			Title:                 fmt.Sprintf("Review reorder for %s (%s)", r.IngredientName, r.BranchName),
			RecommendedAction:     fmt.Sprintf("Advisory only: consider ordering %s %s (current %s vs safety %s).", r.SuggestedReorderQty.StringFixed(2), r.Unit, r.CurrentStock.StringFixed(2), r.SafetyStock.StringFixed(2)),
			EstimatedImpactPKR:    r.EstimatedCostPKR.Round(2),
			RequiresHumanApproval: true,
			TargetReportLink:      "/procurement/purchase-orders",
		})
	}

	links := []ReportLink{
		{Label: "Financial Statements (P&L)", Href: "/accounting/statements"},
		{Label: "Inventory Ledger & Variance", Href: "/inventory/ledger"},
		{Label: "Purchase Orders (Human Review)", Href: "/procurement/purchase-orders"},
		{Label: "Recipe Costing", Href: "/inventory/recipes"},
	}

	return AIReportResponse{
		Question:               question,
		AnswerSummary:          summary,
		DateRange:              DateRange{From: execCtx.FromDate, To: execCtx.ToDate},
		BranchesAnalyzed:       branchesAnalyzed,
		ContributingFactors:    bundle.MarginDrivers,
		DataQualityWarnings:    warnings,
		HumanReviewSuggestions: suggestions,
		UnderlyingReportLinks:  links,
		AuditMetadata: AuditMetadata{
			ModelVersion:              modelVersion,
			ExtractionOrPromptVersion: PromptVersion,
			ToolsInvoked:              invoked,
			GeneratedAt:               time.Now().UTC(),
			ReadOnlyEnforced:          true,
		},
	}, nil
}
