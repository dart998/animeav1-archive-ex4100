package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	"github.com/dart998/animeav1-archive-ex4100/internal/crawler"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
	libraryindex "github.com/dart998/animeav1-archive-ex4100/internal/library"
	sitemirror "github.com/dart998/animeav1-archive-ex4100/internal/site"
)

type Server struct {
	db *database.DB
	crawl *crawler.Service
	mirror *sitemirror.Mirror
	av1 *animeav1.Client
	tmpl *template.Template
	static string
	libraryRoot string
	siteRoot string
	version string
	commitSHA string
}

type avSeries struct {
	MediaID string
	Title string
	Slug string
	URL string
	Status string
	StatusOrder int
	Seen int
	Total int
	LocalName string
	LocalFiles int
	LocalBytes int64
	MatchType string
	RenameSuggestion string
	Managed bool
	Discovered int
}

type adminData struct {
	Version string
	CommitSHA string
	CommitShort string
	CommitURL string
	Mirror sitemirror.State
	AnimeAV1CookieConfigured bool
	AV1SyncAt string
	AV1SyncError string
	AV1Watching int
	AV1Completed int
	AV1Planned int
	AV1OnHold int
	AV1Dropped int
	AV1Local int
	AV1Unmatched int
	AV1Series []avSeries
	Library []database.LibraryItem
	MALUsername string
}

func New(db *database.DB,c *crawler.Service,mirror *sitemirror.Mirror,webDir,libraryRoot,siteRoot,version,commitSHA string)(*Server,error){
	fm:=template.FuncMap{"bytes":func(v int64)string{const gb=1024*1024*1024;const mb=1024*1024;if v>=gb{return fmt.Sprintf("%.1f GB",float64(v)/gb)};if v>=mb{return fmt.Sprintf("%.1f MB",float64(v)/mb)};return fmt.Sprintf("%d B",v)}}
	t,e:=template.New("root").Funcs(fm).ParseFiles(filepath.Join(webDir,"templates","admin.html"));if e!=nil{return nil,e}
	mirror.SetSessionCookie(db.GetSetting("animeav1_session_cookie"))
	return &Server{db:db,crawl:c,mirror:mirror,av1:animeav1.New("https://animeav1.com"),tmpl:t,static:filepath.Join(webDir,"static"),libraryRoot:libraryRoot,siteRoot:siteRoot,version:version,commitSHA:commitSHA},nil
}

func (s *Server) Handler()http.Handler{
	m:=http.NewServeMux()
	m.Handle("/admin-static/",http.StripPrefix("/admin-static/",http.FileServer(http.Dir(s.static))))
	m.HandleFunc("/healthz",s.health)
	m.HandleFunc("/api/status",s.status)
	m.HandleFunc("/admin/settings",s.settings)
	m.HandleFunc("/admin/rescan",s.rescan)
	m.HandleFunc("/admin/sync-av1",s.syncAV1)
	m.HandleFunc("/admin/sync-mal",s.syncMAL)
	m.HandleFunc("/admin/mirror",s.startMirror)
	m.HandleFunc("/admin",s.admin)
	m.Handle("/",s.mirror.Handler())
	return m
}

