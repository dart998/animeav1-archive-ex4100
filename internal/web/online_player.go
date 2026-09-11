package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type onlinePlayer struct {
	Server string `json:"server"`
	URL    string `json:"url"`
}

var embedRE = regexp.MustCompile(`server:"([^"]+)",url:"([^"]+)"`)

func isStreamingSource(server, raw string) bool {
	name := strings.ToLower(strings.TrimSpace(server))
	if name == "hls" || name == "upnshare" || name == "transferit" || name == "transfer.it" || strings.Contains(name, "1fichier") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(u.Path)
	if strings.Contains(host, "transfer.it") || strings.Contains(host, "1fichier.com") {
		return false
	}
	// MEGA publica tanto reproductores como enlaces de descarga. Solo /embed/ es stream.
	if strings.Contains(host, "mega.nz") || strings.Contains(host, "mega.co.nz") {
		return strings.Contains(path, "/embed/")
	}
	return true
}

func (s *Server) onlinePlayerAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	ep, _ := strconv.Atoi(r.URL.Query().Get("episode"))
	if slug == "" || ep < 1 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	players, err := s.fetchOnlinePlayers(r.Context(), slug, ep)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(players) == 0 {
		http.Error(w, "no se encontro un reproductor online compatible con el mirror", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(players)
}

func (s *Server) fetchEpisodeHTML(parent context.Context, slug string, ep int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	u := fmt.Sprintf("https://animeav1.com/media/%s/%d", slug, ep)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/142 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	if cookie := strings.TrimSpace(s.db.GetSetting("animeav1_session_cookie")); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("AnimeAV1 episodio: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func (s *Server) fetchOnlinePlayers(parent context.Context, slug string, ep int) ([]onlinePlayer, error) {
	b, err := s.fetchEpisodeHTML(parent, slug, ep)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []onlinePlayer{}
	for _, m := range embedRE.FindAllStringSubmatch(string(b), -1) {
		if len(m) < 3 || seen[m[2]] || !isStreamingSource(m[1], m[2]) {
			continue
		}
		seen[m[2]] = true
		out = append(out, onlinePlayer{Server: m[1], URL: m[2]})
	}
	return out, nil
}
