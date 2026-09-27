package web

import (
	"strings"
	"testing"
)

func TestLocalPlayerButtonExistsWithoutHLS(t *testing.T) {
	for _, needle := range []string{
		"if(local&&local.available){if(subHLS){localBtn=subHLS}else if(ref){localBtn=document.createElement('button')",
		"ref.parentNode.insertBefore(localBtn,ref)",
		"localBtn.setAttribute('data-mirror-local','1')",
		"localBtn.setAttribute('title',local.file_name||'Archivo local')",
		"activate(localBtn)",
		"if(local&&local.browser_playable){showLocal();return}",
	} {
		if !strings.Contains(localEpisodeBridge, needle) { t.Fatalf("localEpisodeBridge missing %q", needle) }
	}
}
