package web

import (
	"strings"
	"testing"
)

func TestPlayerActiveSourceUsesOwnVisualState(t *testing.T) {
	for _, needle := range []string{
		"classList.add('mirror-source-active')",
		"classList.remove('mirror-source-active')",
		".mirror-source-active{background:#3cecd6!important",
		"if(local&&local.browser_playable){showLocal();return}",
	} {
		if !strings.Contains(localEpisodeBridge, needle) { t.Fatalf("localEpisodeBridge missing %q", needle) }
	}
}
