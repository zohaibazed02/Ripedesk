package storefront_test

import (
	"testing"
	"time"

	"github.com/hospitality-os/cloud-api/internal/storefront"
	"github.com/shopspring/decimal"
)

func TestComputeBranchOnlineStatus(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-10 * time.Second)
	future := now.Add(30 * time.Second)

	if online, accept := storefront.ComputeBranchOnlineStatus(nil, now); online || accept {
		t.Fatalf("expected nil heartbeat to be offline")
	}
	if online, accept := storefront.ComputeBranchOnlineStatus(&past, now); online || accept {
		t.Fatalf("expected expired heartbeat to be offline (accepting_web_orders=false)")
	}
	if online, accept := storefront.ComputeBranchOnlineStatus(&future, now); !online || !accept {
		t.Fatalf("expected active heartbeat to be online and accepting web orders")
	}
}

func TestFilterPublishedMenuItemsAndDecimalPricing(t *testing.T) {
	items := []storefront.PublishedMenuItem{
		{
			ID: "1", SKU: "COF-OAT-LG", Name: "Large Oat Latte",
			BasePrice: decimal.RequireFromString("950.00"), OnlineVisible: true, InStock: true,
			Modifiers: []storefront.PublishedModifier{
				{ID: "m1", Name: "Extra Shot", PriceDelta: decimal.RequireFromString("220.00")},
			},
		},
		{
			ID: "2", SKU: "STAFF-MEAL", Name: "Internal Staff Meal",
			BasePrice: decimal.RequireFromString("0.00"), OnlineVisible: false, InStock: true,
		},
	}

	published := storefront.FilterPublishedMenuItems(items)
	if len(published) != 1 || published[0].SKU != "COF-OAT-LG" {
		t.Fatalf("expected only online_visible=true items, got %+v", published)
	}
	if !published[0].BasePrice.Equal(decimal.RequireFromString("950.00")) {
		t.Fatalf("expected exact Decimal price 950.00, got %s", published[0].BasePrice)
	}
}
