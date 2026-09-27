package web

import (
	"strings"
	"testing"
)

func TestProviderSelectionUsesOriginalActiveStyle(t *testing.T) {
	for _, needle := range []string{
		"var subUPN=providerButtons('UPNShare')",
		"activeClass=subUPN?subUPN.className",
		"providerButtons('UPNShare').forEach(function(b){b.style.display='none'})",
		"b.classList.add('mirror-provider-active')",
		"x.button.classList.remove('mirror-provider-active')",
		".mirror-provider-active{background:#3cecd6!important",
		"if(local&&local.browser_playable){showLocal();return}",
	} {
		if !strings.Contains(localEpisodeBridge, needle) { t.Fatalf("localEpisodeBridge missing %q", needle) }
	}
}
