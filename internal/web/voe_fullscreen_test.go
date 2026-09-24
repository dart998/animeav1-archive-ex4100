package web

import (
	"strings"
	"testing"
)

func TestVoeFullscreenOverlayUsesParentFullscreen(t *testing.T){
	for _,needle:=range []string{
		"mirror-remote-fullscreen-voe",
		"bottom:8px",
		"toggleRemoteFullscreen(shared,f)",
		"document.exitFullscreen",
		"(binding.player.server||'').toLowerCase()==='voe'",
	}{
		if !strings.Contains(localEpisodeBridge,needle){t.Fatalf("localEpisodeBridge missing %q",needle)}
	}
}
