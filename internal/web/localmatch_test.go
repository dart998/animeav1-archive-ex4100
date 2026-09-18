package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
)

func TestDownloadSeriesTotalUsesDiscoveredMaximum(t *testing.T) {
	if got := downloadSeriesTotal(11, 12); got != 12 {
		t.Fatalf("downloadSeriesTotal(11, 12) = %d, want 12", got)
	}
	if got := downloadSeriesTotal(12, 11); got != 12 {
		t.Fatalf("downloadSeriesTotal(12, 11) = %d, want 12", got)
	}
}

func TestSeasonNumberRecognizesSharedFolderPatterns(t *testing.T) {
	cases := map[string]int{
		"Unnamed Memory Act.2": 2,
		"Anime Season 2":       2,
		"Anime Temporada 3":    3,
		"Anime Part 2":         2,
		"S02E01":               2,
		"T1E12":                1,
	}
	for in, want := range cases {
		if got := seasonNumber(in); got != want {
			t.Errorf("seasonNumber(%q) = %d, want %d", in, got, want)
		}
	}
	item := animeav1.Item{Title: "Unnamed Memory Act.2", Aliases: map[string]string{}}
	if got := itemSeasonNumber(item); got != 2 {
		t.Fatalf("itemSeasonNumber() = %d, want 2", got)
	}
}

func TestScanLocalFolderSeasonsSplitsTNotation(t *testing.T) {
	root := t.TempDir()
	files := map[string]int{
		"T1E01.mp4": 101,
		"T1E02.mp4": 102,
		"T2E01.mp4": 201,
		"T2E02.mp4": 202,
	}
	for name, size := range files {
		if err := os.WriteFile(filepath.Join(root, name), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	summary := scanLocalFolderSeasons(root)
	if !summary.Seasonal {
		t.Fatal("expected seasonal layout")
	}
	if got := summary.Stats[1]; got.Files != 2 || got.Bytes != 203 {
		t.Fatalf("season 1 = %+v, want 2 files / 203 bytes", got)
	}
	if got := summary.Stats[2]; got.Files != 2 || got.Bytes != 403 {
		t.Fatalf("season 2 = %+v, want 2 files / 403 bytes", got)
	}
}

func TestEpisodeFileRankRecognizesTNotation(t *testing.T) {
	if got := episodeFileRank("T2E12.mp4", "", 12); got != 1 {
		t.Fatalf("episodeFileRank(T2E12, ep12) = %d, want 1", got)
	}
	if got := seasonRankForPath("/library/Unnamed Memory/T2E12.mp4", 2); got != 0 {
		t.Fatalf("seasonRankForPath(T2E12, season2) = %d, want 0", got)
	}
	if got := seasonRankForPath("/library/Unnamed Memory/T1E12.mp4", 2); got != 9 {
		t.Fatalf("seasonRankForPath(T1E12, season2) = %d, want 9", got)
	}
}
