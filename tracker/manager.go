package tracker

import (
	"context"
	"log/slog"
	"sync"
)

// Manager est le chef d'orchestre du suivi des commandes.
// Il gère le cycle de vie des workers et communique via UpdateChannel.
type Manager struct {
	store         OrderStore
	fetchFn       FetchFn
	activeOrders  map[string]context.CancelFunc
	mutex         sync.Mutex
	UpdateChannel chan TrackedOrder
	wg            sync.WaitGroup
}

// NewManager crée une nouvelle instance avec le store injecté.
// fetchFn est optionnel : si omis, FetchUberJSON est utilisé par défaut.
func NewManager(store OrderStore, fetchFn ...FetchFn) *Manager {
	fn := FetchFn(FetchUberJSON)
	if len(fetchFn) > 0 && fetchFn[0] != nil {
		fn = fetchFn[0]
	}
	return &Manager{
		store:         store,
		fetchFn:       fn,
		activeOrders:  make(map[string]context.CancelFunc),
		UpdateChannel: make(chan TrackedOrder, 500),
	}
}

// StartTracking lance le suivi d'une commande. Retourne false si déjà en cours.
func (m *Manager) StartTracking(id OrderIdentity) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.activeOrders[id.UUID]; exists {
		return false
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.activeOrders[id.UUID] = cancel

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			m.mutex.Lock()
			delete(m.activeOrders, id.UUID)
			m.mutex.Unlock()
			cancel()
		}()
		StartOrderWorker(ctx, m.store, id, m.UpdateChannel, m.fetchFn)
	}()

	return true
}

// StopTracking arrête le suivi d'une commande spécifique.
func (m *Manager) StopTracking(uuid string) {
	m.mutex.Lock()
	cancel, exists := m.activeOrders[uuid]
	if exists {
		cancel()
		delete(m.activeOrders, uuid)
	}
	m.mutex.Unlock()
	slog.Info("suivi arrêté", "uuid", uuid)
}

// ResumeActiveOrders relance le tracking pour toutes les commandes non terminées.
func (m *Manager) ResumeActiveOrders() {
	slog.Info("vérification des commandes interrompues")

	orders, err := m.store.ListResumableOrders(context.Background())
	if err != nil {
		slog.Error("erreur récupération commandes à reprendre", "error", err)
		return
	}

	count := 0
	for _, o := range orders {
		id := OrderIdentity(o)
		if m.StartTracking(id) {
			slog.Info("suivi repris", "uuid", o.UUID)
			count++
		}
	}
	slog.Info("suivis repris", "count", count)
}

// Shutdown arrête proprement tous les workers actifs et ferme le channel.
func (m *Manager) Shutdown() {
	m.mutex.Lock()
	for uuid, cancel := range m.activeOrders {
		cancel()
		delete(m.activeOrders, uuid)
	}
	m.mutex.Unlock()
	m.wg.Wait()
	close(m.UpdateChannel)
}
