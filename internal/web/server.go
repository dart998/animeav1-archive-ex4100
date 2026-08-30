package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/dart998/animeav1-archive-ex4100/internal/crawler"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
	libraryindex "github.com/dart998/animeav1-archive-ex4100/internal/library"
)

type Server struct { db *database.DB; crawl *crawler.Service; tmpl *template.Template; static string; libraryRoot string }
type animeRow struct { Slug,Title,Status string; Episodes,Sources int }
type pageData struct { State crawler.State; Anime []animeRow }
type adminData struct { State crawler.State; MALUsername string; Library []database.LibraryItem }

func New(db *database.DB,c *crawler.Service,webDir,libraryRoot string)(*Server,error){
	fm:=template.FuncMap{"bytes":func(v int64)string{const gb=1024*1024*1024;const mb=1024*1024;if v>=gb{return fmt.Sprintf("%.1f GB",float64(v)/gb)};if v>=mb{return fmt.Sprintf("%.1f MB",float64(v)/mb)};return fmt.Sprintf("%d B",v)}}
	t,e:=template.New("root").Funcs(fm).ParseFiles(filepath.Join(webDir,"templates","index.html"),filepath.Join(webDir,"templates","admin.html"));if e!=nil{return nil,e}
	return &Server{db:db,crawl:c,tmpl:t,static:filepath.Join(webDir,"static"),libraryRoot:libraryRoot},nil
}
func (s *Server) Handler()http.Handler{m:=http.NewServeMux();m.Handle("/static/",http.StripPrefix("/static/",http.FileServer(http.Dir(s.static))));m.HandleFunc("/healthz",s.health);m.HandleFunc("/api/status",s.status);m.HandleFunc("/admin/settings",s.settings);m.HandleFunc("/admin/rescan",s.rescan);m.HandleFunc("/admin",s.admin);m.HandleFunc("/",s.index);return m}
func (s *Server) health(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"status":"ok"}`))}
func (s *Server) status(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(s.crawl.State.Snapshot())}
func (s *Server) index(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/"{http.NotFound(w,r);return};rows,e:=s.db.Query(`SELECT a.slug,a.title,COUNT(DISTINCT e.id),COUNT(DISTINCT vs.id),COALESCE(MAX(e.status),'') FROM anime a LEFT JOIN episodes e ON e.anime_id=a.id LEFT JOIN video_sources vs ON vs.episode_id=e.id GROUP BY a.id ORDER BY a.title`);if e!=nil{http.Error(w,e.Error(),500);return};defer rows.Close();d:=pageData{State:s.crawl.State.Snapshot()};for rows.Next(){var x animeRow;_ = rows.Scan(&x.Slug,&x.Title,&x.Episodes,&x.Sources,&x.Status);d.Anime=append(d.Anime,x)};if e=s.tmpl.ExecuteTemplate(w,"index.html",d);e!=nil{http.Error(w,e.Error(),500)}}
func (s *Server) admin(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/admin"{http.NotFound(w,r);return};items,e:=s.db.Library();if e!=nil{http.Error(w,e.Error(),500);return};d:=adminData{State:s.crawl.State.Snapshot(),MALUsername:s.db.GetSetting("mal_username"),Library:items};if e=s.tmpl.ExecuteTemplate(w,"admin.html",d);e!=nil{http.Error(w,e.Error(),500)}}
func (s *Server) settings(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};if e:=r.ParseForm();e!=nil{http.Error(w,e.Error(),400);return};if e:=s.db.SetSetting("mal_username",r.FormValue("mal_username"));e!=nil{http.Error(w,e.Error(),500);return};s.crawl.RefreshConfigState();http.Redirect(w,r,"/admin",http.StatusSeeOther)}
func (s *Server) rescan(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};items,e:=libraryindex.Scan(s.libraryRoot);if e!=nil{http.Error(w,e.Error(),500);return};if e=s.db.ReplaceLibrary(items);e!=nil{http.Error(w,e.Error(),500);return};http.Redirect(w,r,"/admin",http.StatusSeeOther)}
