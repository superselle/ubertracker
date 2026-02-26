package tracker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// GetFreshCookies lance un navigateur Chrome, navigue vers l'URL et récupère les cookies.
// Cette fonction est lente (~5-10s) et ne doit être utilisée qu'en cas d'erreur 403/401.
func GetFreshCookies(orderURL string) (string, error) {
	slog.Info("lancement navigateur de secours pour rafraîchir les cookies")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(defaultUserAgent),
		chromedp.Flag("headless", true), // Mettre à 'false' pour voir le navigateur (debug)
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.WindowSize(1280, 800),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// Création du contexte (avec un timeout de 30s pour ne pas bloquer le bot)
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 40*time.Second)
	defer cancel()

	// Variable pour stocker les cookies bruts
	var cookies []*network.Cookie

	// Scénario de navigation
	err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate(orderURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		// Laisse le temps aux scripts anti-bot d'Uber de poser leurs cookies.
		chromedp.Sleep(time.Duration(browserSettleDelay)*time.Second),

		// Extraction des cookies
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)

	if err != nil {
		return "", fmt.Errorf("erreur chromedp: %w", err)
	}

	// Formatage des cookies pour tls-client : "nom=valeur; nom2=valeur2"
	cookieParts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	cookieString := strings.Join(cookieParts, "; ")

	if len(cookieParts) == 0 {
		return "", fmt.Errorf("aucun cookie récupéré, Uber a peut-être bloqué l'accès")
	}

	slog.Info("cookies récupérés avec succès", "count", len(cookies))
	return cookieString, nil
}
