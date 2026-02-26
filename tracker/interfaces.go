package tracker

import "context"

// OrderStore abstrait la persistance. Le dépôt privé l'implémente avec SQLite.
type OrderStore interface {
	// GetSnapshot retourne le dernier état connu d'une commande.
	GetSnapshot(ctx context.Context, uuid string) (status string, progress int, text string, rawJSON string, err error)

	// SaveOrder persiste l'état complet d'une commande.
	SaveOrder(ctx context.Context, order TrackedOrder) error

	// GetMessageID retourne l'ID du message Discord associé (ou "").
	GetMessageID(ctx context.Context, uuid string) (string, error)

	// GetPendingOrders retourne les commandes non terminées (uuid → progress).
	GetPendingOrders(ctx context.Context) (map[string]int, error)

	// ListResumableOrders retourne les commandes à reprendre après redémarrage.
	ListResumableOrders(ctx context.Context) ([]ResumableOrder, error)
}

// ResumableOrder contient le minimum pour relancer un worker.
type ResumableOrder struct {
	UUID      string
	ChannelID string
	GuildID   string
	ClientID  string
	CuistotID string
}
