package web

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	libraryindex "github.com/dart998/animeav1-archive-ex4100/internal/library"
)

type downloadEpisodeState struct {
	Episode int `json:"episode"`; Status string `json:"status"`; Provider string `json:"provider,omitempty"`; File string `json:"file,omitempty"`; Bytes int64 `json:"bytes,omitempty"`; TotalBytes int64 `json:"total_bytes,omitempty"`; Error string `json:"error,omitempty"`
}
type seriesDownloadState struct {
	Slug string `json:"slug"`; Running bool `json:"running"`; Preview bool `json:"preview,omitempty"`; Cancelled bool `json:"cancelled,omitempty"`; Current int `json:"current"`; Total int `json:"total"`; Downloaded int `json:"downloaded"`; Skipped int `json:"skipped"`; Errors int `json:"errors"`; CurrentFile string `json:"current_file,omitempty"`; LastError string `json:"last_error,omitempty"`; Started string `json:"started,omitempty"`; Finished string `json:"finished,omitempty"`; Episodes []downloadEpisodeState `json:"episodes"`; ForceUnplayable bool `json:"-"`
}
type episodeNotification struct { ID string `json:"id"`; Slug string `json:"slug"`; Title string `json:"title"`; Episode int `json:"episode"`; CreatedAt string `json:"created_at"`; ReadAt string `json:"read_at,omitempty"` }

var downloadStates=struct{sync.RWMutex;m map[string]*seriesDownloadState}{m:map[string]*seriesDownloadState{}}
var downloadCancels=struct{sync.Mutex;m map[string]context.CancelFunc}{m:map[string]context.CancelFunc{}}
var downloadQueueMu sync.Mutex
var megaDownloadRE=regexp.MustCompile(`server:"Mega",url:"([^"]+)"`)
var episodeAudioLanguageRE=regexp.MustCompile(`(?i)(?:["']?(?:language|lang|audio|version|type|label|name)["']?\s*[:=]\s*["']?(SUB|DUB)["']?|(?:^|[,{])\s*["']?(SUB|DUB)["']?\s*:|>\s*(SUB|DUB)\s*<)`)
var dubEpisodeFileRE=regexp.MustCompile(`(?i)(?:^|[^a-z0-9])dub(?:bed)?(?:[^a-z0-9]|$)`)
var errMegaDubSource=errors.New("fuente Mega DUB descartada")

func isDubEpisodeFileName(name string)bool{return dubEpisodeFileRE.MatchString(strings.TrimSuffix(filepath.Base(name),filepath.Ext(name)))}

func downloadSeriesTotal(itemTotal, discovered int) int {
	if discovered > itemTotal {
		return discovered
	}
	return itemTotal
}

func browserPlayablePath(path string)bool{switch strings.ToLower(filepath.Ext(path)){case ".mp4",".webm",".m4v",".mov":return true};return false}
func (s *Server) findLocalSubEpisodeFile(item animeav1.Item,episode int)(string,error){
	lib,err:=s.db.Library();if err!=nil{return "",err};folders:=localFolderCandidates(item,lib);if len(folders)==0{return "",fmt.Errorf("serie sin carpetas locales asociadas")}
	requestedSeason:=itemSeasonNumber(item);var best string;bestFolder,bestFile,bestSeason:=99,99,99
	for _,folder:=range folders{root:=filepath.Clean(folder.Item.Path);_=filepath.Walk(root,func(path string,info os.FileInfo,e error)error{
		if e!=nil||info==nil||info.IsDir()||isDubEpisodeFileName(info.Name()){return nil}
		ext:=strings.ToLower(filepath.Ext(info.Name()));switch ext{case ".mkv",".mp4",".avi",".webm",".m4v",".mov":default:return nil}
		fr:=episodeFileRank(info.Name(),item.MediaID,episode);if fr>=99{fr=legacyEpisodeFileRank(info.Name(),episode)};if fr>=99{return nil}
		sr:=seasonRankForPath(path,requestedSeason);if sr>=9{return nil}
		if best==""||sr<bestSeason||(sr==bestSeason&&fr<bestFile)||(sr==bestSeason&&fr==bestFile&&folder.Rank<bestFolder)||(sr==bestSeason&&fr==bestFile&&folder.Rank==bestFolder&&path<best){best=path;bestSeason=sr;bestFile=fr;bestFolder=folder.Rank}
		return nil})}
	if best==""{return "",fmt.Errorf("sin copia local SUB del episodio %d",episode)};return best,nil
}
func (s *Server) previewSeriesDownload(item animeav1.Item,total int) seriesDownloadState{
	st:=seriesDownloadState{Slug:item.Slug,Preview:true,Total:total,Episodes:make([]downloadEpisodeState,total)}
	for ep:=1;ep<=total;ep++{q:=downloadEpisodeState{Episode:ep,Status:"pending"};if p,e:=s.findLocalSubEpisodeFile(item,ep);e==nil{q.Status="existing";q.File=filepath.Base(p);st.Skipped++};st.Episodes[ep-1]=q}
	return st
}

