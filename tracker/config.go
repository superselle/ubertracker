package tracker

// Constantes partagées par api.go et browser.go.

// defaultUserAgent est le User-Agent envoyé à la fois par le client TLS et
// par le navigateur headless.  Garder une valeur unique évite les
// incohérences qui peuvent déclencher les protections anti-bot d'Uber.
const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// browserSettleDelay est le temps laissé aux scripts anti-bot d'Uber pour
// s'exécuter après le chargement de la page, avant de récupérer les cookies.
const browserSettleDelay = 8 // secondes
