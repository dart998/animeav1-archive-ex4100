package web

import (
	"strings"
	"testing"
)

func TestArchiveVersionBridgePreservesOriginalFansBlock(t *testing.T) {
	s := string(archiveVersionBridge("0.6.33"))
	for _, needle := range []string{
		"By fans for fans",
		"AnimeAV1 Archive v0.6.33",
		"t.appendChild(v)",
		"data-mirror-fans-tagline",
		"#mirror-app-version{display:block!important",
	} {
		if !strings.Contains(s, needle) { t.Fatalf("version bridge missing %q", needle) }
	}
	for _, forbidden := range []string{
		"mirror-fans-stack",
		"parent.insertBefore(stack,t)",
		"stack.appendChild(t)",
	} {
		if strings.Contains(s, forbidden) { t.Fatalf("version bridge still restructures original footer: %q", forbidden) }
	}
}
