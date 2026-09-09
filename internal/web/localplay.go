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

var episodePathRE=regexp.MustCompile(`^/media/([^/]+)/(\d+)/?$`)

type localEpisodeInfo struct {
	Available bool `json:"available"`
	VideoURL string `json:"video_url,omitempty"`
	FileName string `json:"file_name,omitempty"`
	BrowserPlayable bool `json:"browser_playable"`
	Seen bool `json:"seen"`
	SeenThrough int `json:"seen_through"`
	Episode int `json:"episode"`
	Title string `json:"title,omitempty"`
}

func (s *Server) mediaMirror(w http.ResponseWriter,r *http.Request){
	m:=episodePathRE.FindStringSubmatch(r.URL.Path)
	if len(m)!=3{s.mirror.Handler().ServeHTTP(w,r);return}
	rr:=httptest.NewRecorder();s.mirror.Handler().ServeHTTP(rr,r)
	res:=rr.Result();defer res.Body.Close()
	for k,v:=range res.Header{for _,x:=range v{w.Header().Add(k,x)}}
	body:=rr.Body.Bytes()
	if res.StatusCode>=200&&res.StatusCode<300&&strings.Contains(strings.ToLower(res.Header.Get("Content-Type")),"text/html"){
		bridge:=[]byte(localEpisodeBridge)
		if i:=bytes.LastIndex(bytes.ToLower(body),[]byte("</body>"));i>=0{body=append(append(append([]byte{},body[:i]...),bridge...),body[i:]...)}else{body=append(body,bridge...)}
		w.Header().Set("Content-Length",strconv.Itoa(len(body)))
	}
	w.WriteHeader(res.StatusCode);if r.Method!=http.MethodHead{_,_=w.Write(body)}
}

func (s *Server) localEpisodeAPI(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{http.Error(w,"method not allowed",405);return}
	slug:=strings.TrimSpace(r.URL.Query().Get("slug"));episode,_:=strconv.Atoi(r.URL.Query().Get("episode"));if slug==""||episode<1{http.Error(w,"bad request",400);return}
	info,err:=s.localEpisode(slug,episode);if err!=nil{http.Error(w,err.Error(),404);return}
	w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(info)
}

func (s *Server) localVideo(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet&&r.Method!=http.MethodHead{http.Error(w,"method not allowed",405);return}
	parts:=strings.Split(strings.TrimPrefix(r.URL.Path,"/api/local-video/"),"/");if len(parts)!=2{http.NotFound(w,r);return}
	episode,err:=strconv.Atoi(parts[1]);if err!=nil||episode<1{http.NotFound(w,r);return}
	path,_,err:=s.findLocalEpisodeFile(parts[0],episode);if err!=nil{http.NotFound(w,r);return}
	if c:=mime.TypeByExtension(filepath.Ext(path));c!=""{w.Header().Set("Content-Type",c)}
	w.Header().Set("Cache-Control","private, max-age=3600")
	http.ServeFile(w,r,path)
}

