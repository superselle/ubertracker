package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"time"
)

// FetchFn est le type de la fonction d'appel API injectable (production : FetchUberJSON).
type FetchFn func(ctx context.Context, uuid string) ([]byte, error)

// SafeTruncate tronque une chaîne à maxLen caractères sans risque de panic.
func SafeTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ==========================================
// Sous-fonctions du worker
// ==========================================

// fetchAndParse appelle l'API Uber et désérialise la réponse.
// Retourne la Response complète ou une erreur.
func fetchAndParse(ctx context.Context, fetchFn FetchFn, uuid string) (Response, error) {
	jsonBytes, err := fetchFn(ctx, uuid)
	if err != nil {
		return Response{}, fmt.Errorf("API: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(jsonBytes, &resp); err != nil {
		return Response{}, fmt.Errorf("JSON: %w", err)
	}

	if len(resp.Data.Orders) == 0 {
		return Response{}, fmt.Errorf("aucune commande retournée")
	}

	return resp, nil
}

// ReconcileResult contient le résultat de la réconciliation entre ancien et nouvel état.
type ReconcileResult struct {
	FinalJSON  string
	Phase      string
	Progress   int
	Text       string
	Eta        int
	ShouldEmit bool
}

// Reconcile fusionne newOrder avec l'état précédent stocké via OrderStore.
// Retourne le résultat de réconciliation avec les indicateurs
// nécessaires pour décider si un update Discord est requis.
func Reconcile(ctx context.Context, store OrderStore, uuid string, resp Response) (ReconcileResult, error) {
	newOrder := resp.Data.Orders[0]
	slog.Debug("données reçues", "uuid", SafeTruncate(uuid, 8), "phase", newOrder.OrderStatus.OrderPhase)

	// Snapshot précédent
	lastStatus, lastProgress, lastText, lastJSONStr, err := store.GetSnapshot(ctx, uuid)

	var masterOrder Order
	hasOldData := false

	if err == nil && lastJSONStr != "" {
		var lastResp Response
		if json.Unmarshal([]byte(lastJSONStr), &lastResp) == nil && len(lastResp.Data.Orders) > 0 {
			masterOrder = lastResp.Data.Orders[0]
			hasOldData = true
		}
	}

	if !hasOldData {
		masterOrder = newOrder
	}

	// Fusion (déléguée au parser)
	masterOrder, newPhase := MergeOrderData(masterOrder, newOrder, hasOldData)

	// Toujours prendre les backgroundFeedCards les plus récents (ETA temps réel)
	if len(newOrder.BackgroundFeedCards) > 0 {
		masterOrder.BackgroundFeedCards = newOrder.BackgroundFeedCards
	}

	resp.Data.Orders[0] = masterOrder

	finalJSON, err := json.Marshal(resp)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("sérialisation JSON finale: %w", err)
	}

	var newProgress int
	var newText string
	if len(masterOrder.FeedCards) > 0 && masterOrder.FeedCards[0].Status != nil {
		newProgress = masterOrder.FeedCards[0].Status.CurrentProgress
		newText = masterOrder.FeedCards[0].Status.StatusSummary.Text
	}

	newETA := ExtractETAFromOrder(masterOrder)

	shouldEmit := !hasOldData ||
		newPhase != lastStatus ||
		newProgress != lastProgress ||
		newText != lastText ||
		newETA >= 0 // Toujours pousser si on a un ETA (il change souvent)

	return ReconcileResult{
		FinalJSON:  string(finalJSON),
		Phase:      newPhase,
		Progress:   newProgress,
		Text:       newText,
		Eta:        newETA,
		ShouldEmit: shouldEmit,
	}, nil
}

// emitUpdate persiste l'état et envoie la mise à jour sur le channel.
func emitUpdate(ctx context.Context, store OrderStore, id OrderIdentity, r ReconcileResult, updates chan<- TrackedOrder) error {
	existingMsgID, _ := store.GetMessageID(ctx, id.UUID)

	tracked := TrackedOrder{
		UUID:         id.UUID,
		ChannelID:    id.ChannelID,
		GuildID:      id.GuildID,
		LastStatus:   r.Phase,
		LastUpdated:  time.Now(),
		FullJSONData: r.FinalJSON,
		ClientID:     id.ClientID,
		CuistotID:    id.CuistotID,
		LastProgress: r.Progress,
		LastText:     r.Text,
		MessageID:    existingMsgID,
		ETAMinutes:   r.Eta,
	}

	if err := store.SaveOrder(ctx, tracked); err != nil {
		return fmt.Errorf("SaveOrder: %w", err)
	}

	slog.Debug("update BDD réussi, envoi au consumer", "uuid", SafeTruncate(id.UUID, 8))
	select {
	case updates <- tracked:
	case <-ctx.Done():
	}
	return nil
}

