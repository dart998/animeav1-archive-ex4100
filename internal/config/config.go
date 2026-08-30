package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL            string
	LibraryURL         string
	DBPath             string
	DataDir            string
	MetadataDir        string
	VideoDir           string
	CrawlerEnabled     bool
	CrawlerInterval    time.Duration
	CrawlerBatchSize   int
	ProviderOrder      []string
	ProviderFallback   bool
	DownloadVideos     bool
	DownloadAuthorized bool
	Port               string
}

func Load() Config {
	return Config{
		BaseURL:            env("ANIMEAV1_BASE_URL", "https://animeav1.com"),
		LibraryURL:         env("ANIMEAV1_LIBRARY_URL", "https://animeav1.com/cuenta/listas"),
		DBPath:             env("ARCHIVE_DB", "/data/db/archive.sqlite"),
		DataDir:            env("ARCHIVE_DATA_DIR", "/data"),
		MetadataDir:        env("ARCHIVE_METADATA_DIR", "/data/metadata"),
		VideoDir:           env("ARCHIVE_VIDEO_DIR", "/data/videos"),
		CrawlerEnabled:     boolEnv("CRAWLER_ENABLED", true),
		CrawlerInterval:    durationEnv("CRAWLER_INTERVAL", 30*time.Minute),
		CrawlerBatchSize:   intEnv("CRAWLER_BATCH_SIZE", 5),
		ProviderOrder:      csvEnv("PROVIDER_ORDER", "hls,upnshare,mega,mp4upload"),
		ProviderFallback:   boolEnv("PROVIDER_FALLBACK", true),
		DownloadVideos:     boolEnv("DOWNLOAD_VIDEOS", false),
		DownloadAuthorized: boolEnv("VIDEO_DOWNLOAD_AUTHORIZED", false),
		Port:               env("PORT", "8080"),
	}
}

func env(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }
func boolEnv(k string, def bool) bool { v := os.Getenv(k); if v == "" { return def }; b, err := strconv.ParseBool(v); if err != nil { return def }; return b }
func intEnv(k string, def int) int { v := os.Getenv(k); if v == "" { return def }; n, err := strconv.Atoi(v); if err != nil || n < 1 { return def }; return n }
func durationEnv(k string, def time.Duration) time.Duration { v := os.Getenv(k); if v == "" { return def }; d, err := time.ParseDuration(v); if err != nil { return def }; return d }
func csvEnv(k, def string) []string { v := env(k, def); out := []string{}; for _, s := range strings.Split(v, ",") { if s = strings.TrimSpace(strings.ToLower(s)); s != "" { out = append(out, s) } }; return out }
