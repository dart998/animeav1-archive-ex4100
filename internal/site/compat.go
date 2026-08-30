package site

import (
	"net/http"
	"regexp"
	"strings"
)

// animeAV1RefererTransport reproduce el contexto normal de navegación para
// recursos del CDN sin reenviar la cookie de sesión de AnimeAV1 al CDN.
type animeAV1RefererTransport struct {
	base http.RoundTripper
}

func (t *animeAV1RefererTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if !strings.EqualFold(req.URL.Hostname(), "cdn.animeav1.com") {
		return base.RoundTrip(req)
	}

	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set("Referer", "https://animeav1.com/")
	return base.RoundTrip(clone)
}

func init() {
	// El sanitizador genérico anterior interpretaba también namespaces XML como
	// URLs navegables. AnimeAV1 incluye muchos iconos como data:image/svg+xml con
	// xmlns='http://www.w3.org/2000/svg'; reemplazar ese valor rompe el SVG.
	//
	// La navegación externa ya queda cubierta por sanitizeNavAttrs, CSP,
	// iframe/script filtering y EasyList. Aquí restringimos la sustitución textual
	// a redes publicitarias conocidas, evitando modificar contenido SVG legítimo.
	absoluteURL = regexp.MustCompile(`(?i)https?://(?:[a-z0-9-]+\.)*(?:runative-syndicate\.com|runative\.com|popads[^/]*|adsterra[^/]*)[^\s"'<>)]*`)

	// Los GET directos al CDN pueden aplicar protección anti-hotlink. Conservamos
	// el transporte original y añadimos únicamente Referer para cdn.animeav1.com.
	// No se añade Cookie ni ninguna credencial.
	base := http.DefaultTransport
	http.DefaultTransport = &animeAV1RefererTransport{base: base}
}
