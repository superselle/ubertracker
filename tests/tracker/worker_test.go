// Package tracker_test — Tests Black Box pour le package tracker (worker).
package tracker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/superselle/ubertracker/tests/testutil"
	"github.com/superselle/ubertracker/tracker"
)

func mustMarshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// ══════════════════════════════════════════════════════════════
// Tests SafeTruncate — T20-T23
// ══════════════════════════════════════════════════════════════

func TestSafeTruncate_Short(t *testing.T) {
	got := tracker.SafeTruncate("abc", 10)
	if got != "abc" {
		t.Errorf("SafeTruncate('abc', 10) = %q, want 'abc'", got)
	}
}

func TestSafeTruncate_Exact(t *testing.T) {
	got := tracker.SafeTruncate("abc", 3)
	if got != "abc" {
		t.Errorf("SafeTruncate('abc', 3) = %q, want 'abc'", got)
	}
}

func TestSafeTruncate_Truncated(t *testing.T) {
	got := tracker.SafeTruncate("abcdef", 3)
	if got != "abc" {
		t.Errorf("SafeTruncate('abcdef', 3) = %q, want 'abc'", got)
	}
}

func TestSafeTruncate_Empty(t *testing.T) {
	got := tracker.SafeTruncate("", 5)
	if got != "" {
		t.Errorf("SafeTruncate('', 5) = %q, want ''", got)
	}
}

// ══════════════════════════════════════════════════════════════
// Tests AdaptiveInterval — T24-T29
// ══════════════════════════════════════════════════════════════

func TestAdaptiveInterval_ETAClose(t *testing.T) {
	got := tracker.AdaptiveInterval(3, 0)
	if got != 15 {
		t.Errorf("AdaptiveInterval(3, 0) = %d, want 15", got)
	}
}

func TestAdaptiveInterval_ETAMedium(t *testing.T) {
	got := tracker.AdaptiveInterval(10, 0)
	if got != 25 {
		t.Errorf("AdaptiveInterval(10, 0) = %d, want 25", got)
	}
}

func TestAdaptiveInterval_ETAFar(t *testing.T) {
	got := tracker.AdaptiveInterval(20, 0)
	if got != 30 {
		t.Errorf("AdaptiveInterval(20, 0) = %d, want 30", got)
	}
}

func TestAdaptiveInterval_Backoff(t *testing.T) {
	got := tracker.AdaptiveInterval(-1, 5)
	if got != 80 {
		t.Errorf("AdaptiveInterval(-1, 5) = %d, want 80", got)
	}
}

func TestAdaptiveInterval_MaxCap(t *testing.T) {
	got := tracker.AdaptiveInterval(-1, 20)
	if got != 120 {
		t.Errorf("AdaptiveInterval(-1, 20) = %d, want 120", got)
	}
}

func TestAdaptiveInterval_ETAUnknown(t *testing.T) {
	got := tracker.AdaptiveInterval(-1, 0)
	if got != 30 {
		t.Errorf("AdaptiveInterval(-1, 0) = %d, want 30", got)
	}
}

// ══════════════════════════════════════════════════════════════
// Tests AdaptiveInterval — additional boundaries
// ══════════════════════════════════════════════════════════════

func TestAdaptiveInterval_ETAZero(t *testing.T) {
	got := tracker.AdaptiveInterval(0, 0)
	if got != 15 {
		t.Errorf("AdaptiveInterval(0, 0) = %d, want 15", got)
	}
}

func TestAdaptiveInterval_ETAFive(t *testing.T) {
	got := tracker.AdaptiveInterval(5, 0)
	if got != 15 {
		t.Errorf("AdaptiveInterval(5, 0) = %d, want 15", got)
	}
}

func TestAdaptiveInterval_ETASix(t *testing.T) {
	got := tracker.AdaptiveInterval(6, 0)
	if got != 25 {
		t.Errorf("AdaptiveInterval(6, 0) = %d, want 25", got)
	}
}

func TestAdaptiveInterval_ETAFifteen(t *testing.T) {
	got := tracker.AdaptiveInterval(15, 0)
	if got != 25 {
		t.Errorf("AdaptiveInterval(15, 0) = %d, want 25", got)
	}
}

