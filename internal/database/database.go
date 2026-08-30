package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

type Source struct { Provider, URL, Status, Error string; Priority int }

type LibraryItem struct {
	Name string
	Path string
	Files int
	Bytes int64
	LastScan string
}

type MALWatchingItem struct {
	MALID int
	Title string
	Watched int
	Total int
	Status int
	LocalName string
	LocalPath string
	MatchScore int
	LastSeen string
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return nil, err }
	db, err := sql.Open("sqlite", path); if err != nil { return nil, err }
	d := &DB{db}; if err = d.Migrate(); err != nil { db.Close(); return nil, err }; return d, nil
}

func (d *DB) Migrate() error {
	_, err := d.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS anime (id INTEGER PRIMARY KEY, slug TEXT UNIQUE NOT NULL, title TEXT NOT NULL, url TEXT NOT NULL, last_seen_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS episodes (id INTEGER PRIMARY KEY, anime_id INTEGER NOT NULL REFERENCES anime(id) ON DELETE CASCADE, number INTEGER NOT NULL, title TEXT, url TEXT NOT NULL, selected_provider TEXT, selected_url TEXT, status TEXT NOT NULL DEFAULT 'discovered', last_seen_at TEXT NOT NULL, UNIQUE(anime_id, number));
CREATE TABLE IF NOT EXISTS video_sources (id INTEGER PRIMARY KEY, episode_id INTEGER NOT NULL REFERENCES episodes(id) ON DELETE CASCADE, provider TEXT NOT NULL, source_url TEXT NOT NULL DEFAULT '', priority INTEGER NOT NULL, detected_at TEXT NOT NULL, status TEXT NOT NULL, error TEXT NOT NULL DEFAULT '', UNIQUE(episode_id, provider, source_url));
CREATE TABLE IF NOT EXISTS downloads (id INTEGER PRIMARY KEY, episode_id INTEGER NOT NULL REFERENCES episodes(id) ON DELETE CASCADE, provider TEXT NOT NULL, path TEXT NOT NULL, sha256 TEXT NOT NULL, bytes INTEGER NOT NULL, created_at TEXT NOT NULL, UNIQUE(sha256));
CREATE TABLE IF NOT EXISTS crawl_runs (id INTEGER PRIMARY KEY, started_at TEXT NOT NULL, finished_at TEXT, status TEXT NOT NULL, target TEXT, error TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS library_items (name TEXT PRIMARY KEY, path TEXT NOT NULL, files INTEGER NOT NULL DEFAULT 0, bytes INTEGER NOT NULL DEFAULT 0, last_scan TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS mal_watching (
    mal_id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    watched INTEGER NOT NULL DEFAULT 0,
    total INTEGER NOT NULL DEFAULT 0,
    status INTEGER NOT NULL DEFAULT 1,
    local_name TEXT NOT NULL DEFAULT '',
    local_path TEXT NOT NULL DEFAULT '',
    match_score INTEGER NOT NULL DEFAULT 0,
    last_seen TEXT NOT NULL
);
`)
	return err
}

func (d *DB) BeginRun(target string) (int64, error) { r, e := d.Exec(`INSERT INTO crawl_runs(started_at,status,target) VALUES(?,?,?)`, time.Now().Format(time.RFC3339), "running", target); if e != nil { return 0,e }; return r.LastInsertId() }
func (d *DB) FinishRun(id int64, status, msg string) { _, _ = d.Exec(`UPDATE crawl_runs SET finished_at=?,status=?,error=? WHERE id=?`, time.Now().Format(time.RFC3339), status, msg, id) }
func (d *DB) UpsertAnime(slug,title,url string) (int64,error) { now:=time.Now().Format(time.RFC3339); _,e:=d.Exec(`INSERT INTO anime(slug,title,url,last_seen_at) VALUES(?,?,?,?) ON CONFLICT(slug) DO UPDATE SET title=excluded.title,url=excluded.url,last_seen_at=excluded.last_seen_at`,slug,title,url,now); if e!=nil{return 0,e}; var id int64; e=d.QueryRow(`SELECT id FROM anime WHERE slug=?`,slug).Scan(&id); return id,e }
func (d *DB) UpsertEpisode(animeID int64, number int, title,url string) (int64,error) { now:=time.Now().Format(time.RFC3339); _,e:=d.Exec(`INSERT INTO episodes(anime_id,number,title,url,last_seen_at) VALUES(?,?,?,?,?) ON CONFLICT(anime_id,number) DO UPDATE SET title=excluded.title,url=excluded.url,last_seen_at=excluded.last_seen_at`,animeID,number,title,url,now); if e!=nil{return 0,e}; var id int64; e=d.QueryRow(`SELECT id FROM episodes WHERE anime_id=? AND number=?`,animeID,number).Scan(&id); return id,e }
func (d *DB) SaveSources(episodeID int64, src []Source) error { tx,e:=d.Begin(); if e!=nil{return e}; defer tx.Rollback(); if _,e=tx.Exec(`DELETE FROM video_sources WHERE episode_id=?`,episodeID); e!=nil{return e}; for _,s:=range src { if _,e=tx.Exec(`INSERT INTO video_sources(episode_id,provider,source_url,priority,detected_at,status,error) VALUES(?,?,?,?,?,?,?)`,episodeID,s.Provider,s.URL,s.Priority,time.Now().Format(time.RFC3339),s.Status,s.Error);e!=nil{return e} }; return tx.Commit() }
func (d *DB) SelectSource(episodeID int64, provider,url,status string) { _,_=d.Exec(`UPDATE episodes SET selected_provider=?,selected_url=?,status=? WHERE id=?`,provider,url,status,episodeID) }
func (d *DB) RecordDownload(episodeID int64, provider,path,sha string, bytes int64) { _,_=d.Exec(`INSERT OR IGNORE INTO downloads(episode_id,provider,path,sha256,bytes,created_at) VALUES(?,?,?,?,?,?)`,episodeID,provider,path,sha,bytes,time.Now().Format(time.RFC3339)) }
func (d *DB) SetSetting(key,value string) error { _,e:=d.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,key,value);return e }
func (d *DB) GetSetting(key string) string { var v string; _=d.QueryRow(`SELECT value FROM settings WHERE key=?`,key).Scan(&v); return v }
func (d *DB) ReplaceLibrary(items []LibraryItem) error { tx,e:=d.Begin();if e!=nil{return e};defer tx.Rollback();if _,e=tx.Exec(`DELETE FROM library_items`);e!=nil{return e};for _,x:=range items{if _,e=tx.Exec(`INSERT INTO library_items(name,path,files,bytes,last_scan) VALUES(?,?,?,?,?)`,x.Name,x.Path,x.Files,x.Bytes,x.LastScan);e!=nil{return e}};return tx.Commit() }
func (d *DB) Library() ([]LibraryItem,error){rows,e:=d.Query(`SELECT name,path,files,bytes,last_scan FROM library_items ORDER BY name COLLATE NOCASE`);if e!=nil{return nil,e};defer rows.Close();var out []LibraryItem;for rows.Next(){var x LibraryItem;if e=rows.Scan(&x.Name,&x.Path,&x.Files,&x.Bytes,&x.LastScan);e!=nil{return nil,e};out=append(out,x)};return out,rows.Err()}
func (d *DB) ReplaceMALWatching(items []MALWatchingItem) error { tx,e:=d.Begin();if e!=nil{return e};defer tx.Rollback();if _,e=tx.Exec(`DELETE FROM mal_watching`);e!=nil{return e};for _,x:=range items{if _,e=tx.Exec(`INSERT INTO mal_watching(mal_id,title,watched,total,status,local_name,local_path,match_score,last_seen) VALUES(?,?,?,?,?,?,?,?,?)`,x.MALID,x.Title,x.Watched,x.Total,x.Status,x.LocalName,x.LocalPath,x.MatchScore,x.LastSeen);e!=nil{return e}};return tx.Commit() }
func (d *DB) MALWatching() ([]MALWatchingItem,error){rows,e:=d.Query(`SELECT mal_id,title,watched,total,status,local_name,local_path,match_score,last_seen FROM mal_watching ORDER BY title COLLATE NOCASE`);if e!=nil{return nil,e};defer rows.Close();var out []MALWatchingItem;for rows.Next(){var x MALWatchingItem;if e=rows.Scan(&x.MALID,&x.Title,&x.Watched,&x.Total,&x.Status,&x.LocalName,&x.LocalPath,&x.MatchScore,&x.LastSeen);e!=nil{return nil,e};out=append(out,x)};return out,rows.Err()}
