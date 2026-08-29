package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type health struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Time    string `json:"time"`
}

var version = "dev"

func main() {
	dataDir := env("ARCHIVE_DATA_DIR", "/data")
	for _, d := range []string{"db", "metadata", "images", "videos", "site", "logs", "tmp"} {
		if err := os.MkdirAll(filepath.Join(dataDir, d), 0o755); err != nil {
			log.Fatalf("create data directory %s: %v", d, err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(health{Status: "ok", Version: version, Time: time.Now().Format(time.RFC3339)})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>AnimeAV1 Archive</title><style>body{font-family:system-ui,sans-serif;background:#111827;color:#e5e7eb;margin:0;padding:40px}.card{max-width:760px;margin:auto;background:#1f2937;border-radius:16px;padding:28px;box-shadow:0 10px 30px #0005}h1{margin-top:0}.ok{color:#34d399}code{background:#111827;padding:2px 6px;border-radius:6px}</style></head><body><div class="card"><h1>AnimeAV1 Archive</h1><p class="ok">● Servicio activo</p><p>Versión: <code>%s</code></p><p>Esta es la infraestructura inicial del archivador. El crawler y el catálogo se añadirán en la siguiente iteración.</p><p>Healthcheck: <code>/healthz</code></p></div></body></html>`, version)
	})

	addr := ":" + env("PORT", "8080")
	log.Printf("animeav1-archive %s listening on %s", version, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
