package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/superselle/ubertracker/tracker"
)

// ══════════════════════════════════════════════════════════════
// OrderBuilder — Builder fluent pour tracker.Order
// ══════════════════════════════════════════════════════════════

// OrderBuilder construit des tracker.Order avec des données réalistes.
type OrderBuilder struct {
	order tracker.Order
}

// NewTestOrder retourne un builder pré-rempli avec des valeurs par défaut.
func NewTestOrder() *OrderBuilder {
	progress := 2
	total := 5
	return &OrderBuilder{
		order: tracker.Order{
			ActiveOrderOverview: tracker.ActiveOrderOverview{
				Title: "Test Restaurant",
			},
			FeedCards: []tracker.FeedCard{
				{
					Status: &tracker.StatusInfo{
						CurrentProgress: progress,
						TotalProgress:   total,
						Title:           "En préparation",
					},
				},
			},
		},
	}
}

func (b *OrderBuilder) WithRestaurant(name string) *OrderBuilder {
	b.order.ActiveOrderOverview.Title = name
	return b
}

func (b *OrderBuilder) WithProgress(current, total int) *OrderBuilder {
	if len(b.order.FeedCards) > 0 && b.order.FeedCards[0].Status != nil {
		b.order.FeedCards[0].Status.CurrentProgress = current
		b.order.FeedCards[0].Status.TotalProgress = total
	}
	return b
}

func (b *OrderBuilder) WithStatusText(title string) *OrderBuilder {
	if len(b.order.FeedCards) > 0 && b.order.FeedCards[0].Status != nil {
		b.order.FeedCards[0].Status.Title = title
	}
	return b
}

func (b *OrderBuilder) WithETA(minutes int) *OrderBuilder {
	b.order.BackgroundFeedCards = []tracker.BackgroundFeedCard{
		{
			MapEntity: []tracker.MapEntity{
				{
					// ETA est extrait de BackgroundFeedCards dans le parser
				},
			},
		},
	}
	// Note: L'ETA réelle est extraite via ExtractETAFromOrder.
	// Ce builder met en place la structure minimale.
	return b
}

func (b *OrderBuilder) WithContact(name, phone string) *OrderBuilder {
	b.order.Contacts = []tracker.Contact{
		{Title: name},
	}
	return b
}

func (b *OrderBuilder) WithItems(items ...tracker.Item) *OrderBuilder {
	b.order.ActiveOrderOverview.Items = items
	return b
}

func (b *OrderBuilder) WithCallToAction(title string) *OrderBuilder {
	if len(b.order.FeedCards) > 0 {
		b.order.FeedCards[0].CallToAction = &tracker.CallToAction{
			Title: title,
		}
	}
	return b
}

func (b *OrderBuilder) WithPIN(pin string) *OrderBuilder {
	if len(b.order.FeedCards) > 0 {
		b.order.FeedCards[0].Courier = []tracker.CourierInfo{
			{
				PinInfo: tracker.PinInfo{
					Pin: pin,
				},
			},
		}
	}
	return b
}

// WithTotal sets the order total in OrderSummary.
func (b *OrderBuilder) WithTotal(total string) *OrderBuilder {
	if len(b.order.FeedCards) > 0 {
		b.order.FeedCards[0].OrderSummary.Total = total
	}
	return b
}

// WithAddress sets the delivery address.
func (b *OrderBuilder) WithAddress(addr string) *OrderBuilder {
	if len(b.order.FeedCards) > 0 {
		b.order.FeedCards[0].Delivery = &tracker.DeliveryInfo{
			Address: addr,
		}
	}
	return b
}

// Build retourne le tracker.Order construit.
func (b *OrderBuilder) Build() tracker.Order {
	return b.order
}

// WithPhase définit la phase de l'order (ex: "ACTIVE", "COMPLETED", "DELIVERED").
func (b *OrderBuilder) WithPhase(phase string) *OrderBuilder {
	b.order.OrderStatus.OrderPhase = phase
	return b
}

// BuildResponse wraps l'Order dans une Response complète.
func (b *OrderBuilder) BuildResponse() tracker.Response {
	return tracker.Response{
		Data: tracker.Data{
			Orders: []tracker.Order{b.order},
		},
	}
}

// ══════════════════════════════════════════════════════════════
// Helpers de chargement testdata/
// ══════════════════════════════════════════════════════════════

// testdataDir retourne le chemin absolu vers tracker/testdata/.
func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "tracker", "testdata")
}

// LoadTestJSON lit un fichier JSON depuis tracker/testdata/ et retourne le contenu brut.
func LoadTestJSON(t testing.TB, filename string) []byte {
	t.Helper()
	path := filepath.Join(testdataDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("LoadTestJSON(%s): %v", filename, err)
	}
	return data
}