func (s *Server) persistDownloadState(st *seriesDownloadState){b,_:=json.Marshal(st);_=s.db.SetSetting("download_state_"+st.Slug,string(b))}
func (s *Server) setDL(st *seriesDownloadState,f func(*seriesDownloadState)){downloadStates.Lock();f(st);cp:=*st;cp.Episodes=append([]downloadEpisodeState(nil),st.Episodes...);downloadStates.Unlock();s.persistDownloadState(&cp)}
func episodeState(st *seriesDownloadState,ep int)*downloadEpisodeState{for i:=range st.Episodes{if st.Episodes[i].Episode==ep{return &st.Episodes[i]}};return nil}

func (s *Server) downloadSeriesAPI(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};if e:=r.ParseForm();e!=nil{http.Error(w,e.Error(),400);return};slug:=strings.TrimSpace(r.FormValue("slug"));if slug==""{http.Error(w,"serie no encontrada",404);return}
	action:=r.FormValue("action");if action=="cancel"{s.cancelSeriesDownload(w,slug);return}
	item,ok:=s.itemForSlug(slug);if !ok{http.Error(w,"serie no encontrada",404);return};total:=downloadSeriesTotal(item.Total,s.db.SeriesEpisodeCount(slug));if total<1{http.Error(w,"sin episodios",409);return}
	if lib,e:=libraryindex.Scan(s.libraryRoot);e==nil{_=s.db.ReplaceLibrary(lib)}
	if action=="preview"{downloadStates.RLock();live:=downloadStates.m[slug];if live!=nil&&live.Running{cp:=*live;cp.Episodes=append([]downloadEpisodeState(nil),live.Episodes...);downloadStates.RUnlock();w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(cp);return};downloadStates.RUnlock();st:=s.previewSeriesDownload(item,total);w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(st);return}
	force:=r.FormValue("force_unplayable")=="1";if !force{var bad []int;for ep:=1;ep<=total;ep++{if p,e:=s.findLocalSubEpisodeFile(item,ep);e==nil&&!browserPlayablePath(p){bad=append(bad,ep)}};if len(bad)>0{w.Header().Set("Content-Type","application/json");w.WriteHeader(http.StatusConflict);_=json.NewEncoder(w).Encode(map[string]any{"requires_confirmation":true,"unplayable_episodes":bad});return}}
	downloadStates.Lock();if x:=downloadStates.m[slug];x!=nil&&x.Running{downloadStates.Unlock();http.Error(w,"descarga en curso",409);return};st:=&seriesDownloadState{Slug:slug,Running:true,Total:total,Started:time.Now().Format(time.RFC3339),Episodes:make([]downloadEpisodeState,total),ForceUnplayable:force};for i:=1;i<=total;i++{st.Episodes[i-1]=downloadEpisodeState{Episode:i,Status:"pending"}};downloadStates.m[slug]=st;downloadStates.Unlock()
	ctx,cancel:=context.WithCancel(context.Background());downloadCancels.Lock();downloadCancels.m[slug]=cancel;downloadCancels.Unlock();s.persistDownloadState(st);go s.runMegaSeries(ctx,item,st);w.Header().Set("Content-Type","application/json");w.WriteHeader(http.StatusAccepted);_=json.NewEncoder(w).Encode(st)
}
func (s *Server) cancelSeriesDownload(w http.ResponseWriter,slug string){downloadStates.RLock();st:=downloadStates.m[slug];running:=st!=nil&&st.Running;downloadStates.RUnlock();if !running{http.Error(w,"no hay descarga en curso",409);return};downloadCancels.Lock();cancel:=downloadCancels.m[slug];downloadCancels.Unlock();if cancel!=nil{cancel()};s.setDL(st,func(x *seriesDownloadState){x.Cancelled=true});w.Header().Set("Content-Type","application/json");w.WriteHeader(http.StatusAccepted);_=json.NewEncoder(w).Encode(map[string]bool{"cancelling":true})}
func (s *Server) finishCancelled(st *seriesDownloadState){s.setDL(st,func(x *seriesDownloadState){x.Running=false;x.Cancelled=true;x.CurrentFile="";x.LastError="";x.Finished=time.Now().Format(time.RFC3339);if q:=episodeState(x,x.Current);q!=nil&&(q.Status=="checking"||q.Status=="downloading"){q.Status="cancelled";q.Error=""}})}