// ==========================================
// Boucle principale du worker
// ==========================================

// StartOrderWorker lance le processus de surveillance pour UNE commande.
// Cette fonction est bloquante — elle doit être appelée dans une goroutine.
// Elle communique ses résultats via le channel updates, sans aucune dépendance
// à Discord ou à une base de données concrète (tout passe par l'interface OrderStore).
// fetchFn est injectable pour les tests (production : FetchUberJSON).
func StartOrderWorker(
	ctx context.Context,
	store OrderStore,
	id OrderIdentity,
	updates chan<- TrackedOrder,
	fetchFn FetchFn,
) {
	slog.Info("worker démarré", "uuid", id.UUID)

	failCount := 0
	const maxFails = 10

	// scan effectue un cycle fetch → reconcile → emit.
	// Retourne true si le worker doit s'arrêter (commande terminée ou échecs max).
	scan := func() bool {
		resp, err := fetchAndParse(ctx, fetchFn, id.UUID)
		if err != nil {
			slog.Error("erreur worker", "uuid", SafeTruncate(id.UUID, 8), "error", err)
			failCount++
			if failCount >= maxFails {
				slog.Error("arrêt définitif worker", "uuid", SafeTruncate(id.UUID, 8), "max_fails", maxFails)
				select {
				case updates <- TrackedOrder{
					UUID:       id.UUID,
					ChannelID:  id.ChannelID,
					GuildID:    id.GuildID,
					LastStatus: "FAILED",
					LastText:   "Suivi abandonné après trop d'échecs.",
				}:
				case <-ctx.Done():
				}
				return true
			}
			return false
		}

		failCount = 0

		result, err := Reconcile(ctx, store, id.UUID, resp)
		if err != nil {
			slog.Error("erreur reconcile", "uuid", SafeTruncate(id.UUID, 8), "error", err)
			return false
		}

		if result.ShouldEmit {
			if err := emitUpdate(ctx, store, id, result, updates); err != nil {
				slog.Error("erreur emitUpdate", "uuid", SafeTruncate(id.UUID, 8), "error", err)
			}
		}

		return result.Phase == "COMPLETED" || result.Phase == "DELIVERED" || result.Phase == "CANCELLED"
	}

	// Scan initial
	if scan() {
		return
	}

	// Boucle de polling avec intervalle adaptatif et support d'annulation via context
	noChangeCount := 0
	lastKnownETA := -1
	for {
		interval := AdaptiveInterval(lastKnownETA, noChangeCount)
		jitter := rand.Intn(21) //nolint:gosec // jitter for polling, not security-sensitive
		sleepTime := time.Duration(interval+jitter) * time.Second

		select {
		case <-ctx.Done():
			slog.Info("worker arrêté par contexte", "uuid", SafeTruncate(id.UUID, 8))
			return
		case <-time.After(sleepTime):
		}

		prevFails := failCount
		if scan() {
			return
		}

		// Mise à jour de l'ETA connu pour le prochain cycle de polling
		if _, _, _, lastJSON, err := store.GetSnapshot(ctx, id.UUID); err == nil {
			var r Response
			if json.Unmarshal([]byte(lastJSON), &r) == nil && len(r.Data.Orders) > 0 {
				if eta := ExtractETAFromOrder(r.Data.Orders[0]); eta >= 0 {
					lastKnownETA = eta
				}
			}
		}

		if failCount == prevFails {
			noChangeCount++
		} else {
			noChangeCount = 0
		}
	}
}

// AdaptiveInterval calcule l'intervalle de polling en secondes selon l'ETA
// et le nombre de cycles sans changement.
//   - ETA ≤ 5 min  → 15 s (notifications minute par minute)
//   - ETA ≤ 15 min → 25 s
//   - Sinon        → 30 s + backoff progressif (max 120 s)
func AdaptiveInterval(eta, noChangeCount int) int {
	if eta >= 0 && eta <= 5 {
		return 15
	}
	if eta > 5 && eta <= 15 {
		return 25
	}
	base := 30 + (noChangeCount * 10)
	if base > 120 {
		return 120
	}
	return base
}
