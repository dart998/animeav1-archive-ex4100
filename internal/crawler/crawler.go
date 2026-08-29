package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dart998/animeav1-archive-ex4100/internal/archive"
	"github.com/dart998/animeav1-archive-ex4100/internal/config"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
	"github.com/dart998/animeav1-archive-ex4100/internal/providers"
)

type State struct { mu sync.RWMutex; Running bool `json:"running"`; LastStart,LastFinish,LastStatus,LastError string }
func (s *State) Snapshot() State { s.mu.RLock(); defer s.mu.RUnlock(); return State{Running:s.Running,LastStart:s.LastStart,LastFinish:s.LastFinish,LastStatus:s.LastStatus,LastError:s.LastError} }

type Service struct { cfg config.Config; db *database.DB; client *http.Client; State *State }
type episodeMeta struct { Anime string `json:"anime"`; Slug string `json:"slug"`; Episode int `json:"episode"`; URL string `json:"url"`; SelectedProvider string `json:"selected_provider"`; SelectedURL string `json:"selected_url,omitempty"`; Sources []database.Source `json:"sources"`; ArchivedPath string `json:"archived_path,omitempty"`; SHA256 string `json:"sha256,omitempty"` }

var h1RE=regexp.MustCompile(`(?is)<h1[^>]*>\s*([^<]+)`)
func New(cfg config.Config, db *database.DB)*Service{return &Service{cfg:cfg,db:db,client:&http.Client{Timeout:25*time.Second},State:&State{}}}
func (s *Service) RunAll(ctx context.Context) error { s.State.mu.Lock(); if s.State.Running {s.State.mu.Unlock();return fmt.Errorf("crawl already running")}; s.State.Running=true;s.State.LastStart=time.Now().Format(time.RFC3339);s.State.mu.Unlock(); var first error; for _,t:=range s.cfg.CrawlerTargets{if err:=s.RunTarget(ctx,t);err!=nil&&first==nil{first=err}}; s.State.mu.Lock();s.State.Running=false;s.State.LastFinish=time.Now().Format(time.RFC3339);if first!=nil{s.State.LastStatus="error";s.State.LastError=first.Error()}else{s.State.LastStatus="ok";s.State.LastError=""};s.State.mu.Unlock();return first }
func (s *Service) RunTarget(ctx context.Context,slug string) error {
	runID,_:=s.db.BeginRun(slug); status:="ok"; msg:=""; defer func(){s.db.FinishRun(runID,status,msg)}()
	animeURL:=strings.TrimRight(s.cfg.BaseURL,"/")+"/media/"+slug; log.Printf("[CRAWL] %s",slug); body,err:=s.fetch(ctx,animeURL); if err!=nil{status="error";msg=err.Error();return err}
	title:=slug; if m:=h1RE.FindStringSubmatch(body);len(m)>1{title=strings.TrimSpace(m[1])}; animeID,err:=s.db.UpsertAnime(slug,title,animeURL);if err!=nil{return err}
	epRE:=regexp.MustCompile(`/media/`+regexp.QuoteMeta(slug)+`/(\d+)`); nums:=map[int]bool{};for _,m:=range epRE.FindAllStringSubmatch(body,-1){n,_:=strconv.Atoi(m[1]);if n>0{nums[n]=true}}; episodes:=make([]int,0,len(nums));for n:=range nums{episodes=append(episodes,n)};sort.Ints(episodes);log.Printf("[CRAWL] %d episodios encontrados",len(episodes))
	for _,n:=range episodes { if err:=s.crawlEpisode(ctx,animeID,slug,title,n);err!=nil{log.Printf("[EP%02d] error: %v",n,err)} }
	return nil
}
func (s *Service) crawlEpisode(ctx context.Context,animeID int64,slug,title string,n int) error {
	u:=fmt.Sprintf("%s/media/%s/%d",strings.TrimRight(s.cfg.BaseURL,"/"),slug,n); log.Printf("[EP%02d] Analizando reproductores",n); body,err:=s.fetch(ctx,u);if err!=nil{return err}; epID,err:=s.db.UpsertEpisode(animeID,n,fmt.Sprintf("Episodio %d",n),u);if err!=nil{return err}
	sources:=providers.Detect(body,s.cfg.ProviderOrder); var selected *database.Source
	for i:=range sources { ok,why:=providers.Validate(ctx,sources[i]); if ok {sources[i].Status="ok"; log.Printf("[EP%02d] %s ... OK",n,sources[i].Provider); if selected==nil {cp:=sources[i];selected=&cp}; if !s.cfg.ProviderFallback {break} } else {sources[i].Status="failed";sources[i].Error=why;log.Printf("[EP%02d] %s ... FAILED (%s)",n,sources[i].Provider,why)} }
	if err=s.db.SaveSources(epID,sources);err!=nil{return err}; meta:=episodeMeta{Anime:title,Slug:slug,Episode:n,URL:u,Sources:sources}
	if selected!=nil {meta.SelectedProvider=selected.Provider;meta.SelectedURL=selected.URL;s.db.SelectSource(epID,selected.Provider,selected.URL,"source_ready");log.Printf("[EP%02d] selected provider: %s",n,selected.Provider)
		if s.cfg.DownloadVideos { if !s.cfg.DownloadAuthorized {log.Printf("[EP%02d] download blocked: VIDEO_DOWNLOAD_AUTHORIZED is not true",n)} else {p,sha,bytes,e:=archive.Save(ctx,s.cfg.VideoDir,slug,n,selected.URL);if e!=nil{log.Printf("[EP%02d] archive failed: %v",n,e)}else{meta.ArchivedPath=p;meta.SHA256=sha;s.db.RecordDownload(epID,selected.Provider,p,sha,bytes);s.db.SelectSource(epID,selected.Provider,selected.URL,"archived")}} }
	} else {s.db.SelectSource(epID,"","","source_unavailable")}
	return s.writeMeta(slug,n,meta)
}
func (s *Service) fetch(ctx context.Context,u string)(string,error){req,e:=http.NewRequestWithContext(ctx,http.MethodGet,u,nil);if e!=nil{return "",e};req.Header.Set("User-Agent","Mozilla/5.0 AnimeAV1Archive/0.1");resp,e:=s.client.Do(req);if e!=nil{return "",e};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=400{return "",fmt.Errorf("GET %s: %s",u,resp.Status)};b,e:=io.ReadAll(io.LimitReader(resp.Body,8<<20));return string(b),e}
func (s *Service) writeMeta(slug string,n int,v any)error{dir:=filepath.Join(s.cfg.MetadataDir,slug);if e:=os.MkdirAll(dir,0o755);e!=nil{return e};b,e:=json.MarshalIndent(v,"","  ");if e!=nil{return e};return os.WriteFile(filepath.Join(dir,fmt.Sprintf("%03d.json",n)),b,0o644)}