func (s *Server) markWatched(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return}
	if err:=r.ParseForm();err!=nil{http.Error(w,err.Error(),400);return}
	slug:=strings.TrimSpace(r.FormValue("slug"));episode,_:=strconv.Atoi(r.FormValue("episode"));if slug==""||episode<1{http.Error(w,"bad request",400);return}
	items:=s.cachedAV1();var item *animeav1.Item
	for i:=range items{if items[i].Slug==slug{item=&items[i];break}}
	if item==nil{http.Error(w,"serie no encontrada en las listas de AnimeAV1",404);return}
	cookie:=s.db.GetSetting("animeav1_session_cookie");ctx,cancel:=context.WithTimeout(r.Context(),35*time.Second);defer cancel()
	if err:=s.av1.MarkWatched(ctx,cookie,item.MediaID,episode);err!=nil{http.Error(w,err.Error(),502);return}
	fresh,err:=s.av1.Library(ctx,cookie);if err==nil{if b,e:=json.Marshal(fresh);e==nil{_=s.db.SetSetting("animeav1_library_json",string(b));_=s.db.SetSetting("animeav1_library_updated",time.Now().Format(time.RFC3339))}}
	w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"ok":true}`))
}

func (s *Server) cachedAV1()[]animeav1.Item{var items []animeav1.Item;if raw:=strings.TrimSpace(s.db.GetSetting("animeav1_library_json"));raw!=""{_ = json.Unmarshal([]byte(raw),&items)};return items}

func (s *Server) localEpisode(slug string,episode int)(localEpisodeInfo,error){
	path,item,err:=s.findLocalEpisodeFile(slug,episode);if err!=nil{return localEpisodeInfo{},err}
	ext:=strings.ToLower(filepath.Ext(path));return localEpisodeInfo{Available:true,VideoURL:fmt.Sprintf("/api/local-video/%s/%d",slug,episode),FileName:filepath.Base(path),BrowserPlayable:ext==".mp4"||ext==".webm"||ext==".m4v"||ext==".mov",Seen:item.Seen>=episode,SeenThrough:item.Seen,Episode:episode,Title:item.Title},nil
}

func (s *Server) findLocalEpisodeFile(slug string,episode int)(string,animeav1.Item,error){
	items:=s.cachedAV1();var item *animeav1.Item
	for i:=range items{if items[i].Slug==slug{item=&items[i];break}}
	if item==nil{return "",animeav1.Item{},fmt.Errorf("serie no encontrada")}
	lib,err:=s.db.Library();if err!=nil{return "",*item,err}
	folders:=localFolderCandidates(*item,lib);if len(folders)==0{return "",*item,fmt.Errorf("serie sin carpetas locales asociadas")}
	var files []episodeFileCandidate
	for _,folder:=range folders{
		root:=filepath.Clean(folder.Item.Path)
		_ = filepath.Walk(root,func(path string,info os.FileInfo,e error)error{
			if e!=nil||info==nil||info.IsDir(){return nil}
			ext:=strings.ToLower(filepath.Ext(info.Name()));switch ext{case ".mkv",".mp4",".avi",".webm",".m4v",".mov":default:return nil}
			fr:=episodeFileRank(info.Name(),item.MediaID,episode);if fr<99{files=append(files,episodeFileCandidate{Path:path,FolderRank:folder.Rank,FileRank:fr})}
			return nil
		})
	}
	if len(files)==0{return "",*item,fmt.Errorf("no se encontro el episodio %d en las carpetas relacionadas con %s",episode,item.Title)}
	sort.SliceStable(files,func(i,j int)bool{if files[i].FileRank!=files[j].FileRank{return files[i].FileRank<files[j].FileRank};if files[i].FolderRank!=files[j].FolderRank{return files[i].FolderRank<files[j].FolderRank};return files[i].Path<files[j].Path})
	return files[0].Path,*item,nil
}

const localEpisodeBridge = `<script>(function(){
function start(){var m=location.pathname.match(/^\/media\/([^/]+)\/(\d+)\/?$/);if(!m)return;var slug=m[1],ep=parseInt(m[2],10);fetch('/api/local-episode?slug='+encodeURIComponent(slug)+'&episode='+ep).then(function(r){if(!r.ok)throw new Error('sin copia local');return r.json()}).then(function(x){if(!x.available)return;var box=document.querySelector('.mirror-player-blocked');if(!box){box=document.createElement('div');var main=document.querySelector('main')||document.body;main.insertBefore(box,main.firstChild)}box.className='mirror-local-player';box.innerHTML='';var v=document.createElement('video');v.controls=true;v.preload='metadata';v.playsInline=true;v.src=x.video_url;v.style.width='100%';v.style.maxHeight='75vh';box.appendChild(v);var bar=document.createElement('div');bar.className='mirror-watch-bar';var b=document.createElement('button');b.type='button';b.textContent=x.seen?'Visto en AnimeAV1':'Marcar episodio '+ep+' como visto';b.disabled=!!x.seen;b.onclick=function(){b.disabled=true;b.textContent='Actualizando AnimeAV1...';var f=new URLSearchParams();f.set('slug',slug);f.set('episode',String(ep));fetch('/api/av1/watched',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:f.toString()}).then(function(r){if(!r.ok)return r.text().then(function(t){throw new Error(t||'error')});b.textContent='Visto en AnimeAV1'}).catch(function(e){b.disabled=false;b.textContent='Reintentar marcar como visto';alert('AnimeAV1: '+e.message)})};bar.appendChild(b);if(!x.browser_playable){var n=document.createElement('span');n.textContent=' El navegador puede no reproducir '+x.file_name+' directamente.';bar.appendChild(n)}box.appendChild(bar)}).catch(function(){})}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start);else start();
})();</script><style>.mirror-local-player{width:100%;background:#101116;border-radius:10px;overflow:hidden;margin:12px 0}.mirror-watch-bar{display:flex;gap:12px;align-items:center;padding:10px 12px;color:#c9cbd3}.mirror-watch-bar button{padding:8px 12px;border:0;border-radius:8px;cursor:pointer;font-weight:600}.mirror-watch-bar button:disabled{opacity:.65;cursor:default}</style>`
