package site

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// animeAV1RefererTransport reproduce el contexto normal de navegación para
// recursos del CDN sin reenviar la cookie de sesión de AnimeAV1 al CDN.
// Además usa /data/seed como fallback local para recursos que el origen
// rechaza con 403/404.
type animeAV1RefererTransport struct {
	base     http.RoundTripper
	seedRoot string
}

func (t *animeAV1RefererTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	request := req
	if strings.EqualFold(req.URL.Hostname(), "cdn.animeav1.com") {
		clone := req.Clone(req.Context())
		clone.Header = req.Header.Clone()
		clone.Header.Set("Referer", "https://animeav1.com/")
		request = clone
	}

	resp, err := base.RoundTrip(request)
	if err == nil && resp != nil && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		return resp, nil
	}

	seedResp, seedErr := t.seedResponse(req)
	if seedErr == nil && seedResp != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return seedResp, nil
	}

	return resp, err
}

func (t *animeAV1RefererTransport) seedResponse(req *http.Request) (*http.Response, error) {
	root := t.seedRoot
	if root == "" {
		root = "/data/seed"
	}

	host := strings.ToLower(req.URL.Hostname())
	path := filepath.Clean("/" + strings.TrimPrefix(req.URL.Path, "/"))
	if path == "/" || strings.Contains(path, "..") {
		return nil, os.ErrNotExist
	}

	rel := strings.TrimPrefix(path, "/")
	if host == "cdn.animeav1.com" {
		rel = filepath.Join("_cdn", rel)
	} else if host != "animeav1.com" && host != "www.animeav1.com" {
		return nil, os.ErrNotExist
	}

	full := filepath.Join(root, rel)
	cleanRoot := filepath.Clean(root) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(full)+string(filepath.Separator), cleanRoot) {
		return nil, os.ErrNotExist
	}

	body, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}

	ctype := mime.TypeByExtension(filepath.Ext(full))
	if ctype == "" {
		ctype = "application/octet-stream"
	}

	return &http.Response{
		Status:        "200 OK (seed)",
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": []string{ctype}, "X-AnimeAV1-Seed": []string{"1"}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}, nil
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
	// No se añade Cookie ni ninguna credencial. Si el origen responde 403/404,
	// se intenta resolver el mismo recurso desde /data/seed.
	base := http.DefaultTransport
	http.DefaultTransport = &animeAV1RefererTransport{base: base, seedRoot: "/data/seed"}
}
