package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

func shouldProxyAnimeAV1(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	switch r.URL.Path {
	case "/api/search", "/api/user/library", "/api/user/library/favorite":
		return true
	case "/cuenta/listas":
		return strings.Contains(r.URL.RawQuery, "/library")
	default:
		return false
	}
}

func (s *Server) proxyAnimeAV1(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	target := "https://animeav1.com" + r.URL.RequestURI()
	req, err := http.NewRequestWithContext(ctx, r.Method, target, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/142 Safari/537.36")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	for _, h := range []string{"Accept", "Content-Type", "X-SvelteKit-Action"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	req.Header.Set("Origin", "https://animeav1.com")
	req.Header.Set("Referer", "https://animeav1.com"+r.URL.Path)
	if cookie := strings.TrimSpace(s.db.GetSetting("animeav1_session_cookie")); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	if r.Method != http.MethodHead {
		_, _ = w.Write(respBody)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && r.URL.Path != "/api/search" {
		go s.refreshAnimeAV1Cache()
	}
}

func (s *Server) refreshAnimeAV1Cache() {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	items, err := s.av1.Library(ctx, s.db.GetSetting("animeav1_session_cookie"))
	if err != nil {
		_ = s.db.SetSetting("animeav1_library_error", err.Error())
		return
	}
	b, err := json.Marshal(items)
	if err != nil {
		return
	}
	_ = s.db.SetSetting("animeav1_library_json", string(b))
	_ = s.db.SetSetting("animeav1_library_updated", time.Now().Format(time.RFC3339))
	_ = s.db.SetSetting("animeav1_library_error", "")
	s.crawl.RefreshConfigState()
}
