package web

import (
	"strings"
	"testing"
)

func TestLocalPlayerHasJWStyleControls(t *testing.T) {
	s := string(archiveVersionBridge("0.6.35"))
	for _, needle := range []string{
		"mirror-jw-player",
		"Retroceder 10 segundos",
		"Avanzar 10 segundos",
		"mirror-jw-progress",
		"mirror-jw-volume",
		"0.5",
		"1.25",
		"1.5",
		"Imagen en imagen",
		"Pantalla completa",
		"v.controls=false",
		"v.playbackRate=speed",
		"requestPictureInPicture",
		"requestFullscreen",
		"hideTimer=setTimeout(function(){ui.classList.remove('mirror-jw-visible');menu.classList.remove('open')},2600)",
	} {
		if !strings.Contains(s, needle) { t.Fatalf("player bridge missing %q", needle) }
	}
}

func TestFooterVersionDoesNotMoveHeartAlignment(t *testing.T) {
	s := string(archiveVersionBridge("0.6.35"))
	for _, needle := range []string{
		"v0.6.35",
		"position:relative!important",
		"display:inline-block!important",
		"position:absolute!important",
		"top:calc(100% + 4px)!important",
	} {
		if !strings.Contains(s, needle) { t.Fatalf("footer bridge missing %q", needle) }
	}
	if strings.Contains(s, "flex-direction:column!important") {
		t.Fatal("tagline must not become a two-line flex box because it misaligns the heart")
	}
}

func TestLocalPlayerControlsSurviveLocalRecreation(t *testing.T) {
	s := string(archiveVersionBridge("0.6.35"))
	if !strings.Contains(s, "new MutationObserver(function(){putVersion();enhanceLocal()})") {
		t.Fatal("local player controls must be re-applied after switching providers")
	}
	if !strings.Contains(s, "if(!v||v.dataset.mirrorJW==='1')return") {
		t.Fatal("local player controls must not be duplicated")
	}
}
