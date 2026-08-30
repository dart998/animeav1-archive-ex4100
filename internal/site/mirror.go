package site

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type State struct {
	Running   bool
	Started   string
	Finished  string
	Fetched   int
	Errors    int
	Sanitized int
	Current   string
	LastError string
}

type Mirror struct {
	base   *url.URL
	root   string
	client *http.Client
	mu     sync.RWMutex
	state  State
}

var (
	attrRE       = regexp.MustCompile(`(?i)(?:href|src|poster|action)=["']([^"'#]+)["']`)
	srcsetRE     = regexp.MustCompile(`(?i)srcset=["']([^"']+)["']`)
	cssURLRE     = regexp.MustCompile(`(?i)url\(\s*["']?([^"')]+)`)
	importRE     = regexp.MustCompile(`(?i)(?:from\s*|import\s*\(\s*)["']([^"']+)["']`)
	scriptRE     = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script\s*>`)
	scriptSrcRE  = regexp.MustCompile(`(?i)\bsrc\s*=\s*["']([^"']+)["']`)
	eventAttrRE  = regexp.MustCompile(`(?is)\s+on[a-z0-9_-]+\s*=\s*(["'])(.*?)\1`)
	navAttrRE    = regexp.MustCompile(`(?is)\b(href|action|formaction)\s*=\s*(["'])(.*?)\2`)
	baseTagRE    = regexp.MustCompile(`(?is)<base\b[^>]*>`)
	headOpenRE   = regexp.MustCompile(`(?i)<head\b[^>]*>`)
	suspiciousJS = regexp.MustCompile(`(?i)(window\.open\s*\(|(?:window\.|document\.|top\.|parent\.)?location\s*(?:=|\.|\[)|location\.(?:assign|replace)\s*\(|javascript\s*:|https?://)`)
)

const localCSP = `<meta http-equiv="Content-Security-Policy" content="default-src 'self' data: blob:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; media-src 'self' blob:; frame-src 'self'; form-action 'self'; base-uri 'self'; object-src 'none'">`

func New(baseURL, root string) (*Mirror, error) {
	u, err := url.Parse(baseURL)
	if err != nil { return nil, err }
	return &Mirror{base: u, root: root, client: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (m *Mirror) Snapshot() State { m.mu.RLock(); defer m.mu.RUnlock(); return m.state }

func (m *Mirror) Start(ctx context.Context) bool {
	m.mu.Lock()
	if m.state.Running { m.mu.Unlock(); return false }
	m.state = State{Running: true, Started: time.Now().Format(time.RFC3339)}
	m.mu.Unlock()
	go m.run(ctx)
	return true
}

func (m *Mirror) run(ctx context.Context) {
	defer func() { m.mu.Lock(); m.state.Running = false; m.state.Finished = time.Now().Format(time.RFC3339); m.mu.Unlock() }()

	buildRoot := m.root + ".building"
	_ = os.RemoveAll(buildRoot)
	if err := os.MkdirAll(buildRoot, 0o755); err != nil { m.fail(err); return }

	queue := []string{m.base.String()}
	seen := map[string]bool{}
	for len(queue) > 0 {
		select { case <-ctx.Done(): return; default: }
		raw := queue[0]
		queue = queue[1:]
		u, err := url.Parse(raw)
		if err != nil { continue }
		u.Fragment = ""
		if !m.allowedHost(u.Host) || m.skipURL(u) { continue }
		u.RawQuery = ""
		key := u.String()
		if seen[key] { continue }
		seen[key] = true

		m.mu.Lock(); m.state.Current = key; m.mu.Unlock()
		body, ctype, err := m.fetch(ctx, key)
		if err != nil { m.recordError(err); continue }

		if m.isText(ctype, u.Path) {
			text := string(body)
			for _, ref := range discoverRefs(text) {
				if abs := m.resolve(u, ref); abs != "" { queue = append(queue, abs) }
			}
			if strings.Contains(strings.ToLower(ctype), "text/html") || strings.EqualFold(filepath.Ext(u.Path), ".html") || u.Path == "" || u.Path == "/" {
				var n int
				text, n = m.sanitizeHTML(u, text)
				if n > 0 { m.mu.Lock(); m.state.Sanitized += n; m.mu.Unlock() }
			}
			text = m.rewrite(text)
			body = []byte(text)
		}

		if err = m.save(buildRoot, u, body, ctype); err != nil { m.recordError(err); continue }
		m.mu.Lock(); m.state.Fetched++; m.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	index := filepath.Join(buildRoot, "index.html")
	info, err := os.Stat(index)
	if err != nil || info.Size() < 1024 {
		m.fail(fmt.Errorf("mirror incompleto: index.html no existe o es demasiado pequeno"))
		return
	}

	oldRoot := m.root + ".old"
	_ = os.RemoveAll(oldRoot)
	if _, err = os.Stat(m.root); err == nil { if err = os.Rename(m.root, oldRoot); err != nil { m.fail(err); return } }
	if err = os.Rename(buildRoot, m.root); err != nil {
		_ = os.Rename(oldRoot, m.root)
		m.fail(err)
		return
	}
	_ = os.RemoveAll(oldRoot)
	m.mu.Lock(); m.state.LastError = ""; m.state.Current = ""; m.mu.Unlock()
}

func discoverRefs(text string) []string {
	out := []string{}
	for _, re := range []*regexp.Regexp{attrRE, cssURLRE, importRE} {
		for _, x := range re.FindAllStringSubmatch(text, -1) { if len(x) > 1 { out = append(out, strings.TrimSpace(x[1])) } }
	}
	for _, x := range srcsetRE.FindAllStringSubmatch(text, -1) {
		if len(x) < 2 { continue }
		for _, item := range strings.Split(x[1], ",") {
			if f := strings.Fields(strings.TrimSpace(item)); len(f) > 0 { out = append(out, f[0]) }
		}
	}
	return out
}

func (m *Mirror) sanitizeHTML(page *url.URL, text string) (string, int) {
	removed := 0

	// A local mirror must not inherit a remote <base> that can make relative links leave the NAS.
	text = baseTagRE.ReplaceAllStringFunc(text, func(s string) string { removed++; return "" })

	// Remove remote scripts and inline scripts that contain popup/redirect patterns.
	text = scriptRE.ReplaceAllStringFunc(text, func(block string) string {
		parts := scriptRE.FindStringSubmatch(block)
		if len(parts) < 3 { return block }
		attrs, body := parts[1], parts[2]
		if sm := scriptSrcRE.FindStringSubmatch(attrs); len(sm) > 1 {
			raw := strings.TrimSpace(sm[1])
			r, err := url.Parse(raw)
			if err != nil { removed++; return "" }
			u := page.ResolveReference(r)
			if (u.Scheme == "http" || u.Scheme == "https") && !m.allowedHost(u.Host) { removed++; return "" }
		}
		if suspiciousJS.MatchString(body) { removed++; return "" }
		return block
	})

	// Strip click/mouse/touch handlers that try to open windows or navigate away.
	text = eventAttrRE.ReplaceAllStringFunc(text, func(attr string) string {
		m := eventAttrRE.FindStringSubmatch(attr)
		if len(m) > 2 && suspiciousJS.MatchString(m[2]) { removed++; return "" }
		return attr
	})

	// Neutralize direct third-party links/forms and javascript: links.
	text = navAttrRE.ReplaceAllStringFunc(text, func(attr string) string {
		m := navAttrRE.FindStringSubmatch(attr)
		if len(m) < 4 { return attr }
		name, quote, raw := m[1], m[2], strings.TrimSpace(m[3])
		lower := strings.ToLower(raw)
		if strings.HasPrefix(lower, "javascript:") {
			removed++
			if strings.EqualFold(name, "href") { return name + "=" + quote + "#" + quote }
			return name + "=" + quote + quote
		}
		r, err := url.Parse(raw)
		if err != nil { return attr }
		u := page.ResolveReference(r)
		if (u.Scheme == "http" || u.Scheme == "https") && !m.allowedHost(u.Host) {
			removed++
			if strings.EqualFold(name, "href") { return name + "=" + quote + "#" + quote }
			return name + "=" + quote + quote
		}
		return attr
	})

	// Browser-side safety net: no third-party scripts, frames, forms or network calls.
	if headOpenRE.MatchString(text) {
		text = headOpenRE.ReplaceAllString(text, `${0}`+localCSP)
	} else {
		text = localCSP + text
	}
	return text, removed
}

func (m *Mirror) allowedHost(host string) bool { return host == m.base.Host || host == "cdn.animeav1.com" }

func (m *Mirror) skipURL(u *url.URL) bool {
	p := strings.ToLower(u.Path)
	for _, ext := range []string{".m3u8", ".mp4", ".mkv", ".webm", ".avi", ".mov", ".ts", ".mp3", ".m4a", ".aac"} { if strings.HasSuffix(p, ext) { return true } }
	return strings.HasPrefix(p, "/api/video") || strings.Contains(p, "/stream/")
}

func (m *Mirror) resolve(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "blob:") || strings.HasPrefix(ref, "javascript:") || strings.HasPrefix(ref, "mailto:") { return "" }
	r, err := url.Parse(ref)
	if err != nil { return "" }
	u := base.ResolveReference(r)
	if (u.Scheme != "http" && u.Scheme != "https") || !m.allowedHost(u.Host) || m.skipURL(u) { return "" }
	return u.String()
}

func (m *Mirror) rewrite(text string) string {
	for _, prefix := range []string{"https://" + m.base.Host + "/", "http://" + m.base.Host + "/", "//" + m.base.Host + "/"} { text = strings.ReplaceAll(text, prefix, "/") }
	for _, prefix := range []string{"https://cdn.animeav1.com/", "http://cdn.animeav1.com/", "//cdn.animeav1.com/"} { text = strings.ReplaceAll(text, prefix, "/_cdn/") }
	return text
}

func (m *Mirror) isText(ctype, path string) bool {
	c := strings.ToLower(ctype)
	e := strings.ToLower(filepath.Ext(path))
	return strings.Contains(c, "text/") || strings.Contains(c, "javascript") || strings.Contains(c, "json") || e == ".js" || e == ".mjs" || e == ".css" || e == ".html"
}

func (m *Mirror) fetch(ctx context.Context, u string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil { return nil, "", err }
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux armv7l) AppleWebKit/537.36 Chrome/124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/javascript,text/css,image/avif,image/webp,image/png,image/svg+xml,*/*;q=0.8")
	resp, err := m.client.Do(req)
	if err != nil { return nil, "", err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 { return nil, "", fmt.Errorf("GET %s: %s", u, resp.Status) }
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil { return nil, "", err }
	if len(b) == 0 { return nil, "", fmt.Errorf("GET %s: respuesta vacia", u) }
	return b, resp.Header.Get("Content-Type"), nil
}

func (m *Mirror) save(root string, u *url.URL, body []byte, ctype string) error {
	p := strings.TrimPrefix(filepath.Clean(u.Path), string(filepath.Separator))
	if u.Host == "cdn.animeav1.com" { p = filepath.Join("_cdn", p) }
	if p == "." || p == "" { p = "index.html" } else if strings.Contains(strings.ToLower(ctype), "text/html") && filepath.Ext(p) == "" { p = filepath.Join(p, "index.html") }
	full := filepath.Join(root, p)
	cleanRoot := filepath.Clean(root) + string(filepath.Separator)
	if !strings.HasPrefix(full, cleanRoot) && full != filepath.Join(root, "index.html") { return fmt.Errorf("unsafe path: %s", p) }
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { return err }
	return os.WriteFile(full, body, 0o644)
}

func (m *Mirror) recordError(err error) { m.mu.Lock(); m.state.Errors++; m.state.LastError = err.Error(); m.mu.Unlock() }
func (m *Mirror) fail(err error) { m.mu.Lock(); m.state.Errors++; m.state.LastError = err.Error(); m.mu.Unlock() }
