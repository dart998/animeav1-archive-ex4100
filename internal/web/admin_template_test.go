package web

import (
	"os"
	"strings"
	"testing"
)

func TestAdminTemplateKeepsAV1SyncErrorsInline(t *testing.T) {
	b, err := os.ReadFile("../../web/templates/admin.html")
	if err != nil { t.Fatal(err) }
	s := string(b)
	for _, needle := range []string{
		`id="sync-av1-form"`,
		`id="sync-av1-result"`,
		`e.preventDefault()`,
		`'Accept':'application/json'`,
		`Error al actualizar listas AV1:`,
	} {
		if !strings.Contains(s, needle) { t.Fatalf("template missing %q", needle) }
	}
}
