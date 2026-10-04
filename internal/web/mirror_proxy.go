package web

import (
	"bytes"
	"encoding/json"
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
	raw := strings.TrimSpace(s.db.GetSetting("mirror_resource_count"))
	if raw == "" {
		s.mirrorResourceCountCache.Store(0)
		_ = s.db.SetSetting("mirror_resource_count", "0")
		go s.recountMirrorResources()
	} else if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		s.mirrorResourceCountCache.Store(int64(n))
	}
	go s.watchMirrorResourceCounter()
}

func (s *Server) mirrorResourceCount() int {
	n := s.mirrorResourceCountCache.Load()
	if n < 0 { return 0 }
	return int(n)
}

func (s *Server) recountMirrorResources() int {
	root := filepath.Clean(s.siteRoot)
	total := 0
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() { return nil }
		if strings.HasPrefix(info.Name(), ".frontend-cache-") { return nil }
		total++
		return nil
	})
	s.mirrorResourceCountCache.Store(int64(total))
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

func archiveVersionBridge(version string) []byte {
	label := "AnimeAV1 Archive v" + strings.TrimSpace(version)
	q, _ := json.Marshal(label)
	return []byte(`<script id="mirror-app-version-script">(function(){var label=` + string(q) + `;function put(){if(document.getElementById('mirror-app-version'))return;var xs=Array.prototype.slice.call(document.querySelectorAll('p,span,div,small')).filter(function(e){return (e.textContent||'').replace(/\\s+/g,' ').trim()==='By fans for fans'});xs.sort(function(a,b){return (a.children.length-b.children.length)||((a.textContent||'').length-(b.textContent||'').length)});var t=xs[0];if(!t)return;var v=document.createElement('span');v.id='mirror-app-version';v.textContent=label;t.appendChild(v);t.setAttribute('data-mirror-fans-tagline','1')}function start(){put();new MutationObserver(put).observe(document.documentElement,{subtree:true,childList:true})}if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start,{once:true});else start()})();</script><style>[data-mirror-fans-tagline="1"]{white-space:nowrap!important}[data-mirror-fans-tagline="1"] #mirror-app-version{display:block!important;margin-top:4px!important;width:max-content!important;font-size:12px!important;line-height:1.4!important;opacity:.62;white-space:nowrap!important}</style>`)
}

func injectArchiveVersion(body []byte, version string) []byte {
	bridge := archiveVersionBridge(version)
	if i := bytes.LastIndex(bytes.ToLower(body), []byte("</body>")); i >= 0 {
		out := make([]byte, 0, len(body)+len(bridge))
		out = append(out, body[:i]...)
		out = append(out, bridge...)
		out = append(out, body[i:]...)
		return out
	}
	return append(body, bridge...)
}

func isBrandAsset(path string) bool {
	switch path {
	case "/img/logo.svg", "/img/logo-dark.svg", "/img/logo-ft.svg", "/img/logo-ft-dark.svg": return true
	default: return false
	}
}

func (s *Server) serveBrandAsset(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(s.static, "img", filepath.Base(r.URL.Path))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-AnimeAV1-Source", "bundled")
	http.ServeFile(w, r, path)
}

func (s *Server) siteMirror(w http.ResponseWriter, r *http.Request) {
	if isBrandAsset(r.URL.Path) { s.serveBrandAsset(w, r); return }
	if shouldProxyAnimeAV1(r) { s.proxyAnimeAV1(w, r); return }
	rr := httptest.NewRecorder()
	s.mirror.Handler().ServeHTTP(rr, r)
	res := rr.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	for k, vv := range res.Header { for _, v := range vv { w.Header().Add(k, v) } }
	if strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "text/html") {
		body = patchMirrorHTML(body)
		body = injectArchiveVersion(body, s.version)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}
	w.WriteHeader(res.StatusCode)
	if r.Method != http.MethodHead { _, _ = w.Write(body) }
}

func (s *Server) cdnResource(w http.ResponseWriter, r *http.Request) {
	rr := httptest.NewRecorder()
	s.mirror.Handler().ServeHTTP(rr, r)
	if rr.Code >= 200 && rr.Code < 400 {
		w.Header().Set("X-AnimeAV1-Source", "mirror")
		copyRecorder(w, rr)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/_cdn/")
	if p == "" { http.NotFound(w, r); return }
	u := &url.URL{Scheme:"https", Host:"cdn.animeav1.com", Path:"/"+p, RawQuery:r.URL.RawQuery}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-AnimeAV1-Source", "cdn-fallback")
	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
}
