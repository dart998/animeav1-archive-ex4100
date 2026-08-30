package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/dart998/animeav1-archive-ex4100/internal/crawler"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
	libraryindex "github.com/dart998/animeav1-archive-ex4100/internal/library"
	sitemirror "github.com/dart998/animeav1-archive-ex4100/internal/site"
)

type Server struct { db *database.DB; crawl *crawler.Service; mirror *sitemirror.Mirror; tmpl *template.Template; static string; libraryRoot string; siteRoot string; version string; commitSHA string }
type adminData struct { Version string; CommitSHA string; CommitShort string; CommitURL string; State crawler.State; Mirror sitemirror.State; MALUsername string; AnimeAV1CookieConfigured bool; Library []database.LibraryItem; Watching []database.MALWatchingItem }

func New(db *database.DB,c *crawler.Service,mirror *sitemirror.Mirror,webDir,libraryRoot,siteRoot,version,commitSHA string)(*Server,error){
	fm:=template.FuncMap{"bytes":func(v int64)string{const gb=1024*1024*1024;const mb=1024*1024;if v>=gb{return fmt.Sprintf("%.1f GB",float64(v)/gb)};if v>=mb{return fmt.Sprintf("%.1f MB",float64(v)/mb)};return fmt.Sprintf("%d B",v)}}
	t,e:=template.New("root").Funcs(fm).ParseFiles(filepath.Join(webDir,"templates","admin.html"));if e!=nil{return nil,e}
	mirror.SetSessionCookie(db.GetSetting("animeav1_session_cookie"))
	return &Server{db:db,crawl:c,mirror:mirror,tmpl:t,static:filepath.Join(webDir,"static"),libraryRoot:libraryRoot,siteRoot:siteRoot,version:version,commitSHA:commitSHA},nil
}
func (s *Server) Handler()http.Handler{
	m:=http.NewServeMux()
	m.Handle("/admin-static/",http.StripPrefix("/admin-static/",http.FileServer(http.Dir(s.static))))
	m.HandleFunc("/healthz",s.health);m.HandleFunc("/api/status",s.status)
	m.HandleFunc("/admin/settings",s.settings);m.HandleFunc("/admin/rescan",s.rescan);m.HandleFunc("/admin/sync-mal",s.syncMAL);m.HandleFunc("/admin/mirror",s.startMirror);m.HandleFunc("/admin",s.admin)
	m.Handle("/",s.mirror.Handler())
	return m
}
func (s *Server) health(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"status":"ok"}`))}
func (s *Server) status(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(map[string]any{"version":s.version,"commit":s.commitSHA,"crawler":s.crawl.State.Snapshot(),"mirror":s.mirror.Snapshot(),"animeav1_session_configured":s.mirror.HasSessionCookie()})}
func (s *Server) admin(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/admin"{http.NotFound(w,r);return};items,e:=s.db.Library();if e!=nil{http.Error(w,e.Error(),500);return};watching,e:=s.db.MALWatching();if e!=nil{http.Error(w,e.Error(),500);return};short:=s.commitSHA;if len(short)>7{short=short[:7]};commitURL:="";if s.commitSHA!=""&&s.commitSHA!="unknown"{commitURL="https://github.com/dart998/animeav1-archive-ex4100/commit/"+s.commitSHA};d:=adminData{Version:s.version,CommitSHA:s.commitSHA,CommitShort:short,CommitURL:commitURL,State:s.crawl.State.Snapshot(),Mirror:s.mirror.Snapshot(),MALUsername:s.db.GetSetting("mal_username"),AnimeAV1CookieConfigured:s.mirror.HasSessionCookie(),Library:items,Watching:watching};if e=s.tmpl.ExecuteTemplate(w,"admin.html",d);e!=nil{http.Error(w,e.Error(),500)}}
func (s *Server) settings(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return}
	if e:=r.ParseForm();e!=nil{http.Error(w,e.Error(),400);return}
	if v,ok:=r.Form["mal_username"];ok{if e:=s.db.SetSetting("mal_username",strings.TrimSpace(v[0]));e!=nil{http.Error(w,e.Error(),500);return};s.crawl.RefreshConfigState()}
	if r.FormValue("clear_animeav1_cookie")=="1"{
		if e:=s.db.SetSetting("animeav1_session_cookie","");e!=nil{http.Error(w,e.Error(),500);return}
		s.mirror.SetSessionCookie("")
	}else if cookie:=strings.TrimSpace(r.FormValue("animeav1_session_cookie"));cookie!=""{
		if e:=s.db.SetSetting("animeav1_session_cookie",cookie);e!=nil{http.Error(w,e.Error(),500);return}
		s.mirror.SetSessionCookie(cookie)
	}
	http.Redirect(w,r,"/admin",http.StatusSeeOther)
}
func (s *Server) rescan(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};items,e:=libraryindex.Scan(s.libraryRoot);if e!=nil{http.Error(w,e.Error(),500);return};if e=s.db.ReplaceLibrary(items);e!=nil{http.Error(w,e.Error(),500);return};http.Redirect(w,r,"/admin",http.StatusSeeOther)}
func (s *Server) syncMAL(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};_ = s.crawl.RunAll(context.Background());http.Redirect(w,r,"/admin",http.StatusSeeOther)}
func (s *Server) startMirror(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};s.mirror.Start(context.Background());http.Redirect(w,r,"/admin",http.StatusSeeOther)}
