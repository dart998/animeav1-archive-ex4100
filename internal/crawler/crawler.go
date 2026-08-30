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

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	"github.com/dart998/animeav1-archive-ex4100/internal/archive"
	"github.com/dart998/animeav1-archive-ex4100/internal/config"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
	malclient "github.com/dart998/animeav1-archive-ex4100/internal/mal"
	"github.com/dart998/animeav1-archive-ex4100/internal/providers"
)

type State struct { mu sync.RWMutex; Running bool `json:"running"`; LastStart,LastFinish,LastStatus,LastError string; AV1Series int `json:"animeav1_series"`; AV1Crawled int `json:"animeav1_crawled"`; MALWatching int `json:"mal_watching"`; MALMatched int `json:"mal_matched"` }
func (s *State) Snapshot() State { s.mu.RLock(); defer s.mu.RUnlock(); return State{Running:s.Running,LastStart:s.LastStart,LastFinish:s.LastFinish,LastStatus:s.LastStatus,LastError:s.LastError,AV1Series:s.AV1Series,AV1Crawled:s.AV1Crawled,MALWatching:s.MALWatching,MALMatched:s.MALMatched} }

type Service struct { cfg config.Config; db *database.DB; client *http.Client; mal *malclient.Client; State *State }
type episodeMeta struct { Anime string `json:"anime"`; Slug string `json:"slug"`; Episode int `json:"episode"`; URL string `json:"url"`; SelectedProvider string `json:"selected_provider"`; SelectedURL string `json:"selected_url,omitempty"`; Sources []database.Source `json:"sources"`; ArchivedPath string `json:"archived_path,omitempty"`; SHA256 string `json:"sha256,omitempty"` }

