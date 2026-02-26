// Package testutil fournit les mocks et factories pour les tests du tracker public.
// Ce package est importé par tests/tracker/*_test.go.
package testutil

import (
	"context"
	"sync"

	"github.com/superselle/ubertracker/tracker"
)

// ══════════════════════════════════════════════════════════════
// MockOrderStore — Implémentation in-memory de tracker.OrderStore
// ══════════════════════════════════════════════════════════════

// Vérification compile-time : MockOrderStore satisfait tracker.OrderStore.
var _ tracker.OrderStore = (*MockOrderStore)(nil)

type snapshotEntry struct {
	Status   string
	Progress int
	Text     string
	RawJSON  string
}

// MockOrderStore est une implémentation in-memory de tracker.OrderStore pour les tests.
type MockOrderStore struct {
	mu        sync.Mutex
	snapshots map[string]snapshotEntry
	orders    map[string]tracker.TrackedOrder
	messages  map[string]string

	// SaveErr provoque une erreur au prochain SaveOrder si non-nil.
	SaveErr error
	// SnapshotErr provoque une erreur au prochain GetSnapshot si non-nil.
	SnapshotErr error
}

// NewMockOrderStore crée un MockOrderStore vide prêt à l'emploi.
func NewMockOrderStore() *MockOrderStore {
	return &MockOrderStore{
		snapshots: make(map[string]snapshotEntry),
		orders:    make(map[string]tracker.TrackedOrder),
		messages:  make(map[string]string),
	}
}

func (m *MockOrderStore) GetSnapshot(_ context.Context, uuid string) (string, int, string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SnapshotErr != nil {
		err := m.SnapshotErr
		m.SnapshotErr = nil
		return "", 0, "", "", err
	}

	s, ok := m.snapshots[uuid]
	if !ok {
		return "", 0, "", "", nil
	}
	return s.Status, s.Progress, s.Text, s.RawJSON, nil
}

func (m *MockOrderStore) SaveOrder(_ context.Context, order tracker.TrackedOrder) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SaveErr != nil {
		err := m.SaveErr
		m.SaveErr = nil
		return err
	}

	m.orders[order.UUID] = order
	m.snapshots[order.UUID] = snapshotEntry{
		Status:   order.LastStatus,
		Progress: order.LastProgress,
		Text:     order.LastText,
		RawJSON:  order.FullJSONData,
	}
	if order.MessageID != "" {
		m.messages[order.UUID] = order.MessageID
	}
	return nil
}

func (m *MockOrderStore) GetMessageID(_ context.Context, uuid string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messages[uuid], nil
}

func (m *MockOrderStore) GetPendingOrders(_ context.Context) (map[string]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending := make(map[string]int)
	for uuid, order := range m.orders {
		switch order.LastStatus {
		case "COMPLETED", "DELIVERED", "CANCELLED", "FAILED":
			continue
		default:
			pending[uuid] = order.LastProgress
		}
	}
	return pending, nil
}

func (m *MockOrderStore) ListResumableOrders(_ context.Context) ([]tracker.ResumableOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var resumable []tracker.ResumableOrder
	for _, order := range m.orders {
		switch order.LastStatus {
		case "COMPLETED", "DELIVERED", "CANCELLED", "FAILED":
			continue
		default:
			resumable = append(resumable, tracker.ResumableOrder{
				UUID:      order.UUID,
				ChannelID: order.ChannelID,
				GuildID:   order.GuildID,
				ClientID:  order.ClientID,
				CuistotID: order.CuistotID,
			})
		}
	}
	return resumable, nil
}

// ── Méthodes utilitaires pour les assertions ──

// GetOrder retourne la dernière version sauvée d'une commande.
func (m *MockOrderStore) GetOrder(uuid string) (tracker.TrackedOrder, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[uuid]
	return o, ok
}

// OrderCount retourne le nombre de commandes enregistrées.
func (m *MockOrderStore) OrderCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.orders)
}

// Reset vide toutes les données du mock.
func (m *MockOrderStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots = make(map[string]snapshotEntry)
	m.orders = make(map[string]tracker.TrackedOrder)
	m.messages = make(map[string]string)
	m.SaveErr = nil
	m.SnapshotErr = nil
}

// SeedSnapshot injects a previous snapshot to simulate an existing order in store.
func (m *MockOrderStore) SeedSnapshot(uuid, status string, progress int, text, rawJSON string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots[uuid] = snapshotEntry{
		Status:   status,
		Progress: progress,
		Text:     text,
		RawJSON:  rawJSON,
	}
}
