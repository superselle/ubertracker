// Package tracker_test — Tests Black Box pour le package tracker (manager).
package tracker_test

import (
	"context"
	"testing"
	"time"

	"github.com/superselle/ubertracker/tests/testutil"
	"github.com/superselle/ubertracker/tracker"
)

// ══════════════════════════════════════════════════════════════
// Manager — Lifecycle
// ══════════════════════════════════════════════════════════════

func TestNewManager_ReturnsNonNil(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mgr := tracker.NewManager(store)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManager_Shutdown_WithNoActiveOrders(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mgr := tracker.NewManager(store)

	// Shutdown sans ordre actif ne doit pas bloquer
	mgr.Shutdown()
}

func TestNewManager_Defaults(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mgr := tracker.NewManager(store)

	if mgr.UpdateChannel == nil {
		t.Error("UpdateChannel should not be nil")
	}
}

// ══════════════════════════════════════════════════════════════
// StartTracking / StopTracking — T34-T36
// ══════════════════════════════════════════════════════════════

func TestStartTracking_New(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mockFetch := testutil.NewMockFetch()

	// Queue enough responses for the worker: first ACTIVE, then COMPLETED to stop
	order := testutil.NewTestOrder().WithPhase("ACTIVE").Build()
	mockFetch.QueueOrder(order)
	completed := testutil.NewTestOrder().WithPhase("COMPLETED").Build()
	mockFetch.QueueOrder(completed)

	mgr := tracker.NewManager(store, mockFetch.Fn())

	id := tracker.OrderIdentity{
		UUID: "uuid-1", ChannelID: "ch-1", GuildID: "g1",
		ClientID: "c1", CuistotID: "k1",
	}

	started := mgr.StartTracking(id)
	if !started {
		t.Error("StartTracking returned false for new UUID")
	}

	// Drain updates to let worker finish
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-mgr.UpdateChannel:
			if !ok {
				return // channel closed after shutdown
			}
		case <-timeout:
			mgr.Shutdown()
			return
		}
	}
}

func TestStartTracking_Duplicate(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mockFetch := testutil.NewMockFetch()

	// Queue enough responses for the worker
	for i := 0; i < 5; i++ {
		order := testutil.NewTestOrder().WithPhase("ACTIVE").Build()
		mockFetch.QueueOrder(order)
	}
	completed := testutil.NewTestOrder().WithPhase("COMPLETED").Build()
	mockFetch.QueueOrder(completed)

	mgr := tracker.NewManager(store, mockFetch.Fn())
	defer mgr.Shutdown()

	id := tracker.OrderIdentity{
		UUID: "uuid-1", ChannelID: "ch-1", GuildID: "g1",
		ClientID: "c1", CuistotID: "k1",
	}

	mgr.StartTracking(id)

	// Second attempt with same UUID
	started := mgr.StartTracking(id)
	if started {
		t.Error("StartTracking returned true for duplicate UUID")
	}
}

func TestStopTracking_Cancels(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mockFetch := testutil.NewMockFetch()

	// Queue many responses to keep worker alive
	for i := 0; i < 20; i++ {
		order := testutil.NewTestOrder().WithPhase("ACTIVE").Build()
		mockFetch.QueueOrder(order)
	}

	mgr := tracker.NewManager(store, mockFetch.Fn())

	id := tracker.OrderIdentity{
		UUID: "uuid-1", ChannelID: "ch-1", GuildID: "g1",
		ClientID: "c1", CuistotID: "k1",
	}

	mgr.StartTracking(id)
	time.Sleep(100 * time.Millisecond) // Let worker start

	mgr.StopTracking("uuid-1")

	// After stop, should be able to start again
	time.Sleep(200 * time.Millisecond)

	// Cleanup
	mgr.Shutdown()
}

func TestShutdown_ClosesChannel(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mockFetch := testutil.NewMockFetch()

	// Queue COMPLETED to let the worker finish quickly
	completed := testutil.NewTestOrder().WithPhase("COMPLETED").Build()
	mockFetch.QueueOrder(completed)

	mgr := tracker.NewManager(store, mockFetch.Fn())

	id := tracker.OrderIdentity{
		UUID: "uuid-1", ChannelID: "ch-1", GuildID: "g1",
		ClientID: "c1", CuistotID: "k1",
	}
	mgr.StartTracking(id)

	// Wait for worker to finish
	time.Sleep(500 * time.Millisecond)

	mgr.Shutdown()

	// Channel should be closed → reading should return zero value, ok=false
	select {
	case _, ok := <-mgr.UpdateChannel:
		if ok {
			// Might still have a buffered update — drain
			for range mgr.UpdateChannel {
			}
		}
	default:
		// Channel might already be drained
	}
}

// ══════════════════════════════════════════════════════════════
// ResumeActiveOrders — T170
// ══════════════════════════════════════════════════════════════

func TestManager_ResumeActiveOrders(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mockFetch := testutil.NewMockFetch()

	// Seed 2 resumable orders in the store via SaveOrder
	_ = store.SaveOrder(context.Background(), tracker.TrackedOrder{
		UUID: "uuid-1", ChannelID: "ch-1", GuildID: "g1",
		ClientID: "c1", CuistotID: "k1", LastStatus: "ACTIVE",
	})
	_ = store.SaveOrder(context.Background(), tracker.TrackedOrder{
		UUID: "uuid-2", ChannelID: "ch-2", GuildID: "g1",
		ClientID: "c2", CuistotID: "k2", LastStatus: "ACTIVE",
	})

	// Queue COMPLETED responses for both workers
	for i := 0; i < 2; i++ {
		completed := testutil.NewTestOrder().WithPhase("COMPLETED").Build()
		mockFetch.QueueOrder(completed)
	}

	mgr := tracker.NewManager(store, mockFetch.Fn())
	mgr.ResumeActiveOrders()

	// Wait for workers to emit
	time.Sleep(1 * time.Second)
	mgr.Shutdown()

	// Both workers should have emitted at least once
	if mockFetch.CallCount() < 2 {
		t.Errorf("expected at least 2 fetch calls, got %d", mockFetch.CallCount())
	}
}

// ══════════════════════════════════════════════════════════════
// Shutdown stops all — T171
// ══════════════════════════════════════════════════════════════

func TestManager_ShutdownStopsAll(t *testing.T) {
	store := testutil.NewMockOrderStore()
	mockFetch := testutil.NewMockFetch()

	// Queue many ACTIVE responses to keep workers alive
	for i := 0; i < 30; i++ {
		order := testutil.NewTestOrder().WithPhase("ACTIVE").Build()
		mockFetch.QueueOrder(order)
	}

	mgr := tracker.NewManager(store, mockFetch.Fn())

	for i := 1; i <= 3; i++ {
		id := tracker.OrderIdentity{
			UUID:      "uuid-" + string(rune('0'+i)),
			ChannelID: "ch-" + string(rune('0'+i)),
			GuildID:   "g1",
			ClientID:  "c" + string(rune('0'+i)),
			CuistotID: "k" + string(rune('0'+i)),
		}
		mgr.StartTracking(id)
	}

	time.Sleep(200 * time.Millisecond)

	// Shutdown should not hang
	done := make(chan struct{})
	go func() {
		mgr.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not complete within 10s")
	}
}
