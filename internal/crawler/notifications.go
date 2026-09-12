package crawler

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
)

// RunNotificationCheck revisa únicamente las fichas de las series en estado Viendo.
// Es deliberadamente ligero: una petición por ficha y actualización del índice de episodios,
// sin analizar reproductores ni descargar contenido.
func (s *Service) RunNotificationCheck(ctx context.Context) error {
	raw := strings.TrimSpace(s.db.GetSetting("animeav1_library_json"))
	if raw == "" {
		return nil
	}
	var items []animeav1.Item
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return err
	}
	checked := 0
	for _, item := range items {
		if item.Status != 0 || strings.TrimSpace(item.Slug) == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := s.RunTarget(ctx, item.Slug); err != nil {
			log.Printf("[NOTIFY] %s: %v", item.Slug, err)
			continue
		}
		checked++
	}
	log.Printf("[NOTIFY] %d series en Viendo revisadas", checked)
	return nil
}
