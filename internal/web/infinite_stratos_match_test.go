package web

import (
	"testing"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

func TestExactFolderWinsOverFranchiseStem(t *testing.T) {
	it := animeav1.Item{Title: "IS: Infinite Stratos 2"}
	lib := []database.LibraryItem{
		{Name: "IS - Infinite Stratos", Files: 12},
		{Name: "IS - Infinite Stratos 2", Files: 1},
	}
	got := localFolderCandidates(it, lib)
	if len(got) != 1 { t.Fatalf("candidates=%+v want exactly one", got) }
	if got[0].Item.Name != "IS - Infinite Stratos 2" { t.Fatalf("selected %q", got[0].Item.Name) }
	if got[0].Rank != 0 { t.Fatalf("rank=%d want 0", got[0].Rank) }
}

func TestHistoricalMediaIDFallbackStillWorks(t *testing.T) {
	if rank := episodeFileRank("131_3_RhWz.mp4", animeav1.IDString("9999"), 3); rank == 99 {
		t.Fatal("historical media-id filename should remain accepted inside the correct folder")
	}
}
