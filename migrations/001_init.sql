-- The application applies this schema programmatically on startup.
-- This file documents the initial SQLite model.
CREATE TABLE anime(id INTEGER PRIMARY KEY, slug TEXT UNIQUE NOT NULL, title TEXT NOT NULL, url TEXT NOT NULL, last_seen_at TEXT NOT NULL);
CREATE TABLE episodes(id INTEGER PRIMARY KEY, anime_id INTEGER NOT NULL, number INTEGER NOT NULL, title TEXT, url TEXT NOT NULL, selected_provider TEXT, selected_url TEXT, status TEXT NOT NULL DEFAULT 'discovered', last_seen_at TEXT NOT NULL, UNIQUE(anime_id,number));
CREATE TABLE video_sources(id INTEGER PRIMARY KEY, episode_id INTEGER NOT NULL, provider TEXT NOT NULL, source_url TEXT NOT NULL DEFAULT '', priority INTEGER NOT NULL, detected_at TEXT NOT NULL, status TEXT NOT NULL, error TEXT NOT NULL DEFAULT '');
CREATE TABLE downloads(id INTEGER PRIMARY KEY, episode_id INTEGER NOT NULL, provider TEXT NOT NULL, path TEXT NOT NULL, sha256 TEXT NOT NULL UNIQUE, bytes INTEGER NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE crawl_runs(id INTEGER PRIMARY KEY, started_at TEXT NOT NULL, finished_at TEXT, status TEXT NOT NULL, target TEXT, error TEXT NOT NULL DEFAULT '');
