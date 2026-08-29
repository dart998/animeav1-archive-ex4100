package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/dart998/animeav1-archive-ex4100/internal/crawler"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

type Server struct { db *database.DB; crawl *crawler.Service; tmpl *template.Template; static string }
type animeRow struct { Slug,Title,Status string; Episodes,Sources int }
type pageData struct { State crawler.State; Anime []animeRow }
func New(db *database.DB,c *crawler.Service,webDir string)(*Server,error){t,e:=template.ParseFiles(filepath.Join(webDir,"templates","index.html"));if e!=nil{return nil,e};return &Server{db:db,crawl:c,tmpl:t,static:filepath.Join(webDir,"static")},nil}
func (s *Server) Handler()http.Handler{m:=http.NewServeMux();m.Handle("/static/",http.StripPrefix("/static/",http.FileServer(http.Dir(s.static))));m.HandleFunc("/healthz",s.health);m.HandleFunc("/api/status",s.status);m.HandleFunc("/",s.index);return m}
func (s *Server) health(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"status":"ok"}`))}
func (s *Server) status(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(s.crawl.State.Snapshot())}
func (s *Server) index(w http.ResponseWriter,r *http.Request){rows,e:=s.db.Query(`SELECT a.slug,a.title,COUNT(DISTINCT e.id),COUNT(DISTINCT vs.id),COALESCE(MAX(e.status),'') FROM anime a LEFT JOIN episodes e ON e.anime_id=a.id LEFT JOIN video_sources vs ON vs.episode_id=e.id GROUP BY a.id ORDER BY a.title`);if e!=nil{http.Error(w,e.Error(),500);return};defer rows.Close();d:=pageData{State:s.crawl.State.Snapshot()};for rows.Next(){var x animeRow;_ = rows.Scan(&x.Slug,&x.Title,&x.Episodes,&x.Sources,&x.Status);d.Anime=append(d.Anime,x)};if e=s.tmpl.ExecuteTemplate(w,"index.html",d);e!=nil{http.Error(w,e.Error(),500)}}
