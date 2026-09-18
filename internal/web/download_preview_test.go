package web

import "testing"

func TestBrowserPlayablePath(t *testing.T){
	cases:=map[string]bool{"a.mp4":true,"a.webm":true,"a.m4v":true,"a.mov":true,"a.mkv":false,"a.avi":false}
	for name,want:=range cases{if got:=browserPlayablePath(name);got!=want{t.Errorf("browserPlayablePath(%q)=%v, want %v",name,got,want)}}
}

func TestPreviewStateStartsPending(t *testing.T){
	st:=seriesDownloadState{Slug:"serie",Preview:true,Total:2,Episodes:[]downloadEpisodeState{{Episode:1,Status:"existing"},{Episode:2,Status:"pending"}},Skipped:1}
	if !st.Preview||st.Skipped!=1||st.Episodes[1].Status!="pending"{t.Fatalf("unexpected preview state: %+v",st)}
}
