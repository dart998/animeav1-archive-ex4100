package animeav1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
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
	case 1: return "Planeado"
	case 2: return "Completado"
	case 3: return "En pausa"
	case 4: return "Abandonado"
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
	low := strings.ToLower(string(b))
	if strings.Contains(low, "verifique que es un ser humano") || (strings.Contains(low, "iniciar sesión") && !strings.Contains(string(b), "libraryEntries:")) {
		return nil, errors.New("cookie caducada o sesión no válida")
	}
	return parseLibraryEntries(string(b))
}

var (
	reIntField    = func(name string) *regexp.Regexp { return regexp.MustCompile(regexp.QuoteMeta(name) + `\s*:\s*(-?\d+|null)`) }
	reBoolField   = func(name string) *regexp.Regexp { return regexp.MustCompile(regexp.QuoteMeta(name) + `\s*:\s*(true|false)`) }
	reStringField = func(name string) *regexp.Regexp { return regexp.MustCompile(regexp.QuoteMeta(name) + `\s*:\s*("(?:\\.|[^"\\])*")`) }
	reAlias       = regexp.MustCompile(`("(?:\\.|[^"\\])*")\s*:\s*("(?:\\.|[^"\\])*")`)
)

func jsString(v string) string {
	if v == "" { return "" }
	x, err := strconv.Unquote(v)
	if err != nil { return strings.Trim(v, `"`) }
	return x
}

func fieldInt(block, name string) int {
	m := reIntField(name).FindStringSubmatch(block)
	if len(m) < 2 || m[1] == "null" { return 0 }
	n, _ := strconv.Atoi(m[1])
	return n
}

func fieldBool(block, name string) bool {
	m := reBoolField(name).FindStringSubmatch(block)
	return len(m) > 1 && m[1] == "true"
}

func fieldID(block, name string) IDString {
	re := regexp.MustCompile(`(?:"?` + regexp.QuoteMeta(name) + `"?)\s*:\s*(?:"([^"\\]*(?:\\.[^"\\]*)*)"|(-?[0-9]+))`)
	m := re.FindStringSubmatch(block)
	if len(m) < 3 { return "" }
	if m[1] != "" { return IDString(jsString(`"` + m[1] + `"`)) }
	return IDString(m[2])
}

func fieldString(block, name string) string {
	m := reStringField(name).FindStringSubmatch(block)
	if len(m) < 2 { return "" }
	return jsString(m[1])
}

func balanced(src string, start int, open, close byte) (string, error) {
	if start < 0 || start >= len(src) || src[start] != open { return "", errors.New("inicio de bloque inválido") }
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(src); i++ {
		c := src[i]
		if inString {
			if escaped { escaped = false } else if c == '\\' { escaped = true } else if c == '"' { inString = false }
			continue
		}
		if c == '"' { inString = true; continue }
		if c == open { depth++ }
		if c == close {
			depth--
			if depth == 0 { return src[start:i+1], nil }
		}
	}
	return "", errors.New("bloque SvelteKit incompleto")
}

func splitTopObjects(array string) []string {
	out := []string{}
	depth := 0
	start := -1
	inString := false
	escaped := false
	for i := 0; i < len(array); i++ {
		c := array[i]
		if inString {
			if escaped { escaped = false } else if c == '\\' { escaped = true } else if c == '"' { inString = false }
			continue
		}
		if c == '"' { inString = true; continue }
		if c == '{' { if depth == 0 { start = i }; depth++ }
		if c == '}' {
			depth--
			if depth == 0 && start >= 0 { out = append(out, array[start:i+1]); start = -1 }
		}
	}
	return out
}

func extractObject(block, key string) string {
	i := strings.Index(block, key+":")
	if i < 0 { return "" }
	j := strings.Index(block[i:], "{")
	if j < 0 { return "" }
	v, _ := balanced(block, i+j, '{', '}')
	return v
}

func parseLibraryEntries(body string) ([]Item, error) {
	i := strings.Index(body, "libraryEntries:")
	if i < 0 { return nil, errors.New("AnimeAV1 no contiene libraryEntries; cookie caducada o formato cambiado") }
	j := strings.Index(body[i:], "[")
	if j < 0 { return nil, errors.New("libraryEntries sin array") }
	arr, err := balanced(body, i+j, '[', ']')
	if err != nil { return nil, err }
	objects := splitTopObjects(arr)
	items := make([]Item, 0, len(objects))
	for _, obj := range objects {
		media := extractObject(obj, "media")
		if media == "" { continue }
		it := Item{
			MediaID: fieldID(obj, "mediaId"),
			Status: fieldInt(obj, "status"),
			Seen: fieldInt(obj, "episode"),
			Score: fieldInt(obj, "score"),
			Favorite: fieldBool(obj, "favorite"),
			Title: fieldString(media, "title"),
			Total: fieldInt(media, "episodesCount"),
			Slug: fieldString(media, "slug"),
			Aliases: map[string]string{},
		}
		aka := extractObject(media, "aka")
		for _, m := range reAlias.FindAllStringSubmatch(aka, -1) {
			if len(m) == 3 { it.Aliases[jsString(m[1])] = jsString(m[2]) }
		}
		if strings.TrimSpace(it.Title) != "" { items = append(items, it) }
	}
	if len(items) == 0 { return nil, errors.New("libraryEntries encontrado pero no se pudo leer ninguna entrada") }
	return items, nil
}
