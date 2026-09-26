package crawler

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

func TestEpisodeNotificationsUsePreviousMax(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "archive.sqlite"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	s := &Service{db: db}
	item := animeav1.Item{Slug: "serie", Title: "Serie"}
	s.addEpisodeNotifications(item, []int{0,1,2,24,25}, 24)
	var ns []episodeNotification
	if err = json.Unmarshal([]byte(db.GetSetting("episode_notifications_json")), &ns); err != nil { t.Fatal(err) }
	if len(ns) != 1 || ns[0].Episode != 25 { t.Fatalf("notifications=%+v want only episode 25", ns) }
}

func TestEpisodeZeroDoesNotHideEpisodeOneNotification(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "archive.sqlite"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	s := &Service{db: db}
	s.addEpisodeNotifications(animeav1.Item{Slug:"zero", Title:"Zero"}, []int{0,1}, 0)
	var ns []episodeNotification
	if err = json.Unmarshal([]byte(db.GetSetting("episode_notifications_json")), &ns); err != nil { t.Fatal(err) }
	if len(ns) != 1 || ns[0].Episode != 1 { t.Fatalf("notifications=%+v want episode 1", ns) }
}