func (s *Server) health(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"status":"ok"}`))}
func (s *Server) status(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(map[string]any{"version":s.version,"commit":s.commitSHA,"crawler":s.crawl.State.Snapshot(),"mirror":s.mirror.Snapshot(),"animeav1_session_configured":s.mirror.HasSessionCookie(),"animeav1_library_updated":s.db.GetSetting("animeav1_library_updated")})}

func (s *Server) admin(w http.ResponseWriter,r *http.Request){
	if r.URL.Path!="/admin"{http.NotFound(w,r);return}
	items,e:=s.db.Library();if e!=nil{http.Error(w,e.Error(),500);return}
	series,counts,local,unmatched:=s.avSeries(items)
	short:=s.commitSHA;if len(short)>7{short=short[:7]}
	commitURL:="";if s.commitSHA!=""&&s.commitSHA!="unknown"{commitURL="https://github.com/dart998/animeav1-archive-ex4100/commit/"+s.commitSHA}
	d:=adminData{
		Version:s.version,CommitSHA:s.commitSHA,CommitShort:short,CommitURL:commitURL,
		Mirror:s.mirror.Snapshot(),AnimeAV1CookieConfigured:s.mirror.HasSessionCookie(),
		AV1SyncAt:s.db.GetSetting("animeav1_library_updated"),AV1SyncError:s.db.GetSetting("animeav1_library_error"),
		AV1Watching:counts[0],AV1Planned:counts[1],AV1Completed:counts[2],AV1OnHold:counts[3],AV1Dropped:counts[4],AV1Local:local,AV1Unmatched:unmatched,AV1Series:series,
		Library:items,MALUsername:s.db.GetSetting("mal_username"),
	}
	if e=s.tmpl.ExecuteTemplate(w,"admin.html",d);e!=nil{http.Error(w,e.Error(),500)}
}

func (s *Server) settings(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return}
	if e:=r.ParseForm();e!=nil{http.Error(w,e.Error(),400);return}
	if v,ok:=r.Form["mal_username"];ok{if e:=s.db.SetSetting("mal_username",strings.TrimSpace(v[0]));e!=nil{http.Error(w,e.Error(),500);return}}
	if r.FormValue("clear_animeav1_cookie")=="1"{
		if e:=s.db.SetSetting("animeav1_session_cookie","");e!=nil{http.Error(w,e.Error(),500);return}
		_ = s.db.SetSetting("animeav1_library_json","")
		_ = s.db.SetSetting("animeav1_library_updated","")
		s.mirror.SetSessionCookie("")
	}else if cookie:=strings.TrimSpace(r.FormValue("animeav1_session_cookie"));cookie!=""{
		if e:=s.db.SetSetting("animeav1_session_cookie",cookie);e!=nil{http.Error(w,e.Error(),500);return}
		s.mirror.SetSessionCookie(cookie)
	}
	http.Redirect(w,r,"/admin",http.StatusSeeOther)
}

func (s *Server) syncAV1(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return}
	cookie:=s.db.GetSetting("animeav1_session_cookie")
	ctx,cancel:=context.WithTimeout(context.Background(),35*time.Second);defer cancel()
	items,err:=s.av1.Library(ctx,cookie)
	if err!=nil{
		_ = s.db.SetSetting("animeav1_library_error",err.Error())
		http.Redirect(w,r,"/admin",http.StatusSeeOther);return
	}
	b,err:=json.Marshal(items);if err!=nil{http.Error(w,err.Error(),500);return}
	if err=s.db.SetSetting("animeav1_library_json",string(b));err!=nil{http.Error(w,err.Error(),500);return}
	_ = s.db.SetSetting("animeav1_library_updated",time.Now().Format(time.RFC3339))
	_ = s.db.SetSetting("animeav1_library_error","")
	s.crawl.RefreshConfigState()
	http.Redirect(w,r,"/admin",http.StatusSeeOther)
}

func (s *Server) rescan(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};items,e:=libraryindex.Scan(s.libraryRoot);if e!=nil{http.Error(w,e.Error(),500);return};if e=s.db.ReplaceLibrary(items);e!=nil{http.Error(w,e.Error(),500);return};http.Redirect(w,r,"/admin",http.StatusSeeOther)}
func (s *Server) syncMAL(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};_ = s.crawl.RunMAL(context.Background());http.Redirect(w,r,"/admin",http.StatusSeeOther)}
func (s *Server) startMirror(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};s.mirror.Start(context.Background());http.Redirect(w,r,"/admin",http.StatusSeeOther)}

func (s *Server) avSeries(lib []database.LibraryItem)([]avSeries,map[int]int,int,int){
	var all []animeav1.Item
	if raw:=strings.TrimSpace(s.db.GetSetting("animeav1_library_json"));raw!=""{_ = json.Unmarshal([]byte(raw),&all)}
	out:=make([]avSeries,0,len(all));counts:=map[int]int{};local:=0
	for _,it:=range all{
		counts[it.Status]++
		sr:=avSeries{MediaID:string(it.MediaID),Title:it.Title,Slug:it.Slug,Status:it.StatusName(),StatusOrder:it.Status,Seen:it.Seen,Total:it.Total,Managed:it.Status==0||it.Status==2,Discovered:s.db.SeriesEpisodeCount(it.Slug)}
		if it.Slug!=""{sr.URL="/media/"+it.Slug}
		if li,kind:=matchLocal(it,lib);li!=nil{
			sr.LocalName=li.Name;sr.LocalFiles=li.Files;sr.LocalBytes=li.Bytes;sr.MatchType=kind;local++
			if normalizeName(li.Name)!=normalizeName(it.Title){sr.RenameSuggestion=it.Title}
		}
		out=append(out,sr)
	}
	sort.Slice(out,func(i,j int)bool{if out[i].StatusOrder!=out[j].StatusOrder{return out[i].StatusOrder<out[j].StatusOrder};return strings.ToLower(out[i].Title)<strings.ToLower(out[j].Title)})
	return out,counts,local,len(out)-local
}

func matchLocal(it animeav1.Item,lib []database.LibraryItem)(*database.LibraryItem,string){
	candidates:=[]string{it.Title}
	for _,v:=range it.Aliases{if strings.TrimSpace(v)!=""{candidates=append(candidates,v)}}
	for i:=range lib{
		n:=normalizeName(lib[i].Name)
		for idx,c:=range candidates{
			if n!=""&&n==normalizeName(c){if idx==0{return &lib[i],"Exacta"};return &lib[i],"Alias AV1"}
		}
	}
	return nil,""
}

func normalizeName(s string)string{
	var b strings.Builder
	for _,r:=range strings.ToLower(s){if unicode.IsLetter(r)||unicode.IsDigit(r){b.WriteRune(r)}}
	return b.String()
}
