package tracker

import (
	"strconv"
	"strings"
)

const phaseCompleted = "COMPLETED"

// ExtractETAFromOrder cherche l'entité de type "LABEL" dans les BackgroundFeedCards
// et retourne l'ETA (title) en minutes. Retourne -1 si introuvable.
func ExtractETAFromOrder(order Order) int {
	for _, card := range order.BackgroundFeedCards {
		for _, entity := range card.MapEntity {
			if entity.Type == "LABEL" {
				if parsed, err := strconv.Atoi(strings.TrimSpace(entity.Title)); err == nil {
					return parsed
				}
			}
		}
	}
	return -1
}

// ==========================================
// Données préservées entre deux polls
// ==========================================

// preservedInfo contient les informations qui disparaissent de l'API au fil
// du temps (total, adresse, PIN) et doivent être conservées entre les polls.
type preservedInfo struct {
	total   string
	address string
	pin     string
}

// extractPreservedInfo parcourt les FeedCards d'un Order et extrait les
// informations susceptibles de disparaître de l'API plus tard.
func extractPreservedInfo(o Order) preservedInfo {
	var p preservedInfo
	for _, card := range o.FeedCards {
		if card.OrderSummary.Total != "" {
			p.total = card.OrderSummary.Total
		}
		if card.Delivery != nil && card.Delivery.Address != "" {
			p.address = card.Delivery.Address
		}
		for _, c := range card.Courier {
			if c.PinInfo.Pin != "" {
				p.pin = c.PinInfo.Pin
			}
		}
	}
	return p
}

// restorePreservedInfo réinjecte les données perdues dans le premier FeedCard
// si elles sont absentes de l'Order actuel.
func restorePreservedInfo(o *Order, p preservedInfo) {
	if len(o.FeedCards) == 0 {
		return
	}
	fc := &o.FeedCards[0]

	if fc.OrderSummary.Total == "" && p.total != "" {
		fc.OrderSummary.Total = p.total
	}
	if p.address != "" {
		if fc.Delivery == nil {
			fc.Delivery = &DeliveryInfo{}
		}
		if fc.Delivery.Address == "" {
			fc.Delivery.Address = p.address
		}
	}
	if p.pin != "" {
		if len(fc.Courier) == 0 {
			fc.Courier = []CourierInfo{{}}
		}
		if fc.Courier[0].PinInfo.Pin == "" {
			fc.Courier[0].PinInfo.Pin = p.pin
		}
	}
}

// detectPhase retourne la phase effective de la commande, en corrigeant le
// cas d'une annulation déguisée (Uber renvoie COMPLETED + callToAction "annulée").
func detectPhase(newOrder Order) string {
	phase := newOrder.OrderStatus.OrderPhase
	if phase == phaseCompleted {
		for _, card := range newOrder.FeedCards {
			if card.CallToAction != nil && strings.Contains(strings.ToLower(card.CallToAction.Title), "annul") {
				return "CANCELLED"
			}
		}
	}
	return phase
}

// ==========================================
// Fusion publique
// ==========================================

// MergeOrderData fusionne les données anciennes (masterOrder) et nouvelles (newOrder).
// Retourne l'Order fusionné et la phase détectée (avec détection d'annulation déguisée).
func MergeOrderData(masterOrder, newOrder Order, hasOldData bool) (Order, string) {
	// 1. Préservation des données précieuses
	var kept preservedInfo
	if hasOldData {
		kept = extractPreservedInfo(masterOrder)
	}
	// Les données les plus récentes priment
	fresh := extractPreservedInfo(newOrder)
	if fresh.total != "" {
		kept.total = fresh.total
	}
	if fresh.address != "" {
		kept.address = fresh.address
	}
	if fresh.pin != "" {
		kept.pin = fresh.pin
	}

	// 2. Fusion des champs top-level
	if newOrder.ActiveOrderOverview.Title != "" {
		masterOrder.ActiveOrderOverview = newOrder.ActiveOrderOverview
	}
	if len(newOrder.Contacts) > 0 {
		masterOrder.Contacts = newOrder.Contacts
	}

	// 3. Détection de la phase réelle
	newPhase := detectPhase(newOrder)

	// 4. Mise à jour des feedCards
	if newPhase == phaseCompleted {
		for i, card := range masterOrder.FeedCards {
			if card.Status != nil {
				masterOrder.FeedCards[i].Status.Title = "Commande Livrée"
				masterOrder.FeedCards[i].Status.TitleSummary.Summary.Text = "Bon appétit ! La commande a été livrée."
				masterOrder.FeedCards[i].Status.StatusSummary.Text = "Livraison terminée"
				masterOrder.FeedCards[i].Status.CurrentProgress = 5
				masterOrder.FeedCards[i].Status.TotalProgress = 5
				break
			}
		}
	} else {
		if len(newOrder.FeedCards) > 0 && newOrder.FeedCards[0].Status != nil {
			masterOrder.FeedCards = newOrder.FeedCards
		}
	}
	masterOrder.OrderStatus.OrderPhase = newPhase

	// 5. Restauration des données perdues
	restorePreservedInfo(&masterOrder, kept)

	return masterOrder, newPhase
}
