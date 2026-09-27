package web

import (
	"strings"
	"testing"
)

func TestMobilePlayerUsesResponsiveCandidateAndCleanup(t *testing.T) {
	for _, needle := range []string{
		"r.width>=200&&r.height>=120",
		"area>bestArea",
		"document.querySelectorAll('iframe,video')",
		"watchDuplicatePlayers(shared)",
		"MutationObserver",
	} {
		if !strings.Contains(localEpisodeBridge, needle) { t.Fatalf("localEpisodeBridge missing %q", needle) }
	}
	if strings.Contains(localEpisodeBridge, "r.width>450&&r.height>180") { t.Fatal("desktop-only player threshold still present") }
}
