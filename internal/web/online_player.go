package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func blockedMirrorProvider(server string) bool {
	switch strings.ToLower(strings.TrimSpace(server)) {
	case "hls", "upnshare":
		return true
	default:
		return false
	}
}

func (s *Server) onlinePlayerAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	ep, _ := strconv.Atoi(r.URL.Query().Get("episode"))
	if slug == "" || ep < 1 {
		http.Error(w, "bad request", 400)
		return
	}
	players, err := s.fetchOnlinePlayers(r.Context(), slug, ep)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if len(players) == 0 {
		http.Error(w, "no se encontro un reproductor online compatible con el mirror", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(players)
}

func (s *Server) fetchOnlinePlayers(parent context.Context, slug string, ep int) ([]onlinePlayer, error) {
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
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []onlinePlayer{}
	for _, m := range embedRE.FindAllStringSubmatch(string(b), -1) {
		if len(m) < 3 || blockedMirrorProvider(m[1]) || seen[m[2]] {
			continue
		}
		if !strings.HasPrefix(m[2], "https://") {
			continue
		}
		seen[m[2]] = true
		out = append(out, onlinePlayer{Server: m[1], URL: m[2]})
	}
	return out, nil
}
