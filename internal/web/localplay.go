package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
)

var episodePathRE = regexp.MustCompile(`^/media/([^/]+)/(\d+)/?$`)

type localEpisodeInfo struct {
	Available       bool   `json:"available"`
	VideoURL        string `json:"video_url,omitempty"`
	FileName        string `json:"file_name,omitempty"`
	BrowserPlayable bool   `json:"browser_playable"`
	Seen            bool   `json:"seen"`
	SeenThrough     int    `json:"seen_through"`
	Episode         int    `json:"episode"`
	Title           string `json:"title,omitempty"`
}

func (s *Server) mediaMirror(w http.ResponseWriter, r *http.Request) {
	m := episodePathRE.FindStringSubmatch(r.URL.Path)
	if len(m) != 3 { s.siteMirror(w, r); return }
	rr := httptest.NewRecorder(); s.mirror.Handler().ServeHTTP(rr, r); res := rr.Result(); defer res.Body.Close()
	for k,v:=range res.Header{for _,x:=range v{w.Header().Add(k,x)}}
	body:=rr.Body.Bytes()
	if res.StatusCode>=200&&res.StatusCode<300&&strings.Contains(strings.ToLower(res.Header.Get("Content-Type")),"text/html"){
		body=patchMirrorHTML(body);bridge:=[]byte(localEpisodeBridge)
		if i:=bytes.LastIndex(bytes.ToLower(body),[]byte("</body>"));i>=0{body=append(append(append([]byte{},body[:i]...),bridge...),body[i:]...)}else{body=append(body,bridge...)}
		w.Header().Set("Content-Length",strconv.Itoa(len(body)))
	}
	w.WriteHeader(res.StatusCode);if r.Method!=http.MethodHead{_,_=w.Write(body)}
}

func (s *Server) localEpisodeAPI(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodGet{http.Error(w,"method not allowed",405);return};slug:=strings.TrimSpace(r.URL.Query().Get("slug"));episode,_:=strconv.Atoi(r.URL.Query().Get("episode"));if slug==""||episode<1{http.Error(w,"bad request",400);return};info,err:=s.localEpisode(slug,episode);if err!=nil{http.Error(w,err.Error(),404);return};w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(info)}

func (s *Server) localVideo(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodGet&&r.Method!=http.MethodHead{http.Error(w,"method not allowed",405);return};parts:=strings.Split(strings.TrimPrefix(r.URL.Path,"/api/local-video/"),"/");if len(parts)!=2{http.NotFound(w,r);return};episode,err:=strconv.Atoi(parts[1]);if err!=nil||episode<1{http.NotFound(w,r);return};path,_,err:=s.findLocalEpisodeFile(parts[0],episode);if err!=nil{http.NotFound(w,r);return};if c:=mime.TypeByExtension(filepath.Ext(path));c!=""{w.Header().Set("Content-Type",c)};w.Header().Set("Cache-Control","private, max-age=3600");w.Header().Set("X-AnimeAV1-Source","local-video");http.ServeFile(w,r,path)}

