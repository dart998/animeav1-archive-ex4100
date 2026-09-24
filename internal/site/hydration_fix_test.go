package site

import (
	"strings"
	"testing"
)

func TestHydrationFixAdminNavAndActionCleanup(t *testing.T){
	for _,needle:=range []string{
		"function ensureAdminNav()",
		"a.textContent='Admin'",
		"a.href='/admin'",
		"p.insertBefore(a,h)",
		"ensureAdminNav();ensureActions();prunePageActions()",
		"removeMatchingAction(/^(?:compartir|share)$/i)",
		"reportar(?:\\s+episodio)?",
		"if(el.id&&/^mirror-/.test(el.id))return",
	}{
		if !strings.Contains(hydrationFixUI,needle){t.Fatalf("hydrationFixUI missing %q",needle)}
	}
}