func (s *Server) downloadStatusAPI(w http.ResponseWriter,r *http.Request){if r.URL.Query().Get("notifications")=="1"{s.notificationsAPI(w,r);return};if r.Method!=http.MethodGet{http.Error(w,"method not allowed",405);return};slug:=strings.TrimSpace(r.URL.Query().Get("slug"));downloadStates.RLock();x:=downloadStates.m[slug];if x!=nil{cp:=*x;cp.Episodes=append([]downloadEpisodeState(nil),x.Episodes...);downloadStates.RUnlock();w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(cp);return};downloadStates.RUnlock();raw:=strings.TrimSpace(s.db.GetSetting("download_state_"+slug));if raw==""{http.Error(w,"sin descarga",404);return};var cp seriesDownloadState;if json.Unmarshal([]byte(raw),&cp)!=nil{http.Error(w,"sin descarga",404);return};if cp.Running{cp.Running=false;cp.LastError="Descarga interrumpida por reinicio del contenedor";cp.Finished=time.Now().Format(time.RFC3339);s.persistDownloadState(&cp)};w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(cp)}

type downloadReconcileResult struct {
	States int `json:"states"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
	EpisodesFound int `json:"episodes_found"`
	EpisodesMissing int `json:"episodes_missing"`
	RunningSkipped int `json:"running_skipped"`
}

func (s *Server) reconcileDownloadsAPI(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return}
	rows,e:=s.db.Query(`SELECT key,value FROM settings WHERE key LIKE 'download_state_%' ORDER BY key`);if e!=nil{http.Error(w,e.Error(),500);return}
	type entry struct{key string;st seriesDownloadState}
	var states []entry;wanted:=map[string]bool{}
	for rows.Next(){var key,raw string;if e=rows.Scan(&key,&raw);e!=nil{rows.Close();http.Error(w,e.Error(),500);return};var st seriesDownloadState;if json.Unmarshal([]byte(raw),&st)!=nil||strings.TrimSpace(st.Slug)==""{continue};states=append(states,entry{key:key,st:st});for _,ep:=range st.Episodes{if (ep.Status=="completed"||ep.Status=="existing")&&strings.TrimSpace(ep.File)!=""{wanted[strings.ToLower(filepath.Base(ep.File))]=true}}}
	rows.Close()
	found:=map[string]bool{}
	if len(wanted)>0{_=filepath.Walk(s.libraryRoot,func(path string,info os.FileInfo,err error)error{if err!=nil||info==nil||info.IsDir(){return nil};if wanted[strings.ToLower(info.Name())]{found[strings.ToLower(info.Name())]=true};return nil})}
	result:=downloadReconcileResult{States:len(states)}
	for _,entry:=range states{
		st:=entry.st
		downloadStates.RLock();live:=downloadStates.m[st.Slug];running:=live!=nil&&live.Running;downloadStates.RUnlock()
		if running{result.RunningSkipped++;continue}
		changed:=false;claimed:=0;valid:=0
		for i:=range st.Episodes{
			ep:=&st.Episodes[i];if ep.Status!="completed"&&ep.Status!="existing"{continue};claimed++
			if !isDubEpisodeFileName(ep.File)&&found[strings.ToLower(filepath.Base(ep.File))]{valid++;result.EpisodesFound++;continue}
			ep.Status="pending";ep.Provider="";ep.File="";ep.Bytes=0;ep.TotalBytes=0;ep.Error="";result.EpisodesMissing++;changed=true
		}
		downloaded,skipped,errs:=0,0,0;for _,ep:=range st.Episodes{switch ep.Status{case "completed":downloaded++;case "existing":skipped++;case "error":errs++}}
		if st.Downloaded!=downloaded||st.Skipped!=skipped||st.Errors!=errs{st.Downloaded,st.Skipped,st.Errors=downloaded,skipped,errs;changed=true}
		if claimed>0&&valid==0{_,_=s.db.Exec(`DELETE FROM settings WHERE key=?`,entry.key);downloadStates.Lock();delete(downloadStates.m,st.Slug);downloadStates.Unlock();result.Removed++;continue}
		if changed{b,_:=json.Marshal(&st);_=s.db.SetSetting(entry.key,string(b));downloadStates.Lock();delete(downloadStates.m,st.Slug);downloadStates.Unlock();result.Updated++}
	}
	w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(result)
}

func (s *Server) loadNotifications()[]episodeNotification{var out []episodeNotification;_=json.Unmarshal([]byte(s.db.GetSetting("episode_notifications_json")),&out);cut:=time.Now().Add(-24*time.Hour);keep:=out[:0];changed:=false;for _,n:=range out{if n.ReadAt!=""{if t,e:=time.Parse(time.RFC3339,n.ReadAt);e==nil&&t.Before(cut){changed=true;continue}};keep=append(keep,n)};if changed{b,_:=json.Marshal(keep);_=s.db.SetSetting("episode_notifications_json",string(b))};return keep}
func (s *Server) notificationsAPI(w http.ResponseWriter,r *http.Request){if r.Method==http.MethodGet{n:=s.loadNotifications();unread:=0;for _,x:=range n{if x.ReadAt==""{unread++}};w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(map[string]any{"notifications":n,"unread":unread});return};if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};if e:=r.ParseForm();e!=nil{http.Error(w,e.Error(),400);return};action,id:=r.FormValue("action"),r.FormValue("id");n:=s.loadNotifications();now:=time.Now().Format(time.RFC3339);for i:=range n{if action=="read_all"||(action=="read"&&n[i].ID==id){if n[i].ReadAt==""{n[i].ReadAt=now}}};b,_:=json.Marshal(n);_=s.db.SetSetting("episode_notifications_json",string(b));w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(map[string]bool{"ok":true})}

func (s *Server) runMegaSeries(ctx context.Context,item animeav1.Item,st *seriesDownloadState){downloadQueueMu.Lock();defer downloadQueueMu.Unlock();defer func(){downloadCancels.Lock();delete(downloadCancels.m,item.Slug);downloadCancels.Unlock()}();if ctx.Err()!=nil{s.finishCancelled(st);return};target,e:=s.downloadTargetDir(item);if e!=nil{s.setDL(st,func(x *seriesDownloadState){x.Running=false;x.Errors++;x.LastError=e.Error();x.Finished=time.Now().Format(time.RFC3339)});return};for ep:=1;ep<=st.Total;ep++{if ctx.Err()!=nil{s.finishCancelled(st);return};s.setDL(st,func(x *seriesDownloadState){x.Current=ep;x.CurrentFile="";if q:=episodeState(x,ep);q!=nil{q.Status="checking"}});if p,e:=s.findLocalSubEpisodeFile(item,ep);e==nil&&(browserPlayablePath(p)||!st.ForceUnplayable){s.setDL(st,func(x *seriesDownloadState){x.Skipped++;if q:=episodeState(x,ep);q!=nil{q.Status="existing";q.File=filepath.Base(p)}});continue};megas,e:=s.episodeMegaDownloadURLs(ctx,item.Slug,ep);if e!=nil{if errors.Is(e,context.Canceled){s.finishCancelled(st);return};s.setDL(st,func(x *seriesDownloadState){x.Errors++;x.LastError=fmt.Sprintf("Ep %d: %v",ep,e);if q:=episodeState(x,ep);q!=nil{q.Status="error";q.Error=e.Error()}});continue};s.setDL(st,func(x *seriesDownloadState){if q:=episodeState(x,ep);q!=nil{q.Status="downloading";q.Provider="Mega SUB"}});var p string;var downloadErr error;for _,mega:=range megas{p,downloadErr=downloadMega(ctx,mega,target,func(name string,done,total int64){s.setDL(st,func(x *seriesDownloadState){x.CurrentFile=name;if q:=episodeState(x,ep);q!=nil{q.File=name;q.Bytes=done;q.TotalBytes=total}})});if downloadErr==nil{break};if errors.Is(downloadErr,context.Canceled){s.finishCancelled(st);return};if errors.Is(downloadErr,errMegaDubSource){continue}};if downloadErr!=nil||p==""{if downloadErr==nil{downloadErr=errors.New("sin enlace Mega SUB utilizable")};s.setDL(st,func(x *seriesDownloadState){x.Errors++;x.LastError=fmt.Sprintf("Ep %d: %v",ep,downloadErr);if q:=episodeState(x,ep);q!=nil{q.Status="error";q.Error=downloadErr.Error()}});continue};p=ensureDownloadedEpisodeName(p,item.MediaID,ep);if items,e:=libraryindex.Scan(s.libraryRoot);e==nil{_=s.db.ReplaceLibrary(items)};s.setDL(st,func(x *seriesDownloadState){x.Downloaded++;x.CurrentFile=filepath.Base(p);if q:=episodeState(x,ep);q!=nil{q.Status="completed";q.File=filepath.Base(p);q.Error=""}})};s.setDL(st,func(x *seriesDownloadState){x.Running=false;x.CurrentFile="";x.Finished=time.Now().Format(time.RFC3339)})}

func ensureDownloadedEpisodeName(path string,mediaID animeav1.IDString,episode int)string{if episodeFileRank(filepath.Base(path),mediaID,episode)<99{return path};dir,name:=filepath.Dir(path),filepath.Base(path);prefix:=fmt.Sprintf("EP%02d_",episode);if id:=strings.TrimSpace(string(mediaID));id!=""{prefix=id+"_"+strconv.Itoa(episode)+"_"};dest:=filepath.Join(dir,prefix+name);if _,e:=os.Stat(dest);e==nil{ext:=filepath.Ext(name);base:=strings.TrimSuffix(prefix+name,ext);for i:=1;;i++{candidate:=filepath.Join(dir,fmt.Sprintf("%s_%d%s",base,i,ext));if _,e:=os.Stat(candidate);os.IsNotExist(e){dest=candidate;break}}};if e:=os.Rename(path,dest);e==nil{_=os.Chmod(dest,0666);return dest};return path}
type audioLanguageMarker struct{Pos int;Lang string}
func episodeAudioMarkers(body []byte)[]audioLanguageMarker{matches:=episodeAudioLanguageRE.FindAllSubmatchIndex(body,-1);out:=make([]audioLanguageMarker,0,len(matches));for _,m:=range matches{lang:="";for g:=2;g+1<len(m);g+=2{if m[g]>=0&&m[g+1]>=0{v:=strings.ToUpper(string(body[m[g]:m[g+1]]));if v=="SUB"||v=="DUB"{lang=v;break}}};if lang!=""{out=append(out,audioLanguageMarker{Pos:m[0],Lang:lang})}};return out}
func validMegaDownloadURL(raw string)bool{u,e:=url.Parse(raw);if e!=nil{return false};host,path:=strings.ToLower(u.Hostname()),strings.ToLower(u.Path);return (strings.Contains(host,"mega.nz")||strings.Contains(host,"mega.co.nz"))&&(strings.Contains(path,"/file/")||strings.HasPrefix(u.Fragment,"!"))}
func selectSubMegaDownloadURLs(body []byte)[]string{
	matches:=megaDownloadRE.FindAllSubmatchIndex(body,-1);markers:=episodeAudioMarkers(body);seen:=map[string]bool{};subs:=[]string{};unclassified:=[]string{}
	for _,m:=range matches{if len(m)<4||m[2]<0||m[3]<0{continue};raw:=string(body[m[2]:m[3]]);if !validMegaDownloadURL(raw)||seen[raw]{continue};seen[raw]=true;lang:="";for _,mk:=range markers{if mk.Pos>m[0]{break};lang=mk.Lang};if lang=="SUB"{subs=append(subs,raw)}else if lang==""{unclassified=append(unclassified,raw)}}
	if len(subs)>0{return subs};if len(markers)==0&&len(unclassified)==1{return unclassified};return nil
}
func (s *Server) episodeMegaDownloadURLs(ctx context.Context,slug string,ep int)([]string,error){b,e:=s.fetchEpisodeHTML(ctx,slug,ep);if e!=nil{return nil,e};out:=selectSubMegaDownloadURLs(b);if len(out)==0{return nil,errors.New("sin enlace de descarga Mega SUB para este episodio")};return out,nil}
func (s *Server) downloadTargetDir(item animeav1.Item)(string,error){lib,e:=libraryindex.Scan(s.libraryRoot);if e!=nil{return "",e};_=s.db.ReplaceLibrary(lib);if c:=localFolderCandidates(item,lib);len(c)>0{return c[0].Item.Path,nil};return createSeriesDir(s.libraryRoot,item.Title)}
func safeDirName(s string)string{return strings.TrimSpace(strings.NewReplacer("/","-","\\","-",":"," -","*","","?","","\"","","<","",">","","|","-").Replace(s))}
func truncateUTF8Bytes(s string,max int)string{if max<=0{return ""};if len(s)<=max{return s};end:=max;for end>0&&!utf8.ValidString(s[:end]){end--};return s[:end]}
func compactDirName(name string,max int)string{sum:=sha256.Sum256([]byte(name));suffix:=fmt.Sprintf("-%x",sum[:4]);limit:=max-len(suffix);if limit<1{limit=1};prefix:=truncateUTF8Bytes(name,limit);prefix=strings.TrimRight(strings.TrimSpace(prefix),". -_");if prefix==""{prefix="Anime"};return prefix+suffix}
func exactDirEntry(root,name string)bool{xs,e:=os.ReadDir(root);if e!=nil{return false};for _,x:=range xs{if x.IsDir()&&x.Name()==name{return true}};return false}
func nameLimitError(e error)bool{if e==nil{return false};if errors.Is(e,syscall.ENAMETOOLONG){return true};x:=strings.ToLower(e.Error());return strings.Contains(x,"file name too long")||strings.Contains(x,"name too long")}
func createSeriesDir(root,title string)(string,error){
	if e:=os.MkdirAll(root,0777);e!=nil{return "",e}
	name:=safeDirName(title);if name==""{name="Anime"}
	attempt:=func(candidate string)(string,bool,error){
		dir:=filepath.Join(root,candidate);e:=os.Mkdir(dir,0777)
		if e==nil{if exactDirEntry(root,candidate){_=os.Chmod(dir,0777);return dir,true,nil};xs,readErr:=os.ReadDir(dir);if readErr==nil&&len(xs)==0{if rmErr:=os.Remove(dir);rmErr==nil{return "",false,nil}};return "",false,fmt.Errorf("el filesystem no conserva el nombre de carpeta solicitado %q",candidate)}
		if os.IsExist(e){st,se:=os.Stat(dir);if se==nil&&st.IsDir()&&exactDirEntry(root,candidate){return dir,true,nil};return "",false,fmt.Errorf("conflicto con el nombre de carpeta %q",candidate)}
		if nameLimitError(e){return "",false,nil};return "",false,e
	}
	if dir,ok,e:=attempt(name);ok||e!=nil{return dir,e}
	for _,limit:=range []int{240,200,160,120,80,60,40}{candidate:=compactDirName(name,limit);if candidate==name{continue};if dir,ok,e:=attempt(candidate);ok||e!=nil{return dir,e}}
	return "",fmt.Errorf("no se pudo crear una carpeta válida para %q",title)
}

type megaResp struct{G string `json:"g"`;S int64 `json:"s"`;AT string `json:"at"`;E int `json:"e,omitempty"`;TL int `json:"tl,omitempty"`}
func megaAPIErrorText(code int)string{switch code{case -2:return "argumentos inválidos";case -3:return "error temporal";case -4:return "límite de peticiones";case -6:return "demasiadas solicitudes o transferencias";case -8:return "enlace caducado";case -9:return "archivo no encontrado";case -11:return "acceso denegado";case -14:return "clave criptográfica incorrecta";case -16:return "archivo bloqueado por Mega";case -17:return "cuota de transferencia superada";case -18:return "recurso temporalmente no disponible";case -19:return "demasiadas conexiones al recurso";default:return "error de Mega"}}
func megaAPIShouldRetry(code int)bool{return code==-3||code==-4||code==-6||code==-18||code==-19}
func parseMegaInfoResponse(b []byte)(megaResp,int,error){
	trim:=bytes.TrimSpace(b);if len(trim)==0{return megaResp{},0,errors.New("respuesta Mega vacía")}
	var direct int;if trim[0]!='['&&trim[0]!='{'{if json.Unmarshal(trim,&direct)==nil&&direct<0{return megaResp{},direct,nil}}
	var items []json.RawMessage;if e:=json.Unmarshal(trim,&items);e!=nil||len(items)==0{return megaResp{},0,fmt.Errorf("respuesta Mega inválida: %s",strings.TrimSpace(string(trim)))}
	first:=bytes.TrimSpace(items[0]);var code int;if len(first)>0&&first[0]!='{'{if json.Unmarshal(first,&code)==nil&&code<0{return megaResp{},code,nil}}
	var rr megaResp;if e:=json.Unmarshal(first,&rr);e!=nil{return megaResp{},0,fmt.Errorf("respuesta Mega inválida: %s",strings.TrimSpace(string(trim)))};if rr.E<0{return rr,rr.E,nil};if rr.G==""{return megaResp{},0,errors.New("Mega no devolvió URL de descarga")};return rr,0,nil
}
type progressReader struct{r io.Reader;done,total int64;last time.Time;cb func(int64,int64)}
func (p *progressReader)Read(b []byte)(int,error){n,e:=p.r.Read(b);p.done+=int64(n);now:=time.Now();if p.cb!=nil&&(p.last.IsZero()||now.Sub(p.last)>=time.Second||p.done==p.total){p.last=now;p.cb(p.done,p.total)};return n,e}
func downloadMega(ctx context.Context,raw,target string,progress func(string,int64,int64))(string,error){u,e:=url.Parse(raw);if e!=nil{return "",e};parts:=strings.Split(strings.Trim(u.Path,"/"),"/");var handle,fragment string;if len(parts)>=2&&strings.EqualFold(parts[0],"file"){handle,fragment=parts[1],u.Fragment}else if u.Fragment!=""&&strings.Contains(u.Fragment,"!"){bits:=strings.Split(strings.TrimPrefix(u.Fragment,"!"),"!");if len(bits)>=2{handle,fragment=bits[0],bits[1]}};if handle==""||fragment==""{return "",errors.New("URL Mega de descarga invalida")};keyRaw,e:=base64.RawURLEncoding.DecodeString(strings.TrimSpace(fragment));if e!=nil||len(keyRaw)!=32{return "",errors.New("clave Mega invalida")};key:=make([]byte,16);for i:=0;i<16;i++{key[i]=keyRaw[i]^keyRaw[i+16]};iv:=make([]byte,16);copy(iv,keyRaw[16:24]);payload:=fmt.Sprintf(`[{"a":"g","g":1,"p":"%s"}]`,handle);var info megaResp;for attempt:=0;attempt<4;attempt++{api:="https://g.api.mega.co.nz/cs?id="+strconv.FormatInt(time.Now().UnixNano(),10);req,err:=http.NewRequestWithContext(ctx,http.MethodPost,api,strings.NewReader(payload));if err!=nil{return "",err};req.Header.Set("Content-Type","application/json");resp,err:=http.DefaultClient.Do(req);if err!=nil{if attempt<3{select{case <-ctx.Done():return "",ctx.Err();case <-time.After(time.Duration(1<<attempt)*time.Second):continue}};return "",err};b,readErr:=io.ReadAll(io.LimitReader(resp.Body,2<<20));resp.Body.Close();if readErr!=nil{return "",readErr};parsed,code,parseErr:=parseMegaInfoResponse(b);if parseErr!=nil{return "",parseErr};if code<0{if megaAPIShouldRetry(code)&&attempt<3{select{case <-ctx.Done():return "",ctx.Err();case <-time.After(time.Duration(1<<attempt)*time.Second):continue}};if parsed.TL>0{return "",fmt.Errorf("Mega API: %s (%d), espera sugerida %d s",megaAPIErrorText(code),code,parsed.TL)};return "",fmt.Errorf("Mega API: %s (%d)",megaAPIErrorText(code),code)};info=parsed;break};if info.G==""{return "",errors.New("Mega API: no se obtuvo URL de descarga")};name:="mega-"+handle+".mp4";if info.AT!=""{if n:=megaName(info.AT,key);n!=""{name=filepath.Base(n)}};if isDubEpisodeFileName(name){return "",errMegaDubSource};if e=os.MkdirAll(target,0777);e!=nil{return "",e};dest:=filepath.Join(target,name);if _,e:=os.Stat(dest);e==nil{ext:=filepath.Ext(name);base:=strings.TrimSuffix(name,ext);for i:=1;;i++{candidate:=filepath.Join(target,fmt.Sprintf("%s (descarga %d)%s",base,i,ext));if _,e:=os.Stat(candidate);os.IsNotExist(e){dest=candidate;name=filepath.Base(candidate);break}}};part:=dest+".part";cleanup:=true;defer func(){if cleanup{_=os.Remove(part)}}();get,e:=http.NewRequestWithContext(ctx,http.MethodGet,info.G,nil);if e!=nil{return "",e};r,e:=http.DefaultClient.Do(get);if e!=nil{return "",e};defer r.Body.Close();if r.StatusCode<200||r.StatusCode>=300{return "",fmt.Errorf("Mega descarga: %s",r.Status)};f,e:=os.OpenFile(part,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0666);if e!=nil{return "",e};_=os.Chmod(part,0666);block,e:=aes.NewCipher(key);if e!=nil{f.Close();return "",e};stream:=&cipher.StreamReader{S:cipher.NewCTR(block,iv),R:r.Body};pr:=&progressReader{r:stream,total:info.S,cb:func(done,total int64){if progress!=nil{progress(name,done,total)}}};if progress!=nil{progress(name,0,info.S)};_,copyErr:=io.Copy(f,pr);closeErr:=f.Close();if copyErr!=nil{return "",copyErr};if closeErr!=nil{return "",closeErr};if ctx.Err()!=nil{return "",ctx.Err()};if info.S>0{st,e:=os.Stat(part);if e!=nil{return "",e};if st.Size()!=info.S{return "",fmt.Errorf("descarga Mega incompleta: %d/%d bytes",st.Size(),info.S)}};if e=os.Rename(part,dest);e!=nil{return "",e};cleanup=false;_=os.Chmod(dest,0666);return dest,nil}
func megaName(at string,key []byte)string{b,e:=base64.RawURLEncoding.DecodeString(at);if e!=nil||len(b)==0||len(b)%16!=0{return ""};block,e:=aes.NewCipher(key);if e!=nil{return ""};out:=make([]byte,len(b));cipher.NewCBCDecrypter(block,make([]byte,16)).CryptBlocks(out,b);s:=strings.TrimRight(string(out),"\x00");if !strings.HasPrefix(s,"MEGA"){return ""};var x struct{N string `json:"n"`};if json.Unmarshal([]byte(s[4:]),&x)!=nil{return ""};return x.N}
