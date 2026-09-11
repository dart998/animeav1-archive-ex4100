package web

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	libraryindex "github.com/dart998/animeav1-archive-ex4100/internal/library"
)

type downloadEpisodeState struct {
	Episode    int    `json:"episode"`
	Status     string `json:"status"`
	Provider   string `json:"provider,omitempty"`
	File       string `json:"file,omitempty"`
	Bytes      int64  `json:"bytes,omitempty"`
	TotalBytes int64  `json:"total_bytes,omitempty"`
	Error      string `json:"error,omitempty"`
}

type seriesDownloadState struct {
	Slug            string                 `json:"slug"`
	Running         bool                   `json:"running"`
	Current         int                    `json:"current"`
	Total           int                    `json:"total"`
	Downloaded      int                    `json:"downloaded"`
	Skipped         int                    `json:"skipped"`
	Errors          int                    `json:"errors"`
	CurrentFile     string                 `json:"current_file,omitempty"`
	LastError       string                 `json:"last_error,omitempty"`
	Started         string                 `json:"started,omitempty"`
	Finished        string                 `json:"finished,omitempty"`
	Episodes        []downloadEpisodeState `json:"episodes"`
	ForceUnplayable bool                   `json:"-"`
}

type episodeNotification struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Episode   int    `json:"episode"`
	CreatedAt string `json:"created_at"`
	ReadAt    string `json:"read_at,omitempty"`
}

var downloadStates = struct {
	sync.RWMutex
	m map[string]*seriesDownloadState
}{m: map[string]*seriesDownloadState{}}
var downloadQueueMu sync.Mutex
var megaDownloadRE = regexp.MustCompile(`server:"Mega",url:"([^"]+)"`)

func (s *Server) persistDownloadState(st *seriesDownloadState) {
	b, _ := json.Marshal(st)
	_ = s.db.SetSetting("download_state_"+st.Slug, string(b))
}
func (s *Server) setDL(st *seriesDownloadState, f func(*seriesDownloadState)) {
	downloadStates.Lock()
	f(st)
	copy := *st
	copy.Episodes = append([]downloadEpisodeState(nil), st.Episodes...)
	downloadStates.Unlock()
	s.persistDownloadState(&copy)
}
func episodeState(st *seriesDownloadState, ep int) *downloadEpisodeState {
	for i := range st.Episodes {
		if st.Episodes[i].Episode == ep {
			return &st.Episodes[i]
		}
	}
	return nil
}

func (s *Server) downloadSeriesAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	item, ok := s.itemForSlug(slug)
	if !ok {
		http.Error(w, "serie no encontrada", 404)
		return
	}
	total := item.Total
	if total < 1 {
		total = s.db.SeriesEpisodeCount(slug)
	}
	if total < 1 {
		http.Error(w, "sin episodios", 409)
		return
	}
	if lib, e := libraryindex.Scan(s.libraryRoot); e == nil {
		_ = s.db.ReplaceLibrary(lib)
	}
	forceUnplayable := r.FormValue("force_unplayable") == "1"
	if !forceUnplayable {
		var unplayable []int
		for ep := 1; ep <= total; ep++ {
			if info, e := s.localEpisode(slug, ep); e == nil && info.Available && !info.BrowserPlayable {
				unplayable = append(unplayable, ep)
			}
		}
		if len(unplayable) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"requires_confirmation": true, "unplayable_episodes": unplayable})
			return
		}
	}
	downloadStates.Lock()
	if x := downloadStates.m[slug]; x != nil && x.Running {
		downloadStates.Unlock()
		http.Error(w, "descarga en curso", 409)
		return
	}
	st := &seriesDownloadState{Slug: slug, Running: true, Total: total, Started: time.Now().Format(time.RFC3339), Episodes: make([]downloadEpisodeState, total), ForceUnplayable: forceUnplayable}
	for i := 1; i <= total; i++ {
		st.Episodes[i-1] = downloadEpisodeState{Episode: i, Status: "pending"}
	}
	downloadStates.m[slug] = st
	downloadStates.Unlock()
	s.persistDownloadState(st)
	go s.runMegaSeries(item, st)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(st)
}

