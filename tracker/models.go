package tracker

import "time"

// ==========================================
// MODÈLE DE SUIVI (état d'une commande trackée)
// ==========================================

// OrderIdentity regroupe les identifiants immuables d'une commande.
// Ce struct remplace les 5 paramètres string passés individuellement à
// StartOrderWorker et StartTracking.
type OrderIdentity struct {
	UUID      string
	ChannelID string
	GuildID   string
	ClientID  string
	CuistotID string
}

// TrackedOrder représente l'état complet d'une commande suivie.
// Ce type est utilisé sur le channel de communication entre le tracker et le consommateur.
type TrackedOrder struct {
	UUID         string
	GuildID      string
	ChannelID    string
	LastStatus   string
	LastUpdated  time.Time
	FullJSONData string
	ClientID     string
	CuistotID    string
	LastProgress int
	LastText     string
	MessageID    string
	ETAMinutes   int // Temps restant en minutes (extrait de backgroundFeedCards), -1 = inconnu
}

// ==========================================
// MODÈLES API UBER (JSON PARSING)
// ==========================================

type Response struct {
	Data Data `json:"data"`
}

type Data struct {
	Orders []Order `json:"orders"`
}

type Order struct {
	Contacts            []Contact            `json:"contacts"`
	ActiveOrderOverview ActiveOrderOverview  `json:"activeOrderOverview"`
	FeedCards           []FeedCard           `json:"feedCards"`
	BackgroundFeedCards []BackgroundFeedCard `json:"backgroundFeedCards"`
	OrderStatus         OrderInfo            `json:"orderInfo"`
}

// BackgroundFeedCard contient les données cartographiques (ETA, position du livreur)
type BackgroundFeedCard struct {
	MapEntity []MapEntity `json:"mapEntity"`
	Type      string      `json:"type,omitempty"`
}

// MapEntity représente un point sur la carte (livreur, restaurant, client)
// Le champ Title contient le nombre de minutes restantes (ETA)
type MapEntity struct {
	UUID      string   `json:"uuid"`
	Type      string   `json:"type"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Title     string   `json:"title"`
	Subtitle  []string `json:"subtitle"`
}

type OrderInfo struct {
	OrderPhase string `json:"orderPhase"`
}

type Contact struct {
	Title                string `json:"title"`
	FormattedPhoneNumber string `json:"formattedPhoneNumber"`
}

type ActiveOrderOverview struct {
	Title    string `json:"title"`
	Items    []Item `json:"items"`
	Subtitle string `json:"subtitle"`
}

type Item struct {
	Title    string `json:"title"`
	Quantity int    `json:"quantity"`
	Subtitle string `json:"subtitle"`
}

// FeedCard utilise des pointeurs pour gérer la diversité des objets dans le tableau
type FeedCard struct {
	Type         string        `json:"type,omitempty"`
	Status       *StatusInfo   `json:"status,omitempty"`
	CallToAction *CallToAction `json:"callToAction,omitempty"`
	Courier      []CourierInfo `json:"courier,omitempty"`
	Delivery     *DeliveryInfo `json:"delivery,omitempty"`

	// Structure anonyme pour le total
	OrderSummary struct {
		Total string `json:"total"`
	} `json:"orderSummary,omitempty"`
}

// CallToAction représente un message final (annulation, erreur, etc.)
type CallToAction struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

type StatusInfo struct {
	Title           string         `json:"title"`
	Subtitle        string         `json:"subtitle"`
	StatusSummary   SummaryText    `json:"statusSummary"`
	TimelineSummary string         `json:"timelineSummary"`
	CurrentProgress int            `json:"currentProgress"`
	TotalProgress   int            `json:"totalProgressSegments"`
	TitleSummary    SummaryWrapper `json:"titleSummary"`
}

type SummaryText struct {
	Text     string `json:"text"`
	InfoText string `json:"infoText"`
	InfoBody string `json:"infoBody"`
}

type SummaryWrapper struct {
	Summary SummaryContent `json:"summary"`
}

type SummaryContent struct {
	Text string `json:"text"`
}

type CourierInfo struct {
	PinInfo PinInfo `json:"pinVerificationInfo"`
}

type PinInfo struct {
	Pin string `json:"pin"`
}

type DeliveryInfo struct {
	Address string `json:"formattedAddress"`
}
