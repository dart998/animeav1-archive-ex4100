package web

import (
	"testing"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

func TestLocalFolderCandidatesGroupsSeasonsAndOVA(t *testing.T) {
	it := animeav1.Item{Title:"Re Zero kara Hajimeru Isekai Seikatsu", Aliases:map[string]string{"en":"Re:Zero kara Hajimeru Isekai Seikatsu"}}
	lib := []database.LibraryItem{
		{Name:"Re Zero kara Hajimeru Isekai Seikatsu", Path:"/library/s1"},
		{Name:"Re Zero kara Hajimeru Isekai Seikatsu 2nd Season", Path:"/library/s2"},
		{Name:"Re Zero kara Hajimeru Isekai Seikatsu Season 3", Path:"/library/s3"},
		{Name:"Re Zero kara Hajimeru Isekai Seikatsu OVA", Path:"/library/ova"},
		{Name:"Another Anime", Path:"/library/other"},
	}
	got := localFolderCandidates(it, lib)
	if len(got) != 4 { t.Fatalf("expected 4 related folders, got %d: %#v", len(got), got) }
	if got[0].Item.Path != "/library/s1" || got[0].Rank != 0 { t.Fatalf("exact folder must be first: %#v", got[0]) }
	for _, c := range got[1:] { if c.Rank != 1 { t.Fatalf("season/OVA folder should be related rank 1: %#v", c) } }
}

func TestEpisodeFileRankPrefersAnimeAV1MediaID(t *testing.T) {
	id := animeav1.IDString("4350")
	if r:=episodeFileRank("4350_1_SUB.mp4",id,1); r!=0 { t.Fatalf("mediaId pattern should rank 0, got %d",r) }
	if r:=episodeFileRank("Show.S01E01.mkv",id,1); r!=1 { t.Fatalf("explicit episode should rank 1, got %d",r) }
	if r:=episodeFileRank("Show - 01.mkv",id,1); r!=2 { t.Fatalf("standalone episode should rank 2, got %d",r) }
	if r:=episodeFileRank("9999_1_SUB.mp4",id,1); r==0 { t.Fatalf("foreign numeric prefix must not be treated as AnimeAV1 mediaId") }
}

func TestMirrorStatusPriority(t *testing.T) {
	want := map[int]int{0:0,1:1,2:2,3:3,4:3}
	for status, expected := range want { if got:=mirrorStatusPriority(status); got!=expected { t.Fatalf("status %d priority=%d want=%d",status,got,expected) } }
}
