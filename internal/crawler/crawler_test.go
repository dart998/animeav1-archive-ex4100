package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	"github.com/dart998/animeav1-archive-ex4100/internal/config"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

func testService(t *testing.T, handler http.Handler) (*Service, *database.DB) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	root := t.TempDir()
	db, err := database.Open(filepath.Join(root, "archive.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cfg := config.Config{BaseURL: server.URL, MetadataDir: filepath.Join(root, "metadata"), VideoDir: filepath.Join(root, "videos"), ProviderOrder: []string{"hls"}, ProviderFallback: true}
	return New(cfg, db), db
}

func TestRunAllUsesAnimeAV1Scope(t *testing.T) {
	hits := map[string]int{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		switch r.URL.Path {
		case "/media/watching":
			_, _ = w.Write([]byte(`<h1>Watching</h1><a href="/media/watching/1">1</a>`))
		case "/media/watching/1":
			_, _ = w.Write([]byte(`<h1>Episode</h1>`))
		case "/media/completed":
			_, _ = w.Write([]byte(`<h1>Completed</h1>`))
		default:
			http.NotFound(w, r)
		}
	})
	s, db := testService(t, h)
	items := []animeav1.Item{{Title: "Watching", Slug: "watching", Status: 0}, {Title: "Completed", Slug: "completed", Status: 2}, {Title: "Paused", Slug: "paused", Status: 1}}
	b, _ := json.Marshal(items)
	if err := db.SetSetting("animeav1_library_json", string(b)); err != nil {
		t.Fatal(err)
	}
	if err := s.RunAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits["/media/watching"] != 1 || hits["/media/completed"] != 1 {
		t.Fatalf("series en alcance no rastreadas: %#v", hits)
	}
	if hits["/media/paused"] != 0 {
		t.Fatalf("una serie fuera de alcance fue rastreada: %#v", hits)
	}
	if hits["/media/watching/1"] != 0 {
		t.Fatalf("el descubrimiento no debe abrir páginas de episodio: %#v", hits)
	}
	if got := db.SeriesEpisodeCount("watching"); got != 1 {
		t.Fatalf("episodios guardados=%d, esperado=1", got)
	}
	state := s.State.Snapshot()
	if state.AV1Series != 2 || state.AV1Crawled != 2 || state.LastStatus != "animeav1_crawled" {
		t.Fatalf("estado inesperado: status=%s series=%d crawled=%d error=%s", state.LastStatus, state.AV1Series, state.AV1Crawled, state.LastError)
	}
}

func TestRunAllAdvancesInSmallBatches(t *testing.T) {
	hits := map[string]int{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		_, _ = w.Write([]byte(`<h1>Serie</h1>`))
	})
	s, db := testService(t, h)
	s.cfg.CrawlerBatchSize = 2
	items := []animeav1.Item{{Title: "A", Slug: "a", Status: 0}, {Title: "B", Slug: "b", Status: 0}, {Title: "C", Slug: "c", Status: 0}, {Title: "D", Slug: "d", Status: 2}, {Title: "Fuera", Slug: "fuera", Status: 1}}
	b, _ := json.Marshal(items)
	if err := db.SetSetting("animeav1_library_json", string(b)); err != nil {
		t.Fatal(err)
	}
	if err := s.RunAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits["/media/a"] != 1 || hits["/media/b"] != 1 || hits["/media/c"] != 0 {
		t.Fatalf("primer lote inesperado: %#v", hits)
	}
	if got := db.GetSetting("crawler_scope_cursor"); got != "2" {
		t.Fatalf("cursor=%s, esperado=2", got)
	}
	if err := s.RunAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits["/media/c"] != 1 || hits["/media/d"] != 1 || hits["/media/fuera"] != 0 {
		t.Fatalf("segundo lote inesperado: %#v", hits)
	}
	if got := db.GetSetting("crawler_scope_cursor"); got != "0" {
		t.Fatalf("cursor=%s, esperado=0", got)
	}
}

func TestRunAllWaitsForAnimeAV1Lists(t *testing.T) {
	s, _ := testService(t, http.NotFoundHandler())
	if err := s.RunAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	state := s.State.Snapshot()
	if state.LastStatus != "waiting_for_animeav1" {
		t.Fatalf("estado inesperado: status=%s error=%s", state.LastStatus, state.LastError)
	}
}

func TestRunAllRejectsInvalidAnimeAV1Data(t *testing.T) {
	s, db := testService(t, http.NotFoundHandler())
	if err := db.SetSetting("animeav1_library_json", "{"); err != nil {
		t.Fatal(err)
	}
	if err := s.RunAll(context.Background()); err == nil {
		t.Fatal("se esperaba un error")
	}
	if state := s.State.Snapshot(); state.LastStatus != "animeav1_scope_error" {
		t.Fatalf("estado inesperado: status=%s error=%s", state.LastStatus, state.LastError)
	}
}
