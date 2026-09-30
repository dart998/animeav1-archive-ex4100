package web

import (
	"strings"
	"testing"
)

func TestArchiveVersionBridgeAppearsBelowFansTagline(t *testing.T) {
	s := string(archiveVersionBridge("0.6.32"))
	for _, needle := range []string{
		"By fans for fans",
		"AnimeAV1 Archive v0.6.32",
		"id='mirror-fans-stack'",
		"stack.appendChild(t);stack.appendChild(v)",
		"flex-direction:column!important",
		"white-space:nowrap!important",
		"font-size:inherit!important",
	} {
		if !strings.Contains(s, needle) { t.Fatalf("version bridge missing %q", needle) }
	}
}
