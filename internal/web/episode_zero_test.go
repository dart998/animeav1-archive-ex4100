package web

import "testing"

func TestDownloadEpisodeRangeNormal(t *testing.T){
	got:=downloadEpisodeRange(12,12,false);if len(got)!=12||got[0]!=1||got[11]!=12{t.Fatalf("normal range = %v",got)}
}

func TestDownloadEpisodeRangeWithZero(t *testing.T){
	got:=downloadEpisodeRange(24,25,true);if len(got)!=25||got[0]!=0||got[24]!=24{t.Fatalf("zero-based range = %v",got)}
}
