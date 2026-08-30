package adblock

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const DefaultEasyListURL = "https://easylist.to/easylist/easylist.txt"

type Blocker struct {
	mu        sync.RWMutex
	hosts     map[string]struct{}
	except    map[string]struct{}
	sourceURL string
	cachePath string
	updatedAt time.Time
	client    *http.Client
}

func New(cachePath string) *Blocker {
	b := &Blocker{
		hosts:     map[string]struct{}{},
		except:    map[string]struct{}{},
		sourceURL: DefaultEasyListURL,
		cachePath: cachePath,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
	_ = b.loadFile(cachePath)
	return b
}

func (b *Blocker) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.hosts)
}

func (b *Blocker) UpdatedAt() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.updatedAt
}

func (b *Blocker) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.sourceURL, nil)
	if err != nil { return err }
	req.Header.Set("User-Agent", "AnimeAV1-Archive EasyList/1.0")
	resp, err := b.client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("EasyList: %s", resp.Status) }
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil { return err }
	if len(data) < 1024 { return fmt.Errorf("EasyList: respuesta demasiado pequena") }
	if err := os.MkdirAll(filepath.Dir(b.cachePath), 0o755); err != nil { return err }
	tmp := b.cachePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil { return err }
	if err := b.loadBytes(data); err != nil { _ = os.Remove(tmp); return err }
	if err := os.Rename(tmp, b.cachePath); err != nil { return err }
	return nil
}

func (b *Blocker) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil { return err }
	return b.loadBytes(data)
}

func (b *Blocker) loadBytes(data []byte) error {
	hosts := map[string]struct{}{}
	except := map[string]struct{}{}
	s := bufio.NewScanner(strings.NewReader(string(data)))
	s.Buffer(make([]byte, 64*1024), 1024*1024)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "[") { continue }
		isException := strings.HasPrefix(line, "@@")
		if isException { line = strings.TrimPrefix(line, "@@") }
		if !strings.HasPrefix(line, "||") { continue }
		line = strings.TrimPrefix(line, "||")
		if i := strings.IndexByte(line, '$'); i >= 0 { line = line[:i] }
		if i := strings.IndexAny(line, "^/"); i >= 0 { line = line[:i] }
		h := normalizeHost(line)
		if h == "" || strings.ContainsAny(h, "*|~") { continue }
		if isException { except[h] = struct{}{} } else { hosts[h] = struct{}{} }
	}
	if err := s.Err(); err != nil { return err }
	b.mu.Lock()
	b.hosts = hosts
	b.except = except
	b.updatedAt = time.Now()
	b.mu.Unlock()
	return nil
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")
	if strings.Contains(host, "://") {
		if u, err := url.Parse(host); err == nil { host = strings.ToLower(u.Hostname()) }
	}
	if strings.ContainsAny(host, " []{}()\\") || !strings.Contains(host, ".") { return "" }
	return host
}

func (b *Blocker) BlockedHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, err := strings.Cut(host, ":"); err == false && strings.Count(host, ":") == 1 { host = h }
	host = strings.TrimSuffix(host, ".")
	if host == "" { return false }
	b.mu.RLock()
	defer b.mu.RUnlock()
	for h := host; h != ""; {
		if _, ok := b.except[h]; ok { return false }
		if _, ok := b.hosts[h]; ok { return true }
		idx := strings.IndexByte(h, '.')
		if idx < 0 { break }
		h = h[idx+1:]
	}
	return false
}
