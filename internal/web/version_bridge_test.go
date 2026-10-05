package web

import (
	"strings"
	"testing"
)

func TestArchiveVersionBridgePreservesOriginalFansBlock(t *testing.T) {
	s := string(archiveVersionBridge("0.6.35"))
	for _, needle := range []string{
		"By fans for fans",
		"v0.6.35",
		"t.appendChild(v)",
		"data-mirror-fans-tagline",
		"position:relative!important",
		"position:absolute!important",
		"top:calc(100% + 4px)!important",
	} {
		if !strings.Contains(s, needle) { t.Fatalf("version bridge missing %q", needle) }
	}
	if strings.Contains(s, "AnimeAV1 Archive v0.6.35") { t.Fatal("footer must show only the short version label") }
	for _, forbidden := range []string{
		"mirror-fans-stack",
		"parent.insertBefore(stack,t)",
		"stack.appendChild(t)",
		"flex-direction:column!important",
	} {
		if strings.Contains(s, forbidden) { t.Fatalf("version bridge still misaligns or restructures original footer: %q", forbidden) }
	}
}
