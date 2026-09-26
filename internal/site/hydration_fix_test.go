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


func TestNotificationUIUsesBellAndSameDropdown(t *testing.T) {
	for _, needle := range []string{
		"button:has(svg.lucide-bell)",
		".mirror-bell-unread svg{animation:mirror-bell-ring",
		"function panel(){var readAll=",
		"if(/Notificaciones/i.test(t))",
		"var items=data.notifications||[]",
		"e.style.display=items.length?'none':''",
		"notifPost('read_all').then(loadNotifications)",
	} {
		if !strings.Contains(hydrationFixUI, needle) { t.Fatalf("hydrationFixUI missing %q", needle) }
	}
	if strings.Contains(hydrationFixUI, "stopImmediatePropagation();notifPost('read_all')") {
		t.Fatal("Leer todo must not suppress the original AnimeAV1 handler")
	}
}
