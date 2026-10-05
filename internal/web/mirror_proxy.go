package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *Server) initMirrorResourceCounter() {
	raw := strings.TrimSpace(s.db.GetSetting("mirror_resource_count"))
	if raw == "" {
		s.mirrorResourceCountCache.Store(0)
		_ = s.db.SetSetting("mirror_resource_count", "0")
		go s.recountMirrorResources()
	} else if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		s.mirrorResourceCountCache.Store(int64(n))
	}
	go s.watchMirrorResourceCounter()
}

func (s *Server) mirrorResourceCount() int {
	n := s.mirrorResourceCountCache.Load()
	if n < 0 { return 0 }
	return int(n)
}

func (s *Server) recountMirrorResources() int {
	root := filepath.Clean(s.siteRoot)
	total := 0
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() { return nil }
		if strings.HasPrefix(info.Name(), ".frontend-cache-") { return nil }
		total++
		return nil
	})
	s.mirrorResourceCountCache.Store(int64(total))
	_ = s.db.SetSetting("mirror_resource_count", strconv.Itoa(total))
	return total
}

func (s *Server) watchMirrorResourceCounter() {
	t := time.NewTicker(4 * time.Second)
	defer t.Stop()
	wasRunning := false
	for range t.C {
		running := s.mirror.Snapshot().Running
		if wasRunning && !running { s.recountMirrorResources() }
		wasRunning = running
	}
}

func copyRecorder(w http.ResponseWriter, rr *httptest.ResponseRecorder) {
	res := rr.Result()
	defer res.Body.Close()
	for k, vv := range res.Header { for _, v := range vv { w.Header().Add(k, v) } }
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}

func patchMirrorHTML(body []byte) []byte {
	body = bytes.ReplaceAll(body, []byte("img-src 'self' data: blob:;"), []byte("img-src 'self' data: blob: https://cdn.animeav1.com;"))
	body = bytes.ReplaceAll(body, []byte("frame-src 'self';"), []byte("frame-src 'self' https:;"))
	return body
}

