package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// GlobalCookies et CookieMutex stockent les cookies partagés entre tous les workers.
var (
	GlobalCookies string
	CookieMutex   sync.RWMutex

	// cookieRefreshGroup déduplique les appels concurrents de renouvellement de cookies.
	cookieRefreshGroup singleflight.Group

	// Client TLS réutilisable (un seul client pour toutes les requêtes).
	sharedTLSClient     tls_client.HttpClient
	sharedTLSClientOnce sync.Once
	sharedTLSClientErr  error
)

// TrackingPayload : La structure requise par l'API Uber
type TrackingPayload struct {
	OrderUUID                 string `json:"orderUuid"`
	Timezone                  string `json:"timezone"`
	ShowAppUpsellIllustration bool   `json:"showAppUpsellIllustration"`
	IsDirectTracking          bool   `json:"isDirectTracking"`
}

// FetchUberJSON tente de récupérer le JSON de la commande.
func FetchUberJSON(ctx context.Context, orderUUID string) ([]byte, error) {
	CookieMutex.RLock()
	currentCookies := GlobalCookies
	CookieMutex.RUnlock()

	jsonBytes, statusCode, err := performRequest(ctx, orderUUID, currentCookies)
	if err != nil {
		return nil, err
	}

	if statusCode == 401 || statusCode == 403 {
		publicURL := fmt.Sprintf("https://www.ubereats.com/orders/%s", orderUUID)

		result, err, _ := cookieRefreshGroup.Do("refresh", func() (interface{}, error) {
			return GetFreshCookies(publicURL)
		})
		if err != nil {
			return nil, fmt.Errorf("échec du renouvellement des cookies: %w", err)
		}

		CookieMutex.Lock()
		if val, ok := result.(string); ok {
			GlobalCookies = val
		}
		CookieMutex.Unlock()

		slog.Info("cookies mis à jour, nouvelle tentative")

		jsonBytes, statusCode, err = performRequest(ctx, orderUUID, GlobalCookies)
		if err != nil {
			return nil, err
		}
		if statusCode != 200 {
			return nil, fmt.Errorf("erreur fatale après renouvellement (Code %d)", statusCode)
		}
	} else if statusCode != 200 {
		return nil, fmt.Errorf("erreur API HTTP: %d", statusCode)
	}

	return jsonBytes, nil
}

// getSharedTLSClient retourne le client TLS partagé, initialisé une seule fois.
// En cas d'erreur lors de la création, l'erreur est mémorisée et retournée à
// chaque appel suivant (pas de retry implicite via sync.Once).
func getSharedTLSClient() (tls_client.HttpClient, error) {
	sharedTLSClientOnce.Do(func() {
		options := []tls_client.HttpClientOption{
			tls_client.WithClientProfile(profiles.Chrome_120),
			tls_client.WithTimeoutSeconds(30),
			tls_client.WithRandomTLSExtensionOrder(),
		}
		c, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
		if err != nil {
			sharedTLSClientErr = fmt.Errorf("impossible de créer le client TLS: %w", err)
			return
		}
		sharedTLSClient = c
	})
	return sharedTLSClient, sharedTLSClientErr
}

// performRequest exécute la requête HTTP pure vers l'API Uber.
func performRequest(ctx context.Context, uuid string, cookies string) ([]byte, int, error) {
	client, err := getSharedTLSClient()
	if err != nil {
		return nil, 0, err
	}

	payload := TrackingPayload{
		OrderUUID:                 uuid,
		Timezone:                  "Europe/Paris",
		ShowAppUpsellIllustration: true,
		IsDirectTracking:          false,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("échec sérialisation payload: %w", err)
	}
	payloadReader := strings.NewReader(string(bodyBytes))

	apiURL := "https://www.ubereats.com/_p/api/getActiveOrdersV1?localeCode=fr"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, payloadReader)
	if err != nil {
		return nil, 0, err
	}

	req.Header = http.Header{
		"authority":       {"www.ubereats.com"},
		"accept":          {"*/*"},
		"accept-language": {"fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7"},
		"content-type":    {"application/json"},
		"origin":          {"https://www.ubereats.com"},
		"x-csrf-token":    {"x"},
		"Cookie":          {cookies},
		"user-agent":      {defaultUserAgent},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}