var h1RE=regexp.MustCompile(`(?is)<h1[^>]*>\s*([^<]+)`)
func New(cfg config.Config, db *database.DB)*Service{ s:=&Service{cfg:cfg,db:db,client:&http.Client{Timeout:25*time.Second},mal:malclient.New(),State:&State{}}; s.RefreshConfigState(); return s }
func (s *Service) RefreshConfigState(){
	s.State.mu.Lock();defer s.State.mu.Unlock()
	if strings.TrimSpace(s.db.GetSetting("animeav1_library_json"))==""{s.State.LastStatus="waiting_for_animeav1";s.State.LastError="Configura la sesión y actualiza las listas de AnimeAV1 en /admin";return}
	s.State.LastStatus="animeav1_configured"
	s.State.LastError=""
}
func (s *Service) RunAll(ctx context.Context) error {
	s.State.mu.Lock(); if s.State.Running {s.State.mu.Unlock();return fmt.Errorf("crawl already running")}; s.State.Running=true;s.State.LastStart=time.Now().Format(time.RFC3339);s.State.mu.Unlock()
	defer func(){s.State.mu.Lock();s.State.Running=false;s.State.LastFinish=time.Now().Format(time.RFC3339);s.State.mu.Unlock()}()
	items,err:=s.animeAV1Scope();if err!=nil{return s.fail("animeav1_scope_error",err)}
	if len(items)==0{s.State.mu.Lock();s.State.AV1Series=0;s.State.AV1Crawled=0;s.State.LastStatus="waiting_for_animeav1";s.State.LastError="Actualiza las listas de AnimeAV1 en /admin";s.State.mu.Unlock();return nil}
	sort.Slice(items,func(i,j int)bool{if items[i].Status!=items[j].Status{return items[i].Status<items[j].Status};return strings.ToLower(items[i].Title)<strings.ToLower(items[j].Title)})
	batch:=s.cfg.CrawlerBatchSize;if batch<1{batch=5};cursor,_:=strconv.Atoi(s.db.GetSetting("crawler_scope_cursor"));if cursor<0||cursor>=len(items){cursor=0};end:=cursor+batch;if end>len(items){end=len(items)}
	crawled:=0;s.State.mu.Lock();s.State.AV1Series=len(items);s.State.AV1Crawled=0;s.State.mu.Unlock()
	for _,item:=range items[cursor:end]{if strings.TrimSpace(item.Slug)==""{log.Printf("[AV1] %s: omitida porque no tiene slug",item.Title);continue};if err=s.RunTarget(ctx,item.Slug);err!=nil{log.Printf("[AV1] %s: %v",item.Slug,err);continue};crawled++;s.State.mu.Lock();s.State.AV1Crawled=crawled;s.State.mu.Unlock()}
	next:=end;if next>=len(items){next=0};_ = s.db.SetSetting("crawler_scope_cursor",strconv.Itoa(next))
	s.State.mu.Lock();s.State.LastStatus="animeav1_crawled";s.State.LastError="";s.State.mu.Unlock();log.Printf("[AV1] lote %d-%d de %d: %d series rastreadas",cursor+1,end,len(items),crawled)
	return nil
}
func (s *Service) animeAV1Scope()([]animeav1.Item,error){raw:=strings.TrimSpace(s.db.GetSetting("animeav1_library_json"));if raw==""{return nil,nil};var items []animeav1.Item;if err:=json.Unmarshal([]byte(raw),&items);err!=nil{return nil,fmt.Errorf("listas AnimeAV1 inválidas: %w",err)};return animeav1.InScope(items),nil}
func (s *Service) RunMAL(ctx context.Context) error {
	username:=strings.TrimSpace(s.db.GetSetting("mal_username"))
	if username==""{return fmt.Errorf("configura el usuario público de MyAnimeList en /admin")}
	library,err:=s.db.Library();if err!=nil{return s.fail("library_error",err)}
	watching,err:=s.mal.Watching(ctx,username,library);if err!=nil{return s.fail("mal_error",err)}
	if err=s.db.ReplaceMALWatching(watching);err!=nil{return s.fail("mal_store_error",err)}
	matched:=0;for _,x:=range watching{if x.LocalPath!=""{matched++}}
	s.State.mu.Lock();s.State.MALWatching=len(watching);s.State.MALMatched=matched;s.State.LastStatus="mal_synced";s.State.LastError="";s.State.mu.Unlock()
	log.Printf("[MAL] %s: %d en Watching, %d con carpeta local candidata",username,len(watching),matched)
	return nil
}
func (s *Service) fail(status string,err error)error{s.State.mu.Lock();s.State.LastStatus=status;s.State.LastError=err.Error();s.State.mu.Unlock();return err}
func (s *Service) RunTarget(ctx context.Context,slug string) error {
	runID,_:=s.db.BeginRun(slug); status:="ok"; msg:=""; defer func(){s.db.FinishRun(runID,status,msg)}()
	animeURL:=strings.TrimRight(s.cfg.BaseURL,"/")+"/media/"+slug; log.Printf("[CRAWL] %s",slug); body,err:=s.fetch(ctx,animeURL); if err!=nil{status="error";msg=err.Error();return err}
	title:=slug; if m:=h1RE.FindStringSubmatch(body);len(m)>1{title=strings.TrimSpace(m[1])}; animeID,err:=s.db.UpsertAnime(slug,title,animeURL);if err!=nil{return err}
	epRE:=regexp.MustCompile(`/media/`+regexp.QuoteMeta(slug)+`/(\d+)`); nums:=map[int]bool{};for _,m:=range epRE.FindAllStringSubmatch(body,-1){n,_:=strconv.Atoi(m[1]);if n>0{nums[n]=true}}; episodes:=make([]int,0,len(nums));for n:=range nums{episodes=append(episodes,n)};sort.Ints(episodes);log.Printf("[CRAWL] %d episodios encontrados",len(episodes))
	for _,n:=range episodes { if _,err:=s.db.UpsertEpisode(animeID,n,fmt.Sprintf("Episodio %d",n),fmt.Sprintf("%s/media/%s/%d",strings.TrimRight(s.cfg.BaseURL,"/"),slug,n));err!=nil{log.Printf("[EP%02d] error guardando episodio: %v",n,err)} }
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