func TestAdaptiveInterval_ETASixteen(t *testing.T) {
	got := tracker.AdaptiveInterval(16, 0)
	if got != 30 {
		t.Errorf("AdaptiveInterval(16, 0) = %d, want 30", got)
	}
}

// ══════════════════════════════════════════════════════════════
// Tests Worker Integration — T167-T169
// ══════════════════════════════════════════════════════════════

func TestWorker_EmitsOnChannel(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mockFetch := testutil.NewMockFetch()
	updates := make(chan tracker.TrackedOrder, 10)

	// Queue a valid order response
	order := testutil.NewTestOrder().WithPhase("ACTIVE").Build()
	mockFetch.QueueOrder(order)
	// Then queue COMPLETED to stop the worker
	completedOrder := testutil.NewTestOrder().WithPhase("COMPLETED").Build()
	mockFetch.QueueOrder(completedOrder)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id := tracker.OrderIdentity{
		UUID: "test-uuid", ChannelID: "ch-1", GuildID: "g1",
		ClientID: "client-1", CuistotID: "cuistot-1",
	}

	go tracker.StartOrderWorker(ctx, store, id, updates, mockFetch.Fn())

	// Should receive at least one update
	select {
	case tracked := <-updates:
		if tracked.UUID != "test-uuid" {
			t.Errorf("tracked.UUID = %q, want test-uuid", tracked.UUID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for update on channel")
	}
}

func TestWorker_StopsOnCompleted(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mockFetch := testutil.NewMockFetch()
	updates := make(chan tracker.TrackedOrder, 10)

	// First poll → COMPLETED immediately
	completedOrder := testutil.NewTestOrder().WithPhase("COMPLETED").Build()
	mockFetch.QueueOrder(completedOrder)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id := tracker.OrderIdentity{
		UUID: "test-uuid", ChannelID: "ch-1", GuildID: "g1",
		ClientID: "client-1", CuistotID: "cuistot-1",
	}

	done := make(chan struct{})
	go func() {
		tracker.StartOrderWorker(ctx, store, id, updates, mockFetch.Fn())
		close(done)
	}()

	// Worker should exit quickly
	select {
	case <-done:
		// OK — worker stopped
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop after COMPLETED")
	}
}

func TestWorker_ErrorOnFirstScan_CancelsCleanly(t *testing.T) {
	// Après une erreur au premier scan, le worker entre dans la boucle de polling.
	// On vérifie qu'il peut être arrêté proprement par le contexte.
	// Note : TestWorker_StopsOnMaxFails (T169) est impraticable car le worker
	// dort 30-50s entre chaque retry (10 fails × 30s = 5 min minimum).
	store := testutil.NewMockOrderStore()
	mockFetch := testutil.NewMockFetch()
	updates := make(chan tracker.TrackedOrder, 20)

	// First call: error
	mockFetch.QueueError(fmt.Errorf("transient error"))

	ctx, cancel := context.WithCancel(context.Background())

	id := tracker.OrderIdentity{
		UUID: "test-uuid", ChannelID: "ch-1", GuildID: "g1",
		ClientID: "client-1", CuistotID: "cuistot-1",
	}

	done := make(chan struct{})
	go func() {
		tracker.StartOrderWorker(ctx, store, id, updates, mockFetch.Fn())
		close(done)
	}()

	// Let the first scan error happen
	time.Sleep(100 * time.Millisecond)

	// Cancel the worker — should exit the sleep
	cancel()

	select {
	case <-done:
		// OK — worker exited cleanly
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not exit after context cancel")
	}
}

func TestWorker_ContextCancel(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mockFetch := testutil.NewMockFetch()
	updates := make(chan tracker.TrackedOrder, 10)

	// Queue a valid ACTIVE order, then cancel context before next poll
	order := testutil.NewTestOrder().WithPhase("ACTIVE").Build()
	mockFetch.QueueOrder(order)
	// Queue more to keep worker alive if not cancelled
	for i := 0; i < 5; i++ {
		mockFetch.QueueOrder(order)
	}

	ctx, cancel := context.WithCancel(context.Background())

	id := tracker.OrderIdentity{
		UUID: "test-uuid", ChannelID: "ch-1", GuildID: "g1",
		ClientID: "client-1", CuistotID: "cuistot-1",
	}

	done := make(chan struct{})
	go func() {
		tracker.StartOrderWorker(ctx, store, id, updates, mockFetch.Fn())
		close(done)
	}()

	// Wait for first update, then cancel
	select {
	case <-updates:
		cancel()
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("timeout waiting for first update")
	}

	select {
	case <-done:
		// OK — worker stopped
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop after context cancel")
	}
}

// ══════════════════════════════════════════════════════════════
// Reconcile — shouldEmit logic (T30–T32)
// ══════════════════════════════════════════════════════════════

func TestReconcile_ShouldEmit_FirstPoll(t *testing.T) {
	// T30: No existing snapshot → shouldEmit=true (first time seeing this order)
	store := testutil.NewMockOrderStore()
	ctx := context.Background()

	resp := testutil.NewTestOrder().
		WithPhase("ACTIVE").
		WithProgress(2, 5).
		WithStatusText("En préparation").
		BuildResponse()

	result, err := tracker.Reconcile(ctx, store, "uuid-first", resp)
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}
	if !result.ShouldEmit {
		t.Error("ShouldEmit = false on first poll, want true")
	}
	if result.Phase != "ACTIVE" {
		t.Errorf("Phase = %q, want ACTIVE", result.Phase)
	}
}

func TestReconcile_ShouldEmit_ProgressChange(t *testing.T) {
	// T31: Old snapshot has progress=2, new has progress=3 → shouldEmit=true
	store := testutil.NewMockOrderStore()
	ctx := context.Background()

	// Seed a previous snapshot
	prevResp := testutil.NewTestOrder().
		WithPhase("ACTIVE").
		WithProgress(2, 5).
		WithStatusText("En préparation").
		BuildResponse()
	prevJSON := mustMarshalJSON(prevResp)
	store.SeedSnapshot("uuid-progress", "ACTIVE", 2, "En préparation", prevJSON)

	// New response with progress=3
	newResp := testutil.NewTestOrder().
		WithPhase("ACTIVE").
		WithProgress(3, 5).
		WithStatusText("En préparation").
		BuildResponse()

	result, err := tracker.Reconcile(ctx, store, "uuid-progress", newResp)
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}
	if !result.ShouldEmit {
		t.Error("ShouldEmit = false on progress change, want true")
	}
	if result.Progress != 3 {
		t.Errorf("Progress = %d, want 3", result.Progress)
	}
}

