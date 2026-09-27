package site

import (
	"strings"
	"testing"
)

func TestAdminNavStaysOutOfMobileNavigation(t *testing.T) {
	for _, needle := range []string{
		"if(window.innerWidth<768)",
		"existing.forEach(function(a){a.remove()})",
		"/Cat[aá]logo de Animes/i.test(t)",
		"p.insertBefore(a,horario)",
	} {
		if !strings.Contains(hydrationFixUI, needle) { t.Fatalf("hydrationFixUI missing %q", needle) }
	}
}
