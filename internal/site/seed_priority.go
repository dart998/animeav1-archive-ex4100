package site

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

var seedStaticExt = map[string]bool{
	".css": true, ".js": true, ".mjs": true, ".svg": true,
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".avif": true, ".gif": true, ".ico": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
}

// promoteSeedStatics copia únicamente recursos estáticos conocidos desde el
// seed sobre el cache del mirror. El HTML/JSON nunca se promueve: la fuente de
// verdad de las fichas sigue siendo AnimeAV1.
func promoteSeedStatics(seedRoot, siteRoot string) (int, error) {
	seedRoot = filepath.Clean(seedRoot)
	siteRoot = filepath.Clean(siteRoot)
	count := 0
	err := filepath.Walk(seedRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		if info == nil || info.IsDir() { return nil }
		if !seedStaticExt[strings.ToLower(filepath.Ext(info.Name()))] { return nil }
		rel, err := filepath.Rel(seedRoot, path); if err != nil { return err }
		dst := filepath.Join(siteRoot, rel)
		if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { return err }
		src, err := os.Open(path); if err != nil { return err }; defer src.Close()
		out, err := os.Create(dst); if err != nil { return err }
		_, cpErr := io.Copy(out, src); closeErr := out.Close()
		if cpErr != nil { return cpErr }; if closeErr != nil { return closeErr }
		if err = os.Chmod(dst, info.Mode().Perm()); err != nil { return err }
		count++
		return nil
	})
	if os.IsNotExist(err) { return 0, nil }
	return count, err
}

func init() {
	// En el contenedor oficial /data/site y /data/seed comparten /data.
	// Esto hace efectiva la prioridad seed > cache > origen desde el arranque.
	_, _ = promoteSeedStatics("/data/seed", "/data/site")
}
