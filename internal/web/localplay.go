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

func (s *Server) localEpisodeAPI(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodGet{http.Error(w,"method not allowed",405);return};slug:=strings.TrimSpace(r.URL.Query().Get("slug"));episode,_:=strconv.Atoi(r.URL.Query().Get("episode"));if slug==""||episode<0{http.Error(w,"bad request",400);return};info,err:=s.localEpisode(slug,episode);if err!=nil{http.Error(w,err.Error(),404);return};w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(info)}

func (s *Server) localVideo(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodGet&&r.Method!=http.MethodHead{http.Error(w,"method not allowed",405);return};parts:=strings.Split(strings.TrimPrefix(r.URL.Path,"/api/local-video/"),"/");if len(parts)!=2{http.NotFound(w,r);return};episode,err:=strconv.Atoi(parts[1]);if err!=nil||episode<0{http.NotFound(w,r);return};path,_,err:=s.findLocalEpisodeFile(parts[0],episode);if err!=nil{http.NotFound(w,r);return};if c:=mime.TypeByExtension(filepath.Ext(path));c!=""{w.Header().Set("Content-Type",c)};w.Header().Set("Cache-Control","private, max-age=3600");w.Header().Set("X-AnimeAV1-Source","local-video");http.ServeFile(w,r,path)}

