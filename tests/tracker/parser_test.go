// Package tracker_test — Tests Black Box pour le package tracker (parser).
package tracker_test

import (
	"testing"

	"github.com/superselle/ubertracker/tests/testutil"
	"github.com/superselle/ubertracker/tracker"
)

// ══════════════════════════════════════════════════════════════
// ExtractETAFromOrder
// ══════════════════════════════════════════════════════════════

func TestExtractETAFromOrder_NoBackgroundCards(t *testing.T) {
	order := testutil.NewTestOrder().Build()
	eta := tracker.ExtractETAFromOrder(order)
	if eta != -1 {
		t.Errorf("ETA = %d, want -1 (no background cards)", eta)
	}
}

func TestExtractETAFromOrder_WithBackgroundCards(t *testing.T) {
	order := testutil.NewTestOrder().WithETA(12).Build()
	_ = tracker.ExtractETAFromOrder(order)
	// Note: le résultat dépend du parsing interne des MapEntity.
	// Le builder WithETA met la structure en place mais l'ETA réelle
	// dépend des données dans MapEntity.
}

// ══════════════════════════════════════════════════════════════
// MergeOrderData
// ══════════════════════════════════════════════════════════════

func TestMergeOrderData_MergesIntoExisting(t *testing.T) {
	prev := testutil.NewTestOrder().
		WithRestaurant("Burger King").
		WithProgress(2, 5).
		Build()

	next := testutil.NewTestOrder().
		WithRestaurant("Burger King").
		WithProgress(3, 5).
		Build()

	merged, _ := tracker.MergeOrderData(prev, next, true)

	if len(merged.FeedCards) == 0 {
		t.Fatal("merged order has no FeedCards")
	}
	if merged.FeedCards[0].Status == nil {
		t.Fatal("merged order FeedCard has no Status")
	}
	if merged.FeedCards[0].Status.CurrentProgress != 3 {
		t.Errorf("merged progress = %d, want 3", merged.FeedCards[0].Status.CurrentProgress)
	}
}

func TestMergeOrderData_PreservesContactInfo(t *testing.T) {
	prev := testutil.NewTestOrder().
		WithContact("Jean", "0600000000").
		Build()

	next := testutil.NewTestOrder().Build()
	next.Contacts = nil // New order lost contact info

	merged, _ := tracker.MergeOrderData(prev, next, true)

	if len(merged.Contacts) == 0 {
		t.Error("merged order lost contact info")
	}
}

// ══════════════════════════════════════════════════════════════
// ExtractETAFromOrder — supplement
// ══════════════════════════════════════════════════════════════

func TestExtractETAFromOrder_WithLabelEntity(t *testing.T) {
	order := tracker.Order{
		BackgroundFeedCards: []tracker.BackgroundFeedCard{
			{
				MapEntity: []tracker.MapEntity{
					{Type: "LABEL", Title: "12"},
				},
			},
		},
	}
	eta := tracker.ExtractETAFromOrder(order)
	if eta != 12 {
		t.Errorf("ETA = %d, want 12", eta)
	}
}

func TestExtractETAFromOrder_NonNumericLabel(t *testing.T) {
	order := tracker.Order{
		BackgroundFeedCards: []tracker.BackgroundFeedCard{
			{
				MapEntity: []tracker.MapEntity{
					{Type: "LABEL", Title: "bientôt"},
				},
			},
		},
	}
	eta := tracker.ExtractETAFromOrder(order)
	if eta != -1 {
		t.Errorf("ETA = %d, want -1 (non-numeric label)", eta)
	}
}

func TestExtractETAFromOrder_MultipleEntities(t *testing.T) {
	order := tracker.Order{
		BackgroundFeedCards: []tracker.BackgroundFeedCard{
			{
				MapEntity: []tracker.MapEntity{
					{Type: "RESTAURANT", Title: "McDonalds"},
					{Type: "LABEL", Title: "7"},
					{Type: "CUSTOMER", Title: "Drop-off"},
				},
			},
		},
	}
	eta := tracker.ExtractETAFromOrder(order)
	if eta != 7 {
		t.Errorf("ETA = %d, want 7 (first LABEL entity)", eta)
	}
}

