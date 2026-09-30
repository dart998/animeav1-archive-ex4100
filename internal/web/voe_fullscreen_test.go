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
		"if(isVoe){var fs=document.createElement('button')",
		"f.allow='autoplay; encrypted-media; fullscreen; picture-in-picture'",
		"f.allowFullscreen=true",
		"f.setAttribute('allowfullscreen','')",
	}{
		if !strings.Contains(localEpisodeBridge,needle){t.Fatalf("localEpisodeBridge missing %q",needle)}
	}
	if strings.Contains(localEpisodeBridge,"isVoe?'mirror-remote-fullscreen mirror-remote-fullscreen-voe':'mirror-remote-fullscreen'"){
		t.Fatal("non-Voe providers still receive mirror fullscreen button")
	}
}