func (s *Server) markWatched(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};if err:=r.ParseForm();err!=nil{http.Error(w,err.Error(),400);return};slug:=strings.TrimSpace(r.FormValue("slug"));episode,_:=strconv.Atoi(r.FormValue("episode"));if slug==""||episode<0{http.Error(w,"bad request",400);return};items:=s.cachedAV1();var item *animeav1.Item;for i:=range items{if items[i].Slug==slug{item=&items[i];break}};if item==nil{http.Error(w,"serie no encontrada en las listas de AnimeAV1",404);return};ctx,cancel:=context.WithTimeout(r.Context(),35*time.Second);defer cancel();if err:=s.av1.MarkWatched(ctx,s.db.GetSetting("animeav1_session_cookie"),item.MediaID,episode);err!=nil{http.Error(w,err.Error(),502);return};go s.refreshAnimeAV1Cache();w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"ok":true}`))}

func (s *Server) cachedAV1()[]animeav1.Item{var items []animeav1.Item;if raw:=strings.TrimSpace(s.db.GetSetting("animeav1_library_json"));raw!=""{_=json.Unmarshal([]byte(raw),&items)};return items}

func (s *Server) localEpisode(slug string,episode int)(localEpisodeInfo,error){path,item,err:=s.findLocalEpisodeFile(slug,episode);if err!=nil{return localEpisodeInfo{},err};ext:=strings.ToLower(filepath.Ext(path));return localEpisodeInfo{Available:true,VideoURL:fmt.Sprintf("/api/local-video/%s/%d",slug,episode),FileName:filepath.Base(path),BrowserPlayable:ext==".mp4"||ext==".webm"||ext==".m4v"||ext==".mov",Seen:item.Seen>=episode,SeenThrough:item.Seen,Episode:episode,Title:item.Title},nil}

func (s *Server) itemForSlug(slug string)(animeav1.Item,bool){for _,it:=range s.cachedAV1(){if it.Slug==slug{return it,true}};var title string;if err:=s.db.QueryRow(`SELECT title FROM anime WHERE slug=?`,slug).Scan(&title);err==nil&&strings.TrimSpace(title)!=""{return animeav1.Item{Slug:slug,Title:title,Aliases:map[string]string{}},true};return animeav1.Item{},false}

func legacyEpisodeFileRank(name string,episode int)int{base:=strings.TrimSuffix(filepath.Base(name),filepath.Ext(name));re:=regexp.MustCompile(fmt.Sprintf(`(?i)^\d+[_ .-]+0*%d(?:[_ .-]|$)`,episode));if re.MatchString(base){return 2};return 99}

func (s *Server) findLocalEpisodeFile(slug string,episode int)(string,animeav1.Item,error){item,ok:=s.itemForSlug(slug);if !ok{return "",animeav1.Item{},fmt.Errorf("serie no encontrada")};lib,err:=s.db.Library();if err!=nil{return "",item,err};folders:=localFolderCandidates(item,lib);if len(folders)==0{return "",item,fmt.Errorf("serie sin carpetas locales asociadas")};requestedSeason:=itemSeasonNumber(item);var files []episodeFileCandidate;for _,folder:=range folders{root:=filepath.Clean(folder.Item.Path);_=filepath.Walk(root,func(path string,info os.FileInfo,e error)error{if e!=nil||info==nil||info.IsDir(){return nil};ext:=strings.ToLower(filepath.Ext(info.Name()));switch ext{case ".mkv",".mp4",".avi",".webm",".m4v",".mov":default:return nil};fr:=episodeFileRank(info.Name(),item.MediaID,episode);if fr>=99{fr=legacyEpisodeFileRank(info.Name(),episode)};if fr>=99{return nil};sr:=seasonRankForPath(path,requestedSeason);if sr>=9{return nil};files=append(files,episodeFileCandidate{Path:path,FolderRank:folder.Rank,FileRank:fr,SeasonRank:sr});return nil})};if len(files)==0{return "",item,fmt.Errorf("no se encontro el episodio %d de la temporada %d en las carpetas relacionadas con %s",episode,requestedSeason,item.Title)};sort.SliceStable(files,func(i,j int)bool{if files[i].SeasonRank!=files[j].SeasonRank{return files[i].SeasonRank<files[j].SeasonRank};if files[i].FileRank!=files[j].FileRank{return files[i].FileRank<files[j].FileRank};if files[i].FolderRank!=files[j].FolderRank{return files[i].FolderRank<files[j].FolderRank};return files[i].Path<files[j].Path});return files[0].Path,item,nil}

const localEpisodeBridge = `<script>(function(){
function allButtons(){return Array.prototype.slice.call(document.querySelectorAll('button')).filter(function(b){var t=(b.textContent||'').trim();return t&&t.length<30})}
function providerName(b){return (b.textContent||'').trim()}
function providerButtons(name){var n=(name||'').toLowerCase();return allButtons().filter(function(b){return providerName(b).toLowerCase()===n})}
function rowForButton(b){var p=b;for(var i=0;p&&i<6;i++,p=p.parentElement){var t=(p.textContent||'').replace(/\s+/g,' ').trim();if(/(^|\s)SUB(\s|$)/i.test(t)||/(^|\s)DUB(\s|$)/i.test(t))return p}return b.parentElement}
function languageOfRow(row){var t=((row&&row.textContent)||'').replace(/\s+/g,' ');if(/(^|\s)SUB(\s|$)/i.test(t))return 'sub';if(/(^|\s)DUB(\s|$)/i.test(t))return 'dub';return ''}
function findRows(){var out={sub:null,dub:null};allButtons().forEach(function(b){var n=providerName(b).toLowerCase();if(['hls','upnshare','voe','byse','mega','mp4upload'].indexOf(n)<0)return;var r=rowForButton(b),lang=languageOfRow(r);if(lang&&!out[lang])out[lang]=r});return out}
function mediaArea(el){var r=el.getBoundingClientRect();return r.width>=200&&r.height>=120?r.width*r.height:0}
function playerCandidate(){var old=document.querySelector('.mirror-shared-player');if(old)return old;var candidates=Array.prototype.slice.call(document.querySelectorAll('main iframe,main video,iframe,video')),best=null,bestArea=0;candidates.forEach(function(el){var area=mediaArea(el);if(area>bestArea){best=el;bestArea=area}});if(best){var box=document.createElement('div');box.className='mirror-shared-player';best.replaceWith(box);return box}var main=document.querySelector('main')||document.body,box=document.createElement('div');box.className='mirror-shared-player';main.insertBefore(box,main.firstChild);return box}
function removeDuplicatePlayers(shared){Array.prototype.slice.call(document.querySelectorAll('.mirror-local-player,.mirror-online-player,.mirror-shared-player')).forEach(function(x){if(x!==shared)x.remove()});Array.prototype.slice.call(document.querySelectorAll('iframe,video')).forEach(function(x){if(!shared.contains(x)&&mediaArea(x)>0)x.remove()})}
function watchDuplicatePlayers(shared){if(!document.body||shared.dataset.mirrorWatch==='1')return;shared.dataset.mirrorWatch='1';var scheduled=false;new MutationObserver(function(){if(scheduled)return;scheduled=true;requestAnimationFrame(function(){scheduled=false;removeDuplicatePlayers(shared)})}).observe(document.body,{subtree:true,childList:true})}
function setup(slug,ep,local,players){
 var rows=findRows(),subRow=rows.sub,dubRow=rows.dub;
 providerButtons('UPNShare').forEach(function(b){b.style.display='none'});
 var subHLS=providerButtons('HLS').find(function(b){return subRow&&subRow.contains(b)}),dubHLS=providerButtons('HLS').find(function(b){return dubRow&&dubRow.contains(b)});
 if(dubHLS)dubHLS.style.display='none';
 var ref=subHLS||providerButtons('Voe').find(function(b){return subRow&&subRow.contains(b)})||providerButtons('Voe')[0],activeClass=ref?ref.className:'',inactiveClass='';
 allButtons().some(function(b){var n=providerName(b).toLowerCase();if(['voe','byse','mega','mp4upload'].indexOf(n)>=0&&b!==ref){inactiveClass=b.className;return true}return false});if(!inactiveClass)inactiveClass=activeClass;
 var localBtn=null;if(local&&local.available){if(subHLS){localBtn=subHLS}else if(ref){localBtn=document.createElement('button');localBtn.type='button';localBtn.className=ref.className;ref.parentNode.insertBefore(localBtn,ref)}else if(subRow){localBtn=document.createElement('button');localBtn.type='button';subRow.appendChild(localBtn)}if(localBtn){localBtn.textContent='Local';localBtn.style.display='';localBtn.disabled=!local.browser_playable;localBtn.setAttribute('data-mirror-local','1');localBtn.setAttribute('title',local.file_name||'Archivo local');localBtn.setAttribute('aria-label',local.browser_playable?'Reproducir archivo local: '+(local.file_name||'episodio descargado'):'Archivo local disponible pero no reproducible por el navegador: '+(local.file_name||''))}}else if(subHLS){subHLS.style.display='none'}
 var shared=playerCandidate();removeDuplicatePlayers(shared);watchDuplicatePlayers(shared);
 var bindings=[],occ={};(players||[]).forEach(function(p){var k=(p.server||'').toLowerCase();if(!occ[k])occ[k]=0;var bs=providerButtons(p.server),b=bs[occ[k]++]||null;if(b)bindings.push({button:b,player:p})});
 function clearActive(){bindings.forEach(function(x){if(inactiveClass)x.button.className=inactiveClass;x.button.removeAttribute('aria-current')});if(localBtn){if(inactiveClass)localBtn.className=inactiveClass;localBtn.removeAttribute('aria-current')}}
 function activate(b){clearActive();if(b){if(activeClass)b.className=activeClass;b.setAttribute('aria-current','true')}}
 function remoteFullscreenElement(){return document.fullscreenElement||document.webkitFullscreenElement||document.mozFullScreenElement||document.msFullscreenElement||null}
 function exitRemoteFullscreen(){var fn=document.exitFullscreen||document.webkitExitFullscreen||document.mozCancelFullScreen||document.msExitFullscreen;if(fn){try{var p=fn.call(document);if(p&&p.catch)p.catch(function(){})}catch(e){}}}
 function toggleRemoteFullscreen(shared,f){if(remoteFullscreenElement()){exitRemoteFullscreen();return}var target=shared,fn=target.requestFullscreen||target.webkitRequestFullscreen||target.mozRequestFullScreen||target.msRequestFullscreen;if(fn){try{var p=fn.call(target);if(p&&p.catch)p.catch(function(){var ff=f.requestFullscreen||f.webkitRequestFullscreen||f.mozRequestFullScreen||f.msRequestFullscreen;if(ff)ff.call(f)})}catch(e){var ff=f.requestFullscreen||f.webkitRequestFullscreen||f.mozRequestFullScreen||f.msRequestFullscreen;if(ff)ff.call(f)}}}
 function showRemote(binding){if(!binding||!binding.player)return;shared.className='mirror-shared-player mirror-online-player';shared.innerHTML='';var f=document.createElement('iframe');f.allow='autoplay; encrypted-media; fullscreen; picture-in-picture';f.allowFullscreen=true;f.setAttribute('allowfullscreen','');f.setAttribute('webkitallowfullscreen','');f.setAttribute('mozallowfullscreen','');f.referrerPolicy='origin';f.src=binding.player.url;shared.appendChild(f);var fs=document.createElement('button');fs.type='button';var isVoe=((binding.player.server||'').toLowerCase()==='voe');fs.className=isVoe?'mirror-remote-fullscreen mirror-remote-fullscreen-voe':'mirror-remote-fullscreen';fs.setAttribute('aria-label','Pantalla completa');fs.setAttribute('title','Pantalla completa');fs.innerHTML='<span aria-hidden="true">⛶</span>';fs.onclick=function(e){e.preventDefault();e.stopPropagation();toggleRemoteFullscreen(shared,f)};shared.appendChild(fs);activate(binding.button);removeDuplicatePlayers(shared)}
 function showLocal(){if(!local||!local.browser_playable)return;shared.className='mirror-shared-player mirror-local-player';shared.innerHTML='';var v=document.createElement('video');v.controls=true;v.preload='metadata';v.playsInline=true;v.src=local.video_url;v.style.width='100%';v.style.maxHeight='75vh';shared.appendChild(v);activate(localBtn);removeDuplicatePlayers(shared)}
 bindings.forEach(function(x){x.button.addEventListener('click',function(e){e.preventDefault();e.stopImmediatePropagation();showRemote(x)},true)});if(localBtn)localBtn.addEventListener('click',function(e){e.preventDefault();e.stopImmediatePropagation();showLocal()},true);
 if(local&&local.browser_playable){showLocal();return}
 if(local&&local.available&&!local.browser_playable){var n=document.createElement('div');n.className='mirror-local-note';n.textContent='Archivo local disponible: '+local.file_name+'. El navegador no admite este formato.';shared.appendChild(n)}
 var firstSub=bindings.find(function(x){return subRow&&subRow.contains(x.button)});showRemote(firstSub||bindings[0])
}
function start(){var m=location.pathname.match(/^\/media\/([^/]+)\/(\d+)\/?$/);if(!m)return;var slug=m[1],ep=parseInt(m[2],10),localReq=fetch('/api/local-episode?slug='+encodeURIComponent(slug)+'&episode='+ep,{cache:'no-store'}).then(function(r){if(!r.ok)return null;return r.json()}).catch(function(){return null}),remoteReq=fetch('/api/online-player?slug='+encodeURIComponent(slug)+'&episode='+ep,{cache:'no-store'}).then(function(r){if(!r.ok)return [];return r.json()}).catch(function(){return []});Promise.all([localReq,remoteReq]).then(function(v){setup(slug,ep,v[0],Array.isArray(v[1])?v[1]:[])})}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start);else start();})();</script><style>.mirror-shared-player,.mirror-local-player,.mirror-online-player{width:100%;background:#101116;border-radius:10px;overflow:hidden;margin:12px 0}.mirror-online-player{position:relative}.mirror-online-player iframe{display:block;border:0;width:100%;aspect-ratio:16/9;min-height:360px}.mirror-remote-fullscreen{position:absolute;right:10px;top:10px;z-index:20;width:38px;height:38px;border:1px solid rgba(255,255,255,.3);border-radius:8px;background:rgba(0,0,0,.6);color:#fff;font-size:22px;line-height:1;cursor:pointer}.mirror-remote-fullscreen-voe{top:auto;right:8px;bottom:8px;width:44px;height:44px;border:0;border-radius:4px;background:rgba(20,20,20,.88);font-size:22px}.mirror-remote-fullscreen-voe:hover{background:rgba(45,45,45,.95)}.mirror-shared-player:fullscreen,.mirror-shared-player:-webkit-full-screen{width:100vw;height:100vh;margin:0;border-radius:0;background:#000;display:flex;align-items:center;justify-content:center}.mirror-shared-player:fullscreen iframe,.mirror-shared-player:-webkit-full-screen iframe{width:100vw;height:100vh;min-height:100vh;aspect-ratio:auto}.mirror-local-note{padding:8px 12px;color:#c9cbd3}</style>`