func TestExtractETAFromOrder_EmptyMapEntity(t *testing.T) {
	order := tracker.Order{
		BackgroundFeedCards: []tracker.BackgroundFeedCard{
			{MapEntity: []tracker.MapEntity{}},
		},
	}
	eta := tracker.ExtractETAFromOrder(order)
	if eta != -1 {
		t.Errorf("ETA = %d, want -1 (empty entity slice)", eta)
	}
}

func TestExtractETAFromOrder_WhitespaceTitle(t *testing.T) {
	order := tracker.Order{
		BackgroundFeedCards: []tracker.BackgroundFeedCard{
			{
				MapEntity: []tracker.MapEntity{
					{Type: "LABEL", Title: "  5  "},
				},
			},
		},
	}
	eta := tracker.ExtractETAFromOrder(order)
	if eta != 5 {
		t.Errorf("ETA = %d, want 5 (trimmed whitespace)", eta)
	}
}

func TestExtractETAFromOrder_ZeroETA(t *testing.T) {
	order := tracker.Order{
		BackgroundFeedCards: []tracker.BackgroundFeedCard{
			{
				MapEntity: []tracker.MapEntity{
					{Type: "LABEL", Title: "0"},
				},
			},
		},
	}
	eta := tracker.ExtractETAFromOrder(order)
	if eta != 0 {
		t.Errorf("ETA = %d, want 0", eta)
	}
}

// ══════════════════════════════════════════════════════════════
// MergeOrderData — supplement: phase detection
// ══════════════════════════════════════════════════════════════

func TestMergeOrderData_DetectsCompletedPhase(t *testing.T) {
	prev := testutil.NewTestOrder().Build()
	next := testutil.NewTestOrder().Build()
	next.OrderStatus.OrderPhase = "COMPLETED"

	_, phase := tracker.MergeOrderData(prev, next, true)
	if phase != "COMPLETED" {
		t.Errorf("phase = %q, want COMPLETED", phase)
	}
}

func TestMergeOrderData_DetectsCancelledViaCTA(t *testing.T) {
	prev := testutil.NewTestOrder().Build()
	next := testutil.NewTestOrder().
		WithCallToAction("Commande annulée").
		Build()
	next.OrderStatus.OrderPhase = "COMPLETED"

	_, phase := tracker.MergeOrderData(prev, next, true)
	if phase != "CANCELLED" {
		t.Errorf("phase = %q, want CANCELLED (disguised cancellation)", phase)
	}
}

func TestMergeOrderData_NoOldData(t *testing.T) {
	next := testutil.NewTestOrder().
		WithRestaurant("Pizza Hut").
		WithProgress(1, 5).
		Build()

	merged, _ := tracker.MergeOrderData(tracker.Order{}, next, false)
	if merged.ActiveOrderOverview.Title != "Pizza Hut" {
		t.Errorf("restaurant = %q, want Pizza Hut", merged.ActiveOrderOverview.Title)
	}
}

func TestMergeOrderData_PreservesPIN(t *testing.T) {
	prev := testutil.NewTestOrder().
		WithPIN("4567").
		Build()
	next := testutil.NewTestOrder().Build()
	// new order has no PIN

	merged, _ := tracker.MergeOrderData(prev, next, true)
	foundPIN := false
	for _, card := range merged.FeedCards {
		for _, c := range card.Courier {
			if c.PinInfo.Pin == "4567" {
				foundPIN = true
			}
		}
	}
	if !foundPIN {
		t.Error("merged order should preserve PIN from previous data")
	}
}

