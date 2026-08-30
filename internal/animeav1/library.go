package animeav1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type IDString string

func (id *IDString) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" { *id = ""; return nil }
	if strings.HasPrefix(raw, "\"") {
		var s string
		if err := json.Unmarshal(data, &s); err != nil { return err }
		*id = IDString(s)
		return nil
	}
	*id = IDString(raw)
	return nil
}

type Item struct {
	MediaID  IDString          `json:"media_id"`
	Title    string            `json:"title"`
	Aliases  map[string]string `json:"aliases"`
	Seen     int               `json:"seen"`
	Total    int               `json:"total"`
	Status   int               `json:"status"`
	Score    int               `json:"score"`
	Favorite bool              `json:"favorite"`
	Slug     string            `json:"slug"`
}

func (i Item) StatusName() string {
	switch i.Status {
	case 0: return "Viendo"
	case 2: return "Completado"
	default: return fmt.Sprintf("Estado %d", i.Status)
	}
}

func InScope(items []Item) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.Status == 0 || it.Status == 2 { out = append(out, it) }
	}
	return out
}

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) Library(ctx context.Context, cookie string) ([]Item, error) {
	if strings.TrimSpace(cookie) == "" { return nil, errors.New("cookie de AnimeAV1 no configurada") }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/cuenta/listas", nil)
	if err != nil { return nil, err }
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux armv7l) AppleWebKit/537.36 Chrome/124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	req.Header.Set("Cookie", cookie)
	resp, err := c.http.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 { return nil, fmt.Errorf("AnimeAV1 respondió HTTP %d", resp.StatusCode) }
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil { return nil, err }
	return parseLibraryEntries(string(b))
}

func parseLibraryEntries(body string) ([]Item, error) {
	i := strings.Index(body, "libraryEntries:")
	if i < 0 { return nil, errors.New("AnimeAV1 no contiene libraryEntries; cookie caducada o formato cambiado") }
	startRel := strings.Index(body[i:], "[")
	if startRel < 0 { return nil, errors.New("libraryEntries sin array") }
	start := i + startRel
	arr, err := balanced(body, start, '[', ']')
	if err != nil { return nil, err }
	var items []Item
	if err := json.Unmarshal([]byte(arr), &items); err != nil { return nil, fmt.Errorf("libraryEntries JSON: %w", err) }
	filtered := items[:0]
	for _, it := range items { if strings.TrimSpace(it.Title) != "" { filtered = append(filtered, it) } }
	if len(filtered) == 0 { return nil, errors.New("libraryEntries encontrado pero vacío") }
	return filtered, nil
}

func balanced(s string, start int, open, close byte) (string, error) {
	if start < 0 || start >= len(s) || s[start] != open { return "", errors.New("inicio de bloque inválido") }
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped { escaped = false; continue }
			if ch == '\\' { escaped = true; continue }
			if ch == '"' { inString = false }
			continue
		}
		if ch == '"' { inString = true; continue }
		if ch == open { depth++ }
		if ch == close {
			depth--
			if depth == 0 { return s[start:i+1], nil }
		}
	}
	return "", errors.New("bloque libraryEntries incompleto")
}