func (s *Server) downloadStatusAPI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("notifications") == "1" {
		s.notificationsAPI(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	downloadStates.RLock()
	x := downloadStates.m[slug]
	if x != nil {
		c := *x
		c.Episodes = append([]downloadEpisodeState(nil), x.Episodes...)
		downloadStates.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(c)
		return
	}
	downloadStates.RUnlock()
	raw := strings.TrimSpace(s.db.GetSetting("download_state_" + slug))
	if raw == "" {
		http.Error(w, "sin descarga", 404)
		return
	}
	var c seriesDownloadState
	if json.Unmarshal([]byte(raw), &c) != nil {
		http.Error(w, "sin descarga", 404)
		return
	}
	if c.Running {
		c.Running = false
		c.LastError = "Descarga interrumpida por reinicio del contenedor"
		c.Finished = time.Now().Format(time.RFC3339)
		s.persistDownloadState(&c)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

func (s *Server) loadNotifications() []episodeNotification {
	var out []episodeNotification
	_ = json.Unmarshal([]byte(s.db.GetSetting("episode_notifications_json")), &out)
	cut := time.Now().Add(-24 * time.Hour)
	keep := out[:0]
	changed := false
	for _, n := range out {
		if n.ReadAt != "" {
			if t, e := time.Parse(time.RFC3339, n.ReadAt); e == nil && t.Before(cut) {
				changed = true
				continue
			}
		}
		keep = append(keep, n)
	}
	if changed {
		b, _ := json.Marshal(keep)
		_ = s.db.SetSetting("episode_notifications_json", string(b))
	}
	return keep
}
func (s *Server) notificationsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		n := s.loadNotifications()
		unread := 0
		for _, x := range n {
			if x.ReadAt == "" {
				unread++
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"notifications": n, "unread": unread})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if e := r.ParseForm(); e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	action := r.FormValue("action")
	id := r.FormValue("id")
	n := s.loadNotifications()
	now := time.Now().Format(time.RFC3339)
	for i := range n {
		if action == "read_all" || (action == "read" && n[i].ID == id) {
			if n[i].ReadAt == "" {
				n[i].ReadAt = now
			}
		}
	}
	b, _ := json.Marshal(n)
	_ = s.db.SetSetting("episode_notifications_json", string(b))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) runMegaSeries(item animeav1.Item, st *seriesDownloadState) {
	downloadQueueMu.Lock()
	defer downloadQueueMu.Unlock()
	target, err := s.downloadTargetDir(item)
	if err != nil {
		s.setDL(st, func(x *seriesDownloadState) {
			x.Running = false
			x.Errors++
			x.LastError = err.Error()
			x.Finished = time.Now().Format(time.RFC3339)
		})
		return
	}
	for ep := 1; ep <= st.Total; ep++ {
		s.setDL(st, func(x *seriesDownloadState) {
			x.Current = ep
			x.CurrentFile = ""
			if e := episodeState(x, ep); e != nil {
				e.Status = "checking"
			}
		})
		if info, e := s.localEpisode(item.Slug, ep); e == nil && info.Available && (info.BrowserPlayable || !st.ForceUnplayable) {
			s.setDL(st, func(x *seriesDownloadState) {
				x.Skipped++
				if q := episodeState(x, ep); q != nil {
					q.Status = "existing"
					q.File = info.FileName
				}
			})
			continue
		}
		mega, e := s.episodeMegaDownloadURL(context.Background(), item.Slug, ep)
		if e != nil {
			s.setDL(st, func(x *seriesDownloadState) {
				x.Errors++
				x.LastError = fmt.Sprintf("Ep %d: %v", ep, e)
				if q := episodeState(x, ep); q != nil {
					q.Status = "error"
					q.Error = e.Error()
				}
			})
			continue
		}
		s.setDL(st, func(x *seriesDownloadState) {
			if q := episodeState(x, ep); q != nil {
				q.Status = "downloading"
				q.Provider = "Mega"
			}
		})
		p, e := downloadMega(context.Background(), mega, target, func(name string, done, total int64) {
			s.setDL(st, func(x *seriesDownloadState) {
				x.CurrentFile = name
				if q := episodeState(x, ep); q != nil {
					q.File = name
					q.Bytes = done
					q.TotalBytes = total
				}
			})
		})
		if e != nil {
			s.setDL(st, func(x *seriesDownloadState) {
				x.Errors++
				x.LastError = fmt.Sprintf("Ep %d: %v", ep, e)
				if q := episodeState(x, ep); q != nil {
					q.Status = "error"
					q.Error = e.Error()
				}
			})
			continue
		}
		p = ensureDownloadedEpisodeName(p, item.MediaID, ep)
		if items, e := libraryindex.Scan(s.libraryRoot); e == nil {
			_ = s.db.ReplaceLibrary(items)
		}
		s.setDL(st, func(x *seriesDownloadState) {
			x.Downloaded++
			x.CurrentFile = filepath.Base(p)
			if q := episodeState(x, ep); q != nil {
				q.Status = "completed"
				q.File = filepath.Base(p)
				q.Error = ""
			}
		})
	}
	s.setDL(st, func(x *seriesDownloadState) {
		x.Running = false
		x.CurrentFile = ""
		x.Finished = time.Now().Format(time.RFC3339)
	})
}

func ensureDownloadedEpisodeName(path string, mediaID animeav1.IDString, episode int) string {
	if episodeFileRank(filepath.Base(path), mediaID, episode) < 99 {
		return path
	}
	dir, name := filepath.Dir(path), filepath.Base(path)
	prefix := fmt.Sprintf("EP%02d_", episode)
	if id := strings.TrimSpace(string(mediaID)); id != "" {
		prefix = id + "_" + strconv.Itoa(episode) + "_"
	}
	dest := filepath.Join(dir, prefix+name)
	if _, e := os.Stat(dest); e == nil {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(prefix+name, ext)
		for i := 1; ; i++ {
			candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
			if _, e := os.Stat(candidate); os.IsNotExist(e) {
				dest = candidate
				break
			}
		}
	}
	if e := os.Rename(path, dest); e == nil {
		_ = os.Chmod(dest, 0666)
		return dest
	}
	return path
}

func (s *Server) episodeMegaDownloadURL(ctx context.Context, slug string, ep int) (string, error) {
	b, e := s.fetchEpisodeHTML(ctx, slug, ep)
	if e != nil {
		return "", e
	}
	for _, m := range megaDownloadRE.FindAllSubmatch(b, -1) {
		if len(m) < 2 {
			continue
		}
		raw := string(m[1])
		u, e := url.Parse(raw)
		if e != nil {
			continue
		}
		host := strings.ToLower(u.Hostname())
		path := strings.ToLower(u.Path)
		if (strings.Contains(host, "mega.nz") || strings.Contains(host, "mega.co.nz")) && (strings.Contains(path, "/file/") || strings.HasPrefix(u.Fragment, "!")) {
			return raw, nil
		}
	}
	return "", errors.New("sin enlace de descarga Mega para este episodio")
}

func (s *Server) downloadTargetDir(item animeav1.Item) (string, error) {
	lib, e := libraryindex.Scan(s.libraryRoot)
	if e != nil {
		return "", e
	}
	_ = s.db.ReplaceLibrary(lib)
	if c := localFolderCandidates(item, lib); len(c) > 0 {
		return c[0].Item.Path, nil
	}
	name := safeDirName(item.Title)
	if name == "" {
		name = "Anime"
	}
	dir := filepath.Join(s.libraryRoot, name)
	if e = os.MkdirAll(dir, 0777); e != nil {
		return "", e
	}
	if e = os.Chmod(dir, 0777); e != nil {
		return "", e
	}
	return dir, nil
}
func safeDirName(s string) string {
	return strings.TrimSpace(strings.NewReplacer("/", "-", "\\", "-", ":", " -", "*", "", "?", "", "\"", "", "<", "", ">", "", "|", "-").Replace(s))
}

type megaResp struct {
	G  string `json:"g"`
	S  int64  `json:"s"`
	AT string `json:"at"`
}
type progressReader struct {
	r     io.Reader
	done  int64
	total int64
	last  time.Time
	cb    func(int64, int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, e := p.r.Read(b)
	p.done += int64(n)
	now := time.Now()
	if p.cb != nil && (p.last.IsZero() || now.Sub(p.last) >= time.Second || p.done == p.total) {
		p.last = now
		p.cb(p.done, p.total)
	}
	return n, e
}

func downloadMega(ctx context.Context, raw, target string, progress func(string, int64, int64)) (string, error) {
	u, e := url.Parse(raw)
	if e != nil {
		return "", e
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	var handle, fragment string
	if len(parts) >= 2 && strings.EqualFold(parts[0], "file") {
		handle = parts[1]
		fragment = u.Fragment
	} else if u.Fragment != "" && strings.Contains(u.Fragment, "!") {
		bits := strings.Split(strings.TrimPrefix(u.Fragment, "!"), "!")
		if len(bits) >= 2 {
			handle = bits[0]
			fragment = bits[1]
		}
	}
	if handle == "" || fragment == "" {
		return "", errors.New("URL Mega de descarga invalida")
	}
	keyRaw, e := base64.RawURLEncoding.DecodeString(strings.TrimSpace(fragment))
	if e != nil || len(keyRaw) != 32 {
		return "", errors.New("clave Mega invalida")
	}
	key := make([]byte, 16)
	for i := 0; i < 16; i++ {
		key[i] = keyRaw[i] ^ keyRaw[i+16]
	}
	iv := make([]byte, 16)
	copy(iv, keyRaw[16:24])
	payload := fmt.Sprintf(`[{"a":"g","g":1,"p":"%s"}]`, handle)
	api := "https://g.api.mega.co.nz/cs?id=" + strconv.FormatInt(time.Now().UnixNano(), 10)
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, api, strings.NewReader(payload))
	if e != nil {
		return "", e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return "", e
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	resp.Body.Close()
	if e != nil {
		return "", e
	}
	var rr []megaResp
	if json.Unmarshal(b, &rr) != nil || len(rr) == 0 || rr[0].G == "" {
		return "", fmt.Errorf("Mega API: %s", strings.TrimSpace(string(b)))
	}
	name := "mega-" + handle + ".mp4"
	if rr[0].AT != "" {
		if n := megaName(rr[0].AT, key); n != "" {
			name = filepath.Base(n)
		}
	}
	if err := os.MkdirAll(target, 0777); err != nil {
		return "", err
	}
	dest := filepath.Join(target, name)
	if _, e := os.Stat(dest); e == nil {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		for i := 1; ; i++ {
			candidate := filepath.Join(target, fmt.Sprintf("%s (descarga %d)%s", base, i, ext))
			if _, e := os.Stat(candidate); os.IsNotExist(e) {
				dest = candidate
				name = filepath.Base(candidate)
				break
			}
		}
	}
	part := dest + ".part"
	get, e := http.NewRequestWithContext(ctx, http.MethodGet, rr[0].G, nil)
	if e != nil {
		return "", e
	}
	r, e := http.DefaultClient.Do(get)
	if e != nil {
		return "", e
	}
	defer r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return "", fmt.Errorf("Mega descarga: %s", r.Status)
	}
	f, e := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if e != nil {
		return "", e
	}
	_ = os.Chmod(part, 0666)
	block, e := aes.NewCipher(key)
	if e != nil {
		f.Close()
		return "", e
	}
	stream := &cipher.StreamReader{S: cipher.NewCTR(block, iv), R: r.Body}
	pr := &progressReader{r: stream, total: rr[0].S, cb: func(done, total int64) {
		if progress != nil {
			progress(name, done, total)
		}
	}}
	if progress != nil {
		progress(name, 0, rr[0].S)
	}
	_, copyErr := io.Copy(f, pr)
	closeErr := f.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if rr[0].S > 0 {
		st, e := os.Stat(part)
		if e != nil {
			return "", e
		}
		if st.Size() != rr[0].S {
			return "", fmt.Errorf("descarga Mega incompleta: %d/%d bytes", st.Size(), rr[0].S)
		}
	}
	if e = os.Rename(part, dest); e != nil {
		return "", e
	}
	_ = os.Chmod(dest, 0666)
	return dest, nil
}

func megaName(at string, key []byte) string {
	b, e := base64.RawURLEncoding.DecodeString(at)
	if e != nil || len(b) == 0 || len(b)%16 != 0 {
		return ""
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return ""
	}
	out := make([]byte, len(b))
	cipher.NewCBCDecrypter(block, make([]byte, 16)).CryptBlocks(out, b)
	s := strings.TrimRight(string(out), "\x00")
	if !strings.HasPrefix(s, "MEGA") {
		return ""
	}
	var x struct {
		N string `json:"n"`
	}
	if json.Unmarshal([]byte(s[4:]), &x) != nil {
		return ""
	}
	return x.N
}