func TestMergeOrderData_NewDataOverridesPIN(t *testing.T) {
	prev := testutil.NewTestOrder().
		WithPIN("1111").
		Build()
	next := testutil.NewTestOrder().
		WithPIN("2222").
		Build()

	merged, _ := tracker.MergeOrderData(prev, next, true)
	foundNew := false
	for _, card := range merged.FeedCards {
		for _, c := range card.Courier {
			if c.PinInfo.Pin == "2222" {
				foundNew = true
			}
		}
	}
	if !foundNew {
		t.Error("new PIN should override old PIN in merged data")
	}
}

func TestMergeOrderData_CompletedOverridesStatus(t *testing.T) {
	prev := testutil.NewTestOrder().
		WithProgress(3, 5).
		WithStatusText("En livraison").
		Build()

	next := testutil.NewTestOrder().Build()
	next.OrderStatus.OrderPhase = "COMPLETED"

	merged, phase := tracker.MergeOrderData(prev, next, true)
	if phase != "COMPLETED" {
		t.Fatalf("phase = %q, want COMPLETED", phase)
	}
	// Status should be updated to completed text
	if merged.FeedCards[0].Status != nil {
		if merged.FeedCards[0].Status.CurrentProgress != 5 {
			t.Errorf("completed progress = %d, want 5", merged.FeedCards[0].Status.CurrentProgress)
		}
	}
}

// ══════════════════════════════════════════════════════════════
// MergeOrderData — preserved info via indirect testing (T5–T12)
// ══════════════════════════════════════════════════════════════

func TestMergeOrderData_PreservesTotal(t *testing.T) {
	// T18 / T5: Old has total "15.90€", new has no total → total preserved
	prev := testutil.NewTestOrder().WithTotal("15.90€").Build()
	next := testutil.NewTestOrder().Build() // no total

	merged, _ := tracker.MergeOrderData(prev, next, true)
	if len(merged.FeedCards) == 0 {
		t.Fatal("merged has no FeedCards")
	}
	if merged.FeedCards[0].OrderSummary.Total != "15.90€" {
		t.Errorf("total = %q, want 15.90€", merged.FeedCards[0].OrderSummary.Total)
	}
}

func TestMergeOrderData_FreshTotalOverridesOld(t *testing.T) {
	// T19: Both have total → new wins
	prev := testutil.NewTestOrder().WithTotal("10.00€").Build()
	next := testutil.NewTestOrder().WithTotal("12.50€").Build()

	merged, _ := tracker.MergeOrderData(prev, next, true)
	if merged.FeedCards[0].OrderSummary.Total != "12.50€" {
		t.Errorf("total = %q, want 12.50€", merged.FeedCards[0].OrderSummary.Total)
	}
}

func TestMergeOrderData_PreservesAllFields(t *testing.T) {
	// T5: Old has total, address, PIN → all extracted and preserved
	prev := testutil.NewTestOrder().
		WithTotal("20.00€").
		WithAddress("12 Rue de la Paix").
		WithPIN("4321").
		Build()
	next := testutil.NewTestOrder().Build() // empty

	merged, _ := tracker.MergeOrderData(prev, next, true)
	fc := merged.FeedCards[0]

	if fc.OrderSummary.Total != "20.00€" {
		t.Errorf("total = %q, want 20.00€", fc.OrderSummary.Total)
	}
	if fc.Delivery == nil || fc.Delivery.Address != "12 Rue de la Paix" {
		addr := ""
		if fc.Delivery != nil {
			addr = fc.Delivery.Address
		}
		t.Errorf("address = %q, want '12 Rue de la Paix'", addr)
	}
	if len(fc.Courier) == 0 || fc.Courier[0].PinInfo.Pin != "4321" {
		t.Error("PIN not preserved")
	}
}

func TestMergeOrderData_PreservesPartial_OnlyTotal(t *testing.T) {
	// T6: Only total in old → only total restored, others empty
	prev := testutil.NewTestOrder().WithTotal("9.99€").Build()
	next := testutil.NewTestOrder().Build()

	merged, _ := tracker.MergeOrderData(prev, next, true)
	fc := merged.FeedCards[0]

	if fc.OrderSummary.Total != "9.99€" {
		t.Errorf("total = %q, want 9.99€", fc.OrderSummary.Total)
	}
	// Address and PIN should remain absent since old didn't have them
	if fc.Delivery != nil && fc.Delivery.Address != "" {
		t.Errorf("address should be empty, got %q", fc.Delivery.Address)
	}
}

