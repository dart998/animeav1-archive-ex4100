package web

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *Server) initMirrorResourceCounter() {
	if strings.TrimSpace(s.db.GetSetting("mirror_resource_count")) == "" {
		_ = s.db.SetSetting("mirror_resource_count", "0")
		go s.recountMirrorResources()
	}
	go s.watchMirrorResourceCounter()
}

func (s *Server) mirrorResourceCount() int {
	n, _ := strconv.Atoi(strings.TrimSpace(s.db.GetSetting("mirror_resource_count")))
	if n < 0 { return 0 }
	return n
}

func (s *Server) recountMirrorResources() int {
	root := filepath.Clean(s.siteRoot)
	total := 0
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() { return nil }
		name := info.Name()
		if strings.HasPrefix(name, ".frontend-cache-") { return nil }
		total++
		return nil
	})
	_ = s.db.SetSetting("mirror_resource_count", strconv.Itoa(total))
	return total
}

func (s *Server) watchMirrorResourceCounter() {
	t := time.NewTicker(4 * time.Second)
	defer t.Stop()
	wasRunning := false
	for range t.C {
		running := s.mirror.Snapshot().Running
		if wasRunning && !running { s.recountMirrorResources() }
		wasRunning = running
	}
}

func copyRecorder(w http.ResponseWriter, rr *httptest.ResponseRecorder) {
	res := rr.Result()
	defer res.Body.Close()
	for k, vv := range res.Header { for _, v := range vv { w.Header().Add(k, v) } }
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}

func patchMirrorHTML(body []byte) []byte {
	body = bytes.ReplaceAll(body, []byte("img-src 'self' data: blob:;"), []byte("img-src 'self' data: blob: https://cdn.animeav1.com;"))
	body = bytes.ReplaceAll(body, []byte("frame-src 'self';"), []byte("frame-src 'self' https:;"))
	return body
}

func (s *Server) siteMirror(w http.ResponseWriter, r *http.Request) {
	rr := httptest.NewRecorder()
	s.mirror.Handler().ServeHTTP(rr, r)
	res := rr.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	for k, vv := range res.Header { for _, v := range vv { w.Header().Add(k, v) } }
	if strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "text/html") {
		body = patchMirrorHTML(body)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}
	w.WriteHeader(res.StatusCode)
	if r.Method != http.MethodHead { _, _ = w.Write(body) }
}

func (s *Server) cdnResource(w http.ResponseWriter, r *http.Request) {
	rr := httptest.NewRecorder()
	s.mirror.Handler().ServeHTTP(rr, r)
	if rr.Code >= 200 && rr.Code < 400 { copyRecorder(w, rr); return }
	p := strings.TrimPrefix(r.URL.Path, "/_cdn/")
	if p == "" { http.NotFound(w, r); return }
	u := &url.URL{Scheme:"https", Host:"cdn.animeav1.com", Path:"/"+p, RawQuery:r.URL.RawQuery}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
}
