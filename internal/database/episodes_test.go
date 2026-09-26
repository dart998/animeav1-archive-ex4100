package database

import (
	"path/filepath"
	"testing"
)

func TestSeriesMaxEpisodeSupportsEpisodeZero(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "archive.sqlite"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	id, err := db.UpsertAnime("zero-series", "Zero Series", "https://example/media/zero-series")
	if err != nil { t.Fatal(err) }
	if got := db.SeriesMaxEpisode("zero-series"); got != -1 { t.Fatalf("empty max=%d want -1", got) }
	for _, ep := range []int{0,1,2,24} {
		if _, err = db.UpsertEpisode(id, ep, "", ""); err != nil { t.Fatal(err) }
	}
	if got := db.SeriesEpisodeCount("zero-series"); got != 4 { t.Fatalf("count=%d want 4", got) }
	if got := db.SeriesMaxEpisode("zero-series"); got != 24 { t.Fatalf("max=%d want 24", got) }
}