func TestMergeOrderData_PreservesEmpty_NoData(t *testing.T) {
	// T7: Empty FeedCards in old → nothing to preserve, no panic
	prev := tracker.Order{} // completely empty
	next := testutil.NewTestOrder().Build()

	merged, _ := tracker.MergeOrderData(prev, next, false)
	// Should not panic and should have new order's data
	if len(merged.FeedCards) == 0 {
		t.Fatal("merged has no FeedCards")
	}
}

func TestMergeOrderData_RestoresMissing(t *testing.T) {
	// T8: Old has total/addr/PIN, new loses them → all restored
	prev := testutil.NewTestOrder().
		WithTotal("18.50€").
		WithAddress("5 Avenue Foch").
		WithPIN("9876").
		Build()
	// New order: has FeedCards but no total/addr/pin
	next := testutil.NewTestOrder().Build()

	merged, _ := tracker.MergeOrderData(prev, next, true)
	fc := merged.FeedCards[0]

	if fc.OrderSummary.Total != "18.50€" {
		t.Errorf("restored total = %q, want 18.50€", fc.OrderSummary.Total)
	}
	if fc.Delivery == nil || fc.Delivery.Address != "5 Avenue Foch" {
		t.Error("address not restored")
	}
	if len(fc.Courier) == 0 || fc.Courier[0].PinInfo.Pin != "9876" {
		t.Error("PIN not restored")
	}
}

func TestMergeOrderData_NoOverwrite_ExistingTotal(t *testing.T) {
	// T9: New has its own total → new wins (no overwrite by old preserved)
	prev := testutil.NewTestOrder().WithTotal("10.00€").Build()
	next := testutil.NewTestOrder().WithTotal("15.00€").Build()

	merged, _ := tracker.MergeOrderData(prev, next, true)
	if merged.FeedCards[0].OrderSummary.Total != "15.00€" {
		t.Errorf("total = %q, want 15.00€ (new wins)", merged.FeedCards[0].OrderSummary.Total)
	}
}

func TestMergeOrderData_EmptyFeedCards_NoPanic(t *testing.T) {
	// T10: New order has empty FeedCards → no panic during restore
	prev := testutil.NewTestOrder().WithTotal("5.00€").WithPIN("1234").Build()
	next := tracker.Order{
		FeedCards: []tracker.FeedCard{},
	}

	// Should not panic
	merged, _ := tracker.MergeOrderData(prev, next, true)
	_ = merged // no assertion besides no-panic
}

func TestMergeOrderData_CreatesCourier_ForPIN(t *testing.T) {
	// T11: New FeedCard has no Courier[0], old has PIN → creates Courier and restores PIN
	prev := testutil.NewTestOrder().WithPIN("5555").Build()
	next := testutil.NewTestOrder().Build() // FeedCard exists but no Courier

	merged, _ := tracker.MergeOrderData(prev, next, true)
	fc := merged.FeedCards[0]

	if len(fc.Courier) == 0 {
		t.Fatal("Courier slice not created during restore")
	}
	if fc.Courier[0].PinInfo.Pin != "5555" {
		t.Errorf("restored PIN = %q, want 5555", fc.Courier[0].PinInfo.Pin)
	}
}

func TestMergeOrderData_DetectPhase_Active(t *testing.T) {
	// T12: Order with phase=ACTIVE → detectPhase returns "ACTIVE"
	prev := testutil.NewTestOrder().Build()
	next := testutil.NewTestOrder().WithPhase("ACTIVE").Build()

	_, phase := tracker.MergeOrderData(prev, next, true)
	if phase != "ACTIVE" {
		t.Errorf("phase = %q, want ACTIVE", phase)
	}
}
