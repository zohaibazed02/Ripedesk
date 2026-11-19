package inventory

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

var (
	ErrUnmappedUnit    = errors.New("unmapped or ambiguous unit conversion")
	ErrInvalidQuantity = errors.New("quantity must be greater than zero")
)

type UnitConversion struct {
	ID           string          `json:"id,omitempty"`
	IngredientID string          `json:"ingredient_id,omitempty"`
	FromUnit     string          `json:"from_unit"`
	ToUnit       string          `json:"to_unit"`
	Multiplier   decimal.Decimal `json:"multiplier"`
}

// ConvertToBaseUnit deterministically normalizes qty from fromUnit into baseUnit
// using a breadth-first search over the ingredient's explicit unit_conversions graph.
// Returns exact NUMERIC(19,6) rounded Decimal or ErrUnmappedUnit if no path exists.
func ConvertToBaseUnit(baseUnit string, qty decimal.Decimal, fromUnit string, conversions []UnitConversion) (decimal.Decimal, error) {
	if qty.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, ErrInvalidQuantity
	}
	src := strings.TrimSpace(fromUnit)
	dst := strings.TrimSpace(baseUnit)
	if strings.EqualFold(src, dst) {
		return qty.Round(6), nil
	}

	// Build directed adjacency map: fromUnit -> (toUnit, multiplier)
	type edge struct {
		to   string
		mult decimal.Decimal
	}
	adj := make(map[string][]edge)
	for _, c := range conversions {
		if c.Multiplier.LessThanOrEqual(decimal.Zero) {
			continue
		}
		fromKey := strings.ToLower(strings.TrimSpace(c.FromUnit))
		toKey := strings.ToLower(strings.TrimSpace(c.ToUnit))
		adj[fromKey] = append(adj[fromKey], edge{to: toKey, mult: c.Multiplier})
	}

	start := strings.ToLower(src)
	target := strings.ToLower(dst)

	type node struct {
		unit   string
		factor decimal.Decimal
	}
	queue := []node{{unit: start, factor: decimal.NewFromInt(1)}}
	visited := map[string]bool{start: true}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, e := range adj[cur.unit] {
			nextFactor := cur.factor.Mul(e.mult)
			if e.to == target {
				return qty.Mul(nextFactor).Round(6), nil
			}
			if !visited[e.to] {
				visited[e.to] = true
				queue = append(queue, node{unit: e.to, factor: nextFactor})
			}
		}
	}

	return decimal.Zero, fmt.Errorf("%w: cannot convert %q to base unit %q", ErrUnmappedUnit, fromUnit, baseUnit)
}
