package web

import (
	"strings"
	"testing"
)

func TestArchiveVersionBridgeAppearsBelowFansTagline(t *testing.T) {
	s := string(archiveVersionBridge("0.6.31"))
	for _, needle := range []string{
		"By fans for fans",
		"AnimeAV1 Archive v0.6.31",
		"id='mirror-app-version'",
		"insertAdjacentElement('afterend',v)",
	} {
		if !strings.Contains(s, needle) { t.Fatalf("version bridge missing %q", needle) }
	}
}
