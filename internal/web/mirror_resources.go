package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type mirrorResourceFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Bytes    int64  `json:"bytes"`
	Modified string `json:"modified"`
}

type mirrorResourceNode struct {
	Name     string                `json:"name"`
	Path     string                `json:"path"`
	Files    []mirrorResourceFile  `json:"files,omitempty"`
	Children []*mirrorResourceNode `json:"children,omitempty"`
	Count    int                   `json:"count"`
}

type mirrorLogLine struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

func (s *Server) mirrorResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	state := s.mirror.Snapshot()
	// Lightweight mode is used by the admin polling loop. It never walks /data/site.
	if r.URL.Query().Get("full") != "1" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total": s.mirrorResourceCount(), "saved_this_sync": state.Fetched,
			"fetched": state.Fetched, "reused": state.Reused, "errors": state.Errors, "running": state.Running,
		})
		return
	}

	root := filepath.Clean(s.siteRoot)
	started, _ := time.Parse(time.RFC3339, state.Started)
	tree := &mirrorResourceNode{Name: "/data/site", Path: "/"}
	nodes := map[string]*mirrorResourceNode{".": tree}
	total := 0
	logs := make([]mirrorLogLine, 0, 256)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		rel, er := filepath.Rel(root, path)
		if er != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			parentRel := filepath.ToSlash(filepath.Dir(rel))
			if parentRel == "" {
				parentRel = "."
			}
			p := nodes[parentRel]
			if p == nil {
				p = tree
			}
			n := &mirrorResourceNode{Name: info.Name(), Path: "/" + rel}
			p.Children = append(p.Children, n)
			nodes[rel] = n
			return nil
		}
		if strings.HasPrefix(info.Name(), ".frontend-cache-") {
			return nil
		}
		total++
		parentRel := filepath.ToSlash(filepath.Dir(rel))
		if parentRel == "" {
			parentRel = "."
		}
		p := nodes[parentRel]
		if p == nil {
			p = tree
		}
		p.Files = append(p.Files, mirrorResourceFile{Name: info.Name(), Path: "/" + rel, Bytes: info.Size(), Modified: info.ModTime().Format(time.RFC3339)})
		if !started.IsZero() && !info.ModTime().Before(started) {
			logs = append(logs, mirrorLogLine{Time: info.ModTime().Format("15:04:05"), Level: "OK", Message: "SAVE /" + rel})
		}
		return nil
	})
	var calc func(*mirrorResourceNode) int
	calc = func(n *mirrorResourceNode) int {
		c := len(n.Files)
		for _, ch := range n.Children {
			c += calc(ch)
		}
		n.Count = c
		sort.Slice(n.Children, func(i, j int) bool { return strings.ToLower(n.Children[i].Name) < strings.ToLower(n.Children[j].Name) })
		sort.Slice(n.Files, func(i, j int) bool { return strings.ToLower(n.Files[i].Name) < strings.ToLower(n.Files[j].Name) })
		return c
	}
	calc(tree)
	for _, e := range state.ErrorLog {
		logs = append(logs, mirrorLogLine{Time: logClock(e.Time), Level: "ERR", Message: e.Message})
	}
	sort.SliceStable(logs, func(i, j int) bool { return logs[i].Time < logs[j].Time })
	_ = s.db.SetSetting("mirror_resource_count", fmtInt(total))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"total": total, "saved_this_sync": state.Fetched, "tree": tree, "log": logs, "fetched": state.Fetched, "reused": state.Reused, "errors": state.Errors, "running": state.Running})
}

func fmtInt(v int) string {
	if v == 0 {
		return "0"
	}
	b := make([]byte, 0, 12)
	for v > 0 {
		b = append(b, byte('0'+v%10))
		v /= 10
	}
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

func logClock(v string) string {
	if t, e := time.Parse(time.RFC3339, v); e == nil {
		return t.Format("15:04:05")
	}
	return v
}
