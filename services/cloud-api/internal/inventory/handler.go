package inventory

import (
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) ListIngredients(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	items, err := h.repo.ListIngredients(r.Context(), claims.TenantID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "INGREDIENTS_LIST_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items, map[string]int{"count": len(items)})
}

func (h *Handler) UpsertIngredient(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in Ingredient
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid ingredient payload", err.Error())
		return
	}
	out, err := h.repo.UpsertIngredient(r.Context(), claims.TenantID, in)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INGREDIENT_SAVE_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out, nil)
}

func (h *Handler) ListRecipes(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	targetType := r.URL.Query().Get("target_type")
	targetID := r.URL.Query().Get("target_id")
	if targetID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "MISSING_TARGET_ID", "target_id query parameter is required", nil)
		return
	}
	versions, err := h.repo.ListRecipeVersions(r.Context(), claims.TenantID, targetType, targetID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "RECIPES_LIST_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, versions, map[string]int{"count": len(versions)})
}

func (h *Handler) PublishRecipe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in RecipeVersion
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid recipe payload", err.Error())
		return
	}
	out, err := h.repo.PublishRecipeVersion(r.Context(), claims.TenantID, in)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "RECIPE_PUBLISH_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, out, nil)
}

func (h *Handler) SimulateCost(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in struct {
		Quantity        decimal.Decimal  `json:"quantity"`
		MenuPrice       decimal.Decimal  `json:"menu_price"`
		ItemRecipe      *RecipeVersion   `json:"item_recipe"`
		ModifierRecipes []*RecipeVersion `json:"modifier_recipes"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid simulation payload", err.Error())
		return
	}
	if in.Quantity.LessThanOrEqual(decimal.Zero) {
		in.Quantity = decimal.NewFromInt(1)
	}
	allIngs, err := h.repo.ListIngredients(r.Context(), claims.TenantID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "INGREDIENTS_LOAD_FAILED", err.Error(), nil)
		return
	}
	ingMap := make(map[string]Ingredient, len(allIngs))
	for _, ing := range allIngs {
		ingMap[ing.ID] = ing
	}
	res := ExplodeOrderLine(in.Quantity, in.MenuPrice, in.ItemRecipe, in.ModifierRecipes, ingMap)
	httpx.WriteJSON(w, http.StatusOK, res, nil)
}

func (h *Handler) PostStockMovement(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in MovementInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid stock movement payload", err.Error())
		return
	}
	in.TenantID = claims.TenantID

	var applied bool
	err := h.repo.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		var txErr error
		applied, txErr = PostStockMovementTx(r.Context(), tx, in)
		return txErr
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "MOVEMENT_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"applied":         applied,
		"idempotency_key": in.IdempotencyKey,
		"movement_type":   in.MovementType,
	}, nil)
}