func archiveVersionBridge(version string) []byte {
	label := "v" + strings.TrimSpace(version)
	q, _ := json.Marshal(label)
	return []byte(`<script id="mirror-app-version-script">(function(){var label=` + string(q) + `;
function putVersion(){if(document.getElementById('mirror-app-version'))return;var xs=Array.prototype.slice.call(document.querySelectorAll('p,span,div,small')).filter(function(e){return (e.textContent||'').replace(/\s+/g,' ').trim()==='By fans for fans'});xs.sort(function(a,b){return (a.children.length-b.children.length)||((a.textContent||'').length-(b.textContent||'').length)});var t=xs[0];if(!t)return;var v=document.createElement('span');v.id='mirror-app-version';v.textContent=label;t.appendChild(v);t.setAttribute('data-mirror-fans-tagline','1')}
function fmtTime(n){if(!isFinite(n)||n<0)n=0;var m=Math.floor(n/60),s=Math.floor(n%60);return m+':'+(s<10?'0':'')+s}
function btn(label,title,cls){var b=document.createElement('button');b.type='button';b.className='mirror-jw-btn '+(cls||'');b.setAttribute('aria-label',title);b.setAttribute('title',title);b.innerHTML=label;return b}
function fullscreenElement(){return document.fullscreenElement||document.webkitFullscreenElement||null}
function enhanceLocal(){var player=document.querySelector('.mirror-local-player');if(!player)return;var v=player.querySelector('video');if(!v||v.dataset.mirrorJW==='1')return;v.dataset.mirrorJW='1';v.controls=false;player.classList.add('mirror-jw-player');
 var ui=document.createElement('div');ui.className='mirror-jw-ui mirror-jw-visible';
 var center=btn('▶','Reproducir','mirror-jw-center');ui.appendChild(center);
 var controls=document.createElement('div');controls.className='mirror-jw-controls';
 var progress=document.createElement('input');progress.type='range';progress.min='0';progress.max='1000';progress.value='0';progress.step='1';progress.className='mirror-jw-progress';progress.setAttribute('aria-label','Progreso');controls.appendChild(progress);
 var row=document.createElement('div');row.className='mirror-jw-row';var left=document.createElement('div');left.className='mirror-jw-left';var right=document.createElement('div');right.className='mirror-jw-right';
 var play=btn('▶','Reproducir','mirror-jw-play'),back=btn('−10','Retroceder 10 segundos','mirror-jw-skip'),forward=btn('+10','Avanzar 10 segundos','mirror-jw-skip'),mute=btn('🔊','Silenciar','mirror-jw-volume');var time=document.createElement('span');time.className='mirror-jw-time';time.textContent='0:00 / 0:00';left.appendChild(play);left.appendChild(back);left.appendChild(forward);left.appendChild(mute);left.appendChild(time);
 var pip=btn('▣','Imagen en imagen','mirror-jw-pip'),settings=btn('⚙','Velocidad','mirror-jw-settings'),fs=btn('⛶','Pantalla completa','mirror-jw-fullscreen');if(!document.pictureInPictureEnabled)pip.style.display='none';right.appendChild(pip);right.appendChild(settings);right.appendChild(fs);row.appendChild(left);row.appendChild(right);controls.appendChild(row);
 var menu=document.createElement('div');menu.className='mirror-jw-speed-menu';menu.innerHTML='<div class="mirror-jw-speed-title">Velocidad</div>';[0.5,1,1.25,1.5].forEach(function(speed){var x=btn(String(speed).replace('.0','')+'x','Velocidad '+speed+'x','mirror-jw-speed');x.dataset.speed=String(speed);if(speed===1)x.classList.add('active');x.onclick=function(e){e.stopPropagation();v.playbackRate=speed;Array.prototype.slice.call(menu.querySelectorAll('.mirror-jw-speed')).forEach(function(z){z.classList.toggle('active',z===x)});menu.classList.remove('open')};menu.appendChild(x)});ui.appendChild(menu);ui.appendChild(controls);player.appendChild(ui);
 function sync(){var d=v.duration||0,c=v.currentTime||0;time.textContent=fmtTime(c)+' / '+fmtTime(d);if(d>0&&!progress.matches(':active'))progress.value=String(Math.round(c/d*1000));play.innerHTML=v.paused?'▶':'❚❚';center.innerHTML=v.paused?'▶':'❚❚';mute.innerHTML=(v.muted||v.volume===0)?'🔇':'🔊'}
 function toggle(){if(v.paused){var p=v.play();if(p&&p.catch)p.catch(function(){})}else v.pause()}
 play.onclick=function(e){e.stopPropagation();toggle()};center.onclick=function(e){e.stopPropagation();toggle()};back.onclick=function(e){e.stopPropagation();v.currentTime=Math.max(0,(v.currentTime||0)-10)};forward.onclick=function(e){e.stopPropagation();v.currentTime=Math.min(v.duration||Infinity,(v.currentTime||0)+10)};mute.onclick=function(e){e.stopPropagation();v.muted=!v.muted};progress.oninput=function(){if(v.duration)v.currentTime=(Number(progress.value)/1000)*v.duration};settings.onclick=function(e){e.stopPropagation();menu.classList.toggle('open')};fs.onclick=function(e){e.stopPropagation();if(fullscreenElement()){var exit=document.exitFullscreen||document.webkitExitFullscreen;if(exit)exit.call(document)}else{var req=player.requestFullscreen||player.webkitRequestFullscreen;if(req)req.call(player)}};pip.onclick=function(e){e.stopPropagation();if(document.pictureInPictureElement){document.exitPictureInPicture().catch(function(){})}else if(v.requestPictureInPicture){v.requestPictureInPicture().catch(function(){})}};
 v.addEventListener('click',toggle);v.addEventListener('play',sync);v.addEventListener('pause',sync);v.addEventListener('timeupdate',sync);v.addEventListener('durationchange',sync);v.addEventListener('volumechange',sync);v.addEventListener('ratechange',sync);
 var hideTimer=null;function showControls(){ui.classList.add('mirror-jw-visible');clearTimeout(hideTimer);if(!v.paused)hideTimer=setTimeout(function(){ui.classList.remove('mirror-jw-visible');menu.classList.remove('open')},2600)}player.addEventListener('mousemove',showControls);player.addEventListener('touchstart',showControls,{passive:true});player.addEventListener('mouseleave',function(){if(!v.paused)ui.classList.remove('mirror-jw-visible')});v.addEventListener('pause',showControls);v.addEventListener('play',showControls);player.addEventListener('dblclick',function(e){if(e.target===v)fs.click()});sync();showControls()}
function start(){putVersion();enhanceLocal();new MutationObserver(function(){putVersion();enhanceLocal()}).observe(document.documentElement,{subtree:true,childList:true})}if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start,{once:true});else start()})();</script><style>
[data-mirror-fans-tagline="1"]{position:relative!important;display:inline-block!important;white-space:nowrap!important;overflow:visible!important}
[data-mirror-fans-tagline="1"] #mirror-app-version{position:absolute!important;left:0!important;top:calc(100% + 4px)!important;display:block!important;width:max-content!important;font-size:12px!important;line-height:1.4!important;opacity:.62;white-space:nowrap!important}
.mirror-jw-player{position:relative!important;background:#000!important;overflow:hidden!important}.mirror-jw-player>video{display:block!important;width:100%!important;max-height:75vh!important;background:#000!important}.mirror-jw-ui{position:absolute;inset:0;z-index:15;color:#fff;pointer-events:none;background:linear-gradient(to bottom,rgba(0,0,0,0) 55%,rgba(0,0,0,.62) 100%);opacity:0;transition:opacity .18s ease}.mirror-jw-ui.mirror-jw-visible{opacity:1}.mirror-jw-btn{pointer-events:auto;border:0!important;background:rgba(24,24,24,.72)!important;color:#fff!important;border-radius:999px!important;min-width:42px;height:42px;padding:0 11px!important;font-size:17px!important;font-weight:600!important;display:inline-flex!important;align-items:center!important;justify-content:center!important;cursor:pointer!important}.mirror-jw-btn:hover{background:rgba(50,50,50,.9)!important}.mirror-jw-center{position:absolute!important;left:50%;top:50%;transform:translate(-50%,-50%);width:72px!important;height:72px!important;font-size:31px!important;background:rgba(15,15,15,.72)!important}.mirror-jw-controls{position:absolute;left:0;right:0;bottom:0;padding:0 14px 12px;pointer-events:none}.mirror-jw-progress{pointer-events:auto;width:100%;height:4px;accent-color:#37d7c6;cursor:pointer;margin:0 0 8px}.mirror-jw-row{display:flex;align-items:center;justify-content:space-between;gap:8px}.mirror-jw-left,.mirror-jw-right{display:flex;align-items:center;gap:7px}.mirror-jw-time{font-size:15px;white-space:nowrap;text-shadow:0 1px 2px #000}.mirror-jw-speed-menu{pointer-events:auto;display:none;position:absolute;right:58px;bottom:62px;min-width:150px;padding:8px;background:rgba(22,22,22,.94);border-radius:10px;box-shadow:0 4px 22px rgba(0,0,0,.4)}.mirror-jw-speed-menu.open{display:block}.mirror-jw-speed-title{padding:5px 9px 8px;font-size:13px;opacity:.7}.mirror-jw-speed-menu .mirror-jw-speed{display:flex!important;width:100%;height:36px!important;border-radius:7px!important;justify-content:flex-start!important;background:transparent!important}.mirror-jw-speed-menu .mirror-jw-speed.active{color:#43e3d1!important;background:rgba(67,227,209,.12)!important}.mirror-jw-player:fullscreen,.mirror-jw-player:-webkit-full-screen{width:100vw!important;height:100vh!important;max-height:none!important;border-radius:0!important;margin:0!important;display:flex!important;align-items:center!important;justify-content:center!important}.mirror-jw-player:fullscreen>video,.mirror-jw-player:-webkit-full-screen>video{width:100vw!important;height:100vh!important;max-height:none!important;object-fit:contain!important}.mirror-jw-player:fullscreen .mirror-jw-controls,.mirror-jw-player:-webkit-full-screen .mirror-jw-controls{padding:0 24px 18px}
@media(max-width:600px){.mirror-jw-btn{min-width:36px;height:36px;padding:0 8px!important;font-size:14px!important}.mirror-jw-center{width:60px!important;height:60px!important;font-size:27px!important}.mirror-jw-controls{padding:0 8px 8px}.mirror-jw-left,.mirror-jw-right{gap:4px}.mirror-jw-time{font-size:12px}.mirror-jw-speed-menu{right:44px;bottom:54px}.mirror-jw-progress{margin-bottom:5px}}
</style>`)
}