func TestReconcile_ShouldEmit_NoChange_StillEmitsDueToETA(t *testing.T) {
	// T32: Same status/progress/text with ETA present → shouldEmit=true
	// NOTE: because the code does `newETA >= 0` → always emits if ETA is present.
	// A truly "no emit" case would need ETA=-1 AND identical status/progress/text.
	store := testutil.NewMockOrderStore()
	ctx := context.Background()

	order := testutil.NewTestOrder().
		WithPhase("ACTIVE").
		WithProgress(2, 5).
		WithStatusText("En préparation").
		Build()
	// Remove all background cards so ETA = -1
	order.BackgroundFeedCards = nil

	prevResp := tracker.Response{Data: tracker.Data{Orders: []tracker.Order{order}}}
	prevJSON := mustMarshalJSON(prevResp)
	store.SeedSnapshot("uuid-nochange", "ACTIVE", 2, order.FeedCards[0].Status.StatusSummary.Text, prevJSON)

	// Same order with same everything and no ETA
	newOrder := testutil.NewTestOrder().
		WithPhase("ACTIVE").
		WithProgress(2, 5).
		WithStatusText("En préparation").
		Build()
	newOrder.BackgroundFeedCards = nil
	newResp := tracker.Response{Data: tracker.Data{Orders: []tracker.Order{newOrder}}}

	result, err := tracker.Reconcile(ctx, store, "uuid-nochange", newResp)
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}
	// With identical data and no ETA → shouldEmit=false
	if result.ShouldEmit {
		t.Error("ShouldEmit = true when nothing changed and no ETA, want false")
	}
}
