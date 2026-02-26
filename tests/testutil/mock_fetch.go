package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/superselle/ubertracker/tracker"
)

// ══════════════════════════════════════════════════════════════
// MockFetchFn — Simule FetchUberJSON avec une file de réponses
// ══════════════════════════════════════════════════════════════

type fetchResponse struct {
	data []byte
	err  error
}

// MockFetchFn retourne des réponses prédéfinies en FIFO.
type MockFetchFn struct {
	mu        sync.Mutex
	responses []fetchResponse
	Calls     int
}

// NewMockFetch crée un MockFetchFn vide.
func NewMockFetch() *MockFetchFn {
	return &MockFetchFn{}
}

// QueueResponse empile une réponse brute.
func (m *MockFetchFn) QueueResponse(data []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, fetchResponse{data: data, err: err})
}

// QueueOrder empile une réponse construite à partir d'un Order (sérialisation auto).
func (m *MockFetchFn) QueueOrder(order tracker.Order) {
	resp := tracker.Response{
		Data: tracker.Data{
			Orders: []tracker.Order{order},
		},
	}
	data, _ := json.Marshal(resp)
	m.QueueResponse(data, nil)
}

// QueueError empile une erreur.
func (m *MockFetchFn) QueueError(err error) {
	m.QueueResponse(nil, err)
}

// Fn retourne la fonction injectable de signature func(ctx, uuid) ([]byte, error).
func (m *MockFetchFn) Fn() func(context.Context, string) ([]byte, error) {
	return func(_ context.Context, _ string) ([]byte, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.Calls++

		if len(m.responses) == 0 {
			return nil, fmt.Errorf("MockFetchFn: no more queued responses (call #%d)", m.Calls)
		}

		resp := m.responses[0]
		m.responses = m.responses[1:]
		return resp.data, resp.err
	}
}

// CallCount retourne le nombre d'appels effectués.
func (m *MockFetchFn) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Calls
}

// Reset vide la file de réponses et remet le compteur à zéro.
func (m *MockFetchFn) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = nil
	m.Calls = 0
}
