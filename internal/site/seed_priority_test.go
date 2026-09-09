package site

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPromoteSeedStaticsOverwritesStaticButNotHTML(t *testing.T) {
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	site := filepath.Join(root, "site")
	mustWrite := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil { t.Fatal(err) }
	}
	mustWrite(filepath.Join(seed, "_app", "common.css"), "seed-css")
	mustWrite(filepath.Join(seed, "_app", "bundle.js"), "seed-js")
	mustWrite(filepath.Join(seed, "index.html"), "seed-html")
	mustWrite(filepath.Join(site, "_app", "common.css"), "old-css")
	mustWrite(filepath.Join(site, "index.html"), "live-html")

	n, err := promoteSeedStatics(seed, site)
	if err != nil { t.Fatal(err) }
	if n != 2 { t.Fatalf("expected 2 promoted statics, got %d", n) }
	css, _ := os.ReadFile(filepath.Join(site, "_app", "common.css"))
	if string(css) != "seed-css" { t.Fatalf("seed did not win over cache: %q", css) }
	html, _ := os.ReadFile(filepath.Join(site, "index.html"))
	if string(html) != "live-html" { t.Fatalf("HTML must not be promoted from seed: %q", html) }
}

func TestPromoteSeedStaticsMissingSeedIsNoop(t *testing.T) {
	n, err := promoteSeedStatics(filepath.Join(t.TempDir(), "missing"), t.TempDir())
	if err != nil || n != 0 { t.Fatalf("missing seed should be noop: n=%d err=%v", n, err) }
}