func (s *Server) markWatched(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};if err:=r.ParseForm();err!=nil{http.Error(w,err.Error(),400);return};slug:=strings.TrimSpace(r.FormValue("slug"));episode,_:=strconv.Atoi(r.FormValue("episode"));if slug==""||episode<1{http.Error(w,"bad request",400);return};items:=s.cachedAV1();var item *animeav1.Item;for i:=range items{if items[i].Slug==slug{item=&items[i];break}};if item==nil{http.Error(w,"serie no encontrada en las listas de AnimeAV1",404);return};ctx,cancel:=context.WithTimeout(r.Context(),35*time.Second);defer cancel();if err:=s.av1.MarkWatched(ctx,s.db.GetSetting("animeav1_session_cookie"),item.MediaID,episode);err!=nil{http.Error(w,err.Error(),502);return};go s.refreshAnimeAV1Cache();w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"ok":true}`))}

func (s *Server) cachedAV1()[]animeav1.Item{var items []animeav1.Item;if raw:=strings.TrimSpace(s.db.GetSetting("animeav1_library_json"));raw!=""{_=json.Unmarshal([]byte(raw),&items)};return items}

func (s *Server) localEpisode(slug string,episode int)(localEpisodeInfo,error){path,item,err:=s.findLocalEpisodeFile(slug,episode);if err!=nil{return localEpisodeInfo{},err};ext:=strings.ToLower(filepath.Ext(path));return localEpisodeInfo{Available:true,VideoURL:fmt.Sprintf("/api/local-video/%s/%d",slug,episode),FileName:filepath.Base(path),BrowserPlayable:ext==".mp4"||ext==".webm"||ext==".m4v"||ext==".mov",Seen:item.Seen>=episode,SeenThrough:item.Seen,Episode:episode,Title:item.Title},nil}

func (s *Server) itemForSlug(slug string)(animeav1.Item,bool){for _,it:=range s.cachedAV1(){if it.Slug==slug{return it,true}};var title string;if err:=s.db.QueryRow(`SELECT title FROM anime WHERE slug=?`,slug).Scan(&title);err==nil&&strings.TrimSpace(title)!=""{return animeav1.Item{Slug:slug,Title:title,Aliases:map[string]string{}},true};return animeav1.Item{},false}

func legacyEpisodeFileRank(name string,episode int)int{base:=strings.TrimSuffix(filepath.Base(name),filepath.Ext(name));re:=regexp.MustCompile(fmt.Sprintf(`(?i)^\d+[_ .-]+0*%d(?:[_ .-]|$)`,episode));if re.MatchString(base){return 2};return 99}

func (s *Server) findLocalEpisodeFile(slug string,episode int)(string,animeav1.Item,error){item,ok:=s.itemForSlug(slug);if !ok{return "",animeav1.Item{},fmt.Errorf("serie no encontrada")};lib,err:=s.db.Library();if err!=nil{return "",item,err};folders:=localFolderCandidates(item,lib);if len(folders)==0{return "",item,fmt.Errorf("serie sin carpetas locales asociadas")};requestedSeason:=seasonNumber(item.Title);if requestedSeason==0{for _,a:=range item.Aliases{if n:=seasonNumber(a);n>0{requestedSeason=n;break}}};if requestedSeason==0{requestedSeason=1};var files []episodeFileCandidate;for _,folder:=range folders{root:=filepath.Clean(folder.Item.Path);_=filepath.Walk(root,func(path string,info os.FileInfo,e error)error{if e!=nil||info==nil||info.IsDir(){return nil};ext:=strings.ToLower(filepath.Ext(info.Name()));switch ext{case ".mkv",".mp4",".avi",".webm",".m4v",".mov":default:return nil};fr:=episodeFileRank(info.Name(),item.MediaID,episode);if fr>=99{fr=legacyEpisodeFileRank(info.Name(),episode)};if fr>=99{return nil};sr:=seasonRankForPath(path,requestedSeason);if sr>=9{return nil};files=append(files,episodeFileCandidate{Path:path,FolderRank:folder.Rank,FileRank:fr,SeasonRank:sr});return nil})};if len(files)==0{return "",item,fmt.Errorf("no se encontro el episodio %d de la temporada %d en las carpetas relacionadas con %s",episode,requestedSeason,item.Title)};sort.SliceStable(files,func(i,j int)bool{if files[i].SeasonRank!=files[j].SeasonRank{return files[i].SeasonRank<files[j].SeasonRank};if files[i].FileRank!=files[j].FileRank{return files[i].FileRank<files[j].FileRank};if files[i].FolderRank!=files[j].FolderRank{return files[i].FolderRank<files[j].FolderRank};return files[i].Path<files[j].Path});return files[0].Path,item,nil}

const localEpisodeBridge = `<script>(function(){
function playerBox(){var box=document.querySelector('.mirror-player-blocked,.mirror-online-player,.mirror-local-player');if(box)return box;var frame=document.querySelector('main iframe');if(frame){box=document.createElement('div');frame.replaceWith(box);return box}return null}
function buttons(){return Array.prototype.slice.call(document.querySelectorAll('button')).filter(function(b){var t=(b.textContent||'').trim();return t&&t.length<30})}
function byText(name){var n=(name||'').toLowerCase();return buttons().find(function(b){return (b.textContent||'').trim().toLowerCase()===n})}
function setup(slug,ep,local,players){var hls=byText('HLS'),upn=byText('UPNShare');if(upn)upn.style.display='none';var map={};(players||[]).forEach(function(p){map[(p.server||'').toLowerCase()]=p});var activeClass=hls?hls.className:'',inactiveClass='';buttons().some(function(b){var t=(b.textContent||'').trim().toLowerCase();if(map[t]&&(!hls||b!==hls)){inactiveClass=b.className;return true}return false});if(!inactiveClass)inactiveClass=activeClass;var localBtn=null;if(local&&local.available&&hls){localBtn=hls;localBtn.textContent='Local';localBtn.style.display='';localBtn.title=local.browser_playable?'Reproducir archivo local':'Archivo local disponible pero no reproducible por el navegador';localBtn.disabled=!local.browser_playable}else if(hls){hls.style.display='none'}
var box=playerBox();if(!box){box=document.createElement('div');var main=document.querySelector('main')||document.body;main.insertBefore(box,main.firstChild)}
function setActive(name){Object.keys(map).forEach(function(k){var b=byText(map[k].server);if(b){b.className=inactiveClass;b.removeAttribute('aria-current')}});if(localBtn){localBtn.className=inactiveClass;localBtn.removeAttribute('aria-current')}var b=name==='local'?localBtn:byText((map[name]||{}).server);if(b){b.className=activeClass;b.setAttribute('aria-current','true')}}
function showRemote(key){var p=map[key];if(!p)return;box.className='mirror-online-player';box.innerHTML='';var f=document.createElement('iframe');f.allow='autoplay; fullscreen; picture-in-picture';f.allowFullscreen=true;f.referrerPolicy='origin';f.src=p.url;box.appendChild(f);setActive(key)}
function showLocal(){if(!local||!local.browser_playable)return;box.className='mirror-local-player';box.innerHTML='';var v=document.createElement('video');v.controls=true;v.preload='metadata';v.playsInline=true;v.src=local.video_url;v.style.width='100%';v.style.maxHeight='75vh';box.appendChild(v);setActive('local')}
Object.keys(map).forEach(function(k){var b=byText(map[k].server);if(!b)return;b.addEventListener('click',function(e){e.preventDefault();e.stopImmediatePropagation();showRemote(k)},true)});if(localBtn){localBtn.addEventListener('click',function(e){e.preventDefault();e.stopImmediatePropagation();showLocal()},true)}
if(local&&local.browser_playable){showLocal()}else{var keys=Object.keys(map);if(local&&local.available&&!local.browser_playable){var n=document.createElement('div');n.className='mirror-local-note';n.textContent='Archivo local disponible: '+local.file_name+'. El navegador no admite este formato.';box.appendChild(n)}if(keys.length)showRemote(keys[0])}}
function start(){var m=location.pathname.match(/^\/media\/([^/]+)\/(\d+)\/?$/);if(!m)return;var slug=m[1],ep=parseInt(m[2],10),localReq=fetch('/api/local-episode?slug='+encodeURIComponent(slug)+'&episode='+ep,{cache:'no-store'}).then(function(r){if(!r.ok)return null;return r.json()}).catch(function(){return null}),remoteReq=fetch('/api/online-player?slug='+encodeURIComponent(slug)+'&episode='+ep,{cache:'no-store'}).then(function(r){if(!r.ok)return [];return r.json()}).catch(function(){return []});Promise.all([localReq,remoteReq]).then(function(v){setup(slug,ep,v[0],Array.isArray(v[1])?v[1]:[])})}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start);else start();})();</script><style>.mirror-local-player,.mirror-online-player{width:100%;background:#101116;border-radius:10px;overflow:hidden;margin:12px 0}.mirror-online-player iframe{display:block;border:0;width:100%;aspect-ratio:16/9;min-height:360px}.mirror-local-note{padding:8px 12px;color:#c9cbd3}</style>`
