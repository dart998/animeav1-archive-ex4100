package web

import (
	"strings"
	"testing"
)

func TestArchiveVersionBridgePreservesOriginalFansBlock(t *testing.T) {
	s := string(archiveVersionBridge("0.6.34"))
	for _, needle := range []string{
		"By fans for fans",
		"v0.6.34",
		"t.appendChild(v)",
		"data-mirror-fans-tagline",
		"flex-direction:column!important",
		"#mirror-app-version{display:block!important",
	} {
		if !strings.Contains(s, needle) { t.Fatalf("version bridge missing %q", needle) }
	}
	if strings.Contains(s, "AnimeAV1 Archive v0.6.34") { t.Fatal("footer must show only the short version label") }
	for _, forbidden := range []string{
		"mirror-fans-stack",
		"parent.insertBefore(stack,t)",
		"stack.appendChild(t)",
	} {
		if strings.Contains(s, forbidden) { t.Fatalf("version bridge still restructures original footer: %q", forbidden) }
	}
}