func injectArchiveVersion(body []byte, version string) []byte {
	bridge := archiveVersionBridge(version)
	if i := bytes.LastIndex(bytes.ToLower(body), []byte("</body>")); i >= 0 {
		out := make([]byte, 0, len(body)+len(bridge))
		out = append(out, body[:i]...)
		out = append(out, bridge...)
		out = append(out, body[i:]...)
		return out
	}
	return append(body, bridge...)
}

func isBrandAsset(path string) bool {
	switch path {
	case "/img/logo.svg", "/img/logo-dark.svg", "/img/logo-ft.svg", "/img/logo-ft-dark.svg": return true
	default: return false
	}
}

func (s *Server) serveBrandAsset(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(s.static, "img", filepath.Base(r.URL.Path))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-AnimeAV1-Source", "bundled")
	http.ServeFile(w, r, path)
}

func (s *Server) siteMirror(w http.ResponseWriter, r *http.Request) {
	if isBrandAsset(r.URL.Path) { s.serveBrandAsset(w, r); return }
	if shouldProxyAnimeAV1(r) { s.proxyAnimeAV1(w, r); return }
	rr := httptest.NewRecorder()
	s.mirror.Handler().ServeHTTP(rr, r)
	res := rr.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	for k, vv := range res.Header { for _, v := range vv { w.Header().Add(k, v) } }
	if strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "text/html") {
		body = patchMirrorHTML(body)
		body = injectArchiveVersion(body, s.version)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}
	w.WriteHeader(res.StatusCode)
	if r.Method != http.MethodHead { _, _ = w.Write(body) }
}

func (s *Server) cdnResource(w http.ResponseWriter, r *http.Request) {
	rr := httptest.NewRecorder()
	s.mirror.Handler().ServeHTTP(rr, r)
	if rr.Code >= 200 && rr.Code < 400 {
		w.Header().Set("X-AnimeAV1-Source", "mirror")
		copyRecorder(w, rr)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/_cdn/")
	if p == "" { http.NotFound(w, r); return }
	u := &url.URL{Scheme:"https", Host:"cdn.animeav1.com", Path:"/"+p, RawQuery:r.URL.RawQuery}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-AnimeAV1-Source", "cdn-fallback")
	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
}
