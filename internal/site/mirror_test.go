package site

import (
	"net/url"
	"strings"
	"testing"
)

func TestRewriteAnimeAV1AndCDNURLs(t *testing.T) {
	m := &Mirror{base: &url.URL{Scheme:"https", Host:"animeav1.com"}}
	in := `https://animeav1.com/media/test https://cdn.animeav1.com/screenshots/4350/4.jpg`
	out := m.rewrite(in)
	if !strings.Contains(out, `/media/test`) { t.Fatalf("animeav1 URL not localized: %s", out) }
	if !strings.Contains(out, `/_cdn/screenshots/4350/4.jpg`) { t.Fatalf("CDN URL not localized: %s", out) }
}

func TestSVGNamespaceIsNotNeutralized(t *testing.T) {
	m := &Mirror{base: &url.URL{Scheme:"https", Host:"animeav1.com"}}
	in := `<svg xmlns='http://www.w3.org/2000/svg'></svg>`
	out, _ := m.neutralizeExternalURLs(in)
	if out != in { t.Fatalf("SVG namespace was modified: %s", out) }
}

func TestHydrationFixCoversImageAttributes(t *testing.T) {
	for _, token := range []string{"src","srcset","poster","data-src","data-lazy-src","/_cdn/"} {
		if !strings.Contains(hydrationFixUI, token) { t.Fatalf("hydration fix missing %s", token) }
	}
}
