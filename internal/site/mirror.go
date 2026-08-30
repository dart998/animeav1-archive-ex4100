package site

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type ErrorEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

type State struct {
	Running   bool         `json:"running"`
	Started   string       `json:"started"`
	Finished  string       `json:"finished"`
	Fetched   int          `json:"fetched"`
	Errors    int          `json:"errors"`
	Sanitized int          `json:"sanitized"`
	Current   string       `json:"current"`
	LastError string       `json:"last_error"`
	ErrorLog  []ErrorEntry `json:"error_log"`
}

type Mirror struct {
	base          *url.URL
	root          string
	client        *http.Client
	mu            sync.RWMutex
	state         State
	sessionCookie string
}

var (
	attrRE       = regexp.MustCompile(`(?i)(?:href|src|poster|action)=["']([^"'#]+)["']`)
	srcsetRE     = regexp.MustCompile(`(?i)srcset=["']([^"']+)["']`)
	cssURLRE     = regexp.MustCompile(`(?i)url\(\s*["']?([^"')]+)`)
	importRE     = regexp.MustCompile(`(?i)(?:from\s*|import\s*\(\s*)["']([^"']+)["']`)
	scriptRE     = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script\s*>`)
	scriptSrcRE  = regexp.MustCompile(`(?i)\bsrc\s*=\s*["']([^"']+)["']`)
	iframeRE     = regexp.MustCompile(`(?is)<iframe\b([^>]*)>.*?</iframe\s*>|<iframe\b([^>]*)/?>`)
	iframeSrcRE  = regexp.MustCompile(`(?i)\bsrc\s*=\s*["']([^"']+)["']`)
	eventDQRE    = regexp.MustCompile(`(?is)\s+on[a-z0-9_-]+\s*=\s*"([^"]*)"`)
	eventSQRE    = regexp.MustCompile(`(?is)\s+on[a-z0-9_-]+\s*=\s*'([^']*)'`)
	navDQRE      = regexp.MustCompile(`(?is)\b(href|action|formaction)\s*=\s*"([^"]*)"`)
	navSQRE      = regexp.MustCompile(`(?is)\b(href|action|formaction)\s*=\s*'([^']*)'`)
	baseTagRE    = regexp.MustCompile(`(?is)<base\b[^>]*>`)
	headOpenRE   = regexp.MustCompile(`(?i)<head\b[^>]*>`)
	absoluteURL  = regexp.MustCompile(`(?i)https?://[a-z0-9._:-]+[^\s"'<>)]*`)
	popupJS      = regexp.MustCompile(`(?i)(window\.open\s*\(|popunder|pop[-_ ]?up|runative[-.]syndicate|onclick\s*=|onmousedown\s*=|onmouseup\s*=)`)
	badAdDomain  = regexp.MustCompile(`(?i)(runative-syndicate\.com|runative\.com|syndication|popads|onclicka|adsterra)`)
)

const localCSP = `<meta http-equiv="Content-Security-Policy" content="default-src 'self' data: blob:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; media-src 'self' blob:; frame-src 'self'; form-action 'self'; base-uri 'self'; object-src 'none'">`
const localGuard = `<script>(function(){try{
window.open=function(){return null};
var bad=function(h){if(!h)return false;if(/^javascript:/i.test(h))return true;try{return new URL(h,location.href).origin!==location.origin}catch(_){return false}};
var clean=function(root){
(root.querySelectorAll?root.querySelectorAll('a[href],form[action],iframe[src]'):[]).forEach(function(el){var h=el.getAttribute('href')||el.getAttribute('action')||el.getAttribute('src')||'';if(bad(h)){if(el.tagName==='IFRAME'){var d=document.createElement('div');d.className='mirror-player-blocked';d.textContent='Reproductor externo no disponible en la copia local';el.replaceWith(d)}else if(el.tagName==='FORM'){el.setAttribute('action','')}else{el.setAttribute('href','#')}}});
(root.querySelectorAll?root.querySelectorAll('[class*="comment" i],[id*="comment" i],.disqus,.disqus-thread,#disqus_thread'):[]).forEach(function(el){el.remove()});
};
document.addEventListener('click',function(e){var a=e.target&&e.target.closest?e.target.closest('a[href]'):null;if(a&&bad(a.getAttribute('href')||'')){e.preventDefault();e.stopImmediatePropagation()}},true);
document.addEventListener('submit',function(e){var f=e.target;if(f&&f.action&&bad(f.action)){e.preventDefault();e.stopImmediatePropagation()}},true);
clean(document);
new MutationObserver(function(ms){ms.forEach(function(m){m.addedNodes&&m.addedNodes.forEach(function(n){if(n&&n.nodeType===1)clean(n)})})}).observe(document.documentElement,{childList:true,subtree:true});
}catch(_){}})();</script><style>.mirror-player-blocked{display:flex;min-height:220px;align-items:center;justify-content:center;padding:24px;border:1px solid #343746;border-radius:10px;background:#17181f;color:#aeb4c7;text-align:center}</style>`

func New(baseURL, root string) (*Mirror, error) {
	u, err := url.Parse(baseURL)
	if err != nil { return nil, err }
	return &Mirror{base: u, root: root, client: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (m *Mirror) Snapshot() State {
	m.mu.RLock(); defer m.mu.RUnlock()
	s := m.state
	s.ErrorLog = append([]ErrorEntry(nil), m.state.ErrorLog...)
	return s
}

func (m *Mirror) SetSessionCookie(cookie string) { m.mu.Lock(); m.sessionCookie = strings.TrimSpace(cookie); m.mu.Unlock() }
func (m *Mirror) HasSessionCookie() bool { m.mu.RLock(); defer m.mu.RUnlock(); return m.sessionCookie != "" }

func (m *Mirror) Start(ctx context.Context) bool {
	m.mu.Lock()
	if m.state.Running { m.mu.Unlock(); return false }
	m.state = State{Running:true, Started:time.Now().Format(time.RFC3339), ErrorLog:[]ErrorEntry{}}
	m.mu.Unlock()
	go m.run(ctx)
	return true
}

func (m *Mirror) Handler() http.Handler { return http.HandlerFunc(m.serveHTTP) }

func (m *Mirror) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/_blocked_external" { w.WriteHeader(http.StatusNoContent); return }
	if r.Method != http.MethodGet && r.Method != http.MethodHead { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
	localPath, upstream, ok := m.requestPaths(r.URL)
	if !ok { http.NotFound(w,r); return }

	// Los ficheros de texto cacheados se vuelven a sanear al servirlos. Esto evita
	// que HTML/JS guardado por versiones antiguas conserve publicidad o popups.
	if f, found := existingFile(localPath); found {
		ctype := mime.TypeByExtension(filepath.Ext(f))
		if m.isText(ctype, f) {
			if body, err := os.ReadFile(f); err == nil {
				body, cleaned := m.prepareForPublish(upstream, body, ctype)
				if cleaned > 0 { m.addSanitized(cleaned); _ = os.WriteFile(f,body,0o644) }
				if ctype != "" { w.Header().Set("Content-Type",ctype) }
				w.Header().Set("Cache-Control","no-cache")
				if r.Method != http.MethodHead { _,_ = w.Write(body) }
				return
			}
		}
		http.ServeFile(w,r,f); return
	}

	body, ctype, err := m.fetch(r.Context(), upstream.String())
	if err != nil { m.recordError(err); http.Error(w,err.Error(),http.StatusBadGateway); return }
	body, cleaned := m.prepareForPublish(upstream,body,ctype); if cleaned>0 { m.addSanitized(cleaned) }
	if r.URL.RawQuery=="" && !strings.Contains(strings.ToLower(ctype),"application/json") { _ = m.save(m.root,upstream,body,ctype) }
	if ctype!="" { w.Header().Set("Content-Type",ctype) }; w.Header().Set("Cache-Control","no-cache")
	if r.Method!=http.MethodHead { _,_ = w.Write(body) }
}

func (m *Mirror) requestPaths(req *url.URL) (string,*url.URL,bool) {
	p:=req.Path; host:=m.base.Host; upPath:=p
	if strings.HasPrefix(p,"/_cdn/") { host="cdn.animeav1.com"; upPath="/"+strings.TrimPrefix(p,"/_cdn/") }
	u:=&url.URL{Scheme:"https",Host:host,Path:upPath,RawQuery:req.RawQuery}; if m.skipURL(u){return "",nil,false}
	local:=filepath.Join(m.root,strings.TrimPrefix(filepath.Clean(p),string(filepath.Separator))); if p=="/"{local=filepath.Join(m.root,"index.html")}
	return local,u,true
}

func existingFile(path string)(string,bool){st,e:=os.Stat(path);if e==nil&&!st.IsDir(){return path,true};if e==nil&&st.IsDir(){idx:=filepath.Join(path,"index.html");if s,x:=os.Stat(idx);x==nil&&!s.IsDir(){return idx,true}};return "",false}

func (m *Mirror) run(ctx context.Context) {
	defer func(){m.mu.Lock();m.state.Running=false;m.state.Finished=time.Now().Format(time.RFC3339);m.state.Current="";m.mu.Unlock()}()
	buildRoot:=m.root+".building"; _=os.RemoveAll(buildRoot); if err:=os.MkdirAll(buildRoot,0o755);err!=nil{m.fail(err);return}
	queue:=[]string{m.base.String()}; seen:=map[string]bool{}
	for len(queue)>0 {
		select{case<-ctx.Done():return;default:}
		raw:=queue[0];queue=queue[1:];u,err:=url.Parse(raw);if err!=nil{continue};u.Fragment=""
		if !m.allowedHost(u.Host)||m.skipURL(u){continue};key:=u.String();if seen[key]{continue};seen[key]=true
		m.mu.Lock();m.state.Current=key;m.mu.Unlock()
		body,ctype,err:=m.fetch(ctx,key);if err!=nil{m.recordError(err);continue}
		if m.isText(ctype,u.Path){original:=string(body);for _,ref:=range discoverRefs(original){if abs:=m.resolve(u,ref);abs!=""{queue=append(queue,abs)}}}
		body,n:=m.prepareForPublish(u,body,ctype);if n>0{m.addSanitized(n)}
		if err=m.save(buildRoot,u,body,ctype);err!=nil{m.recordError(err);continue}
		m.mu.Lock();m.state.Fetched++;m.mu.Unlock();time.Sleep(75*time.Millisecond)
	}
	index:=filepath.Join(buildRoot,"index.html");info,err:=os.Stat(index);if err!=nil||info.Size()<1024{m.fail(fmt.Errorf("mirror incompleto: index.html no existe o es demasiado pequeno"));return}
	oldRoot:=m.root+".old";_=os.RemoveAll(oldRoot);if _,err=os.Stat(m.root);err==nil{if err=os.Rename(m.root,oldRoot);err!=nil{m.fail(err);return}}
	if err=os.Rename(buildRoot,m.root);err!=nil{_=os.Rename(oldRoot,m.root);m.fail(err);return};_=os.RemoveAll(oldRoot)
	m.mu.Lock();m.state.LastError="";m.mu.Unlock()
}

func discoverRefs(text string)[]string{out:=[]string{};for _,re:=range []*regexp.Regexp{attrRE,cssURLRE,importRE}{for _,x:=range re.FindAllStringSubmatch(text,-1){if len(x)>1{out=append(out,strings.TrimSpace(x[1]))}}};for _,x:=range srcsetRE.FindAllStringSubmatch(text,-1){if len(x)<2{continue};for _,item:=range strings.Split(x[1],","){if f:=strings.Fields(strings.TrimSpace(item));len(f)>0{out=append(out,f[0])}}};return out}

func (m *Mirror) prepareForPublish(page *url.URL, body []byte, ctype string)([]byte,int){if !m.isText(ctype,page.Path){return body,0};text:=string(body);n:=0;if m.isHTML(ctype,page.Path){text,n=m.sanitizeHTML(page,text)};var x int;text,x=m.neutralizeExternalURLs(text);n+=x;text=m.rewrite(text);return []byte(text),n}

func (m *Mirror) sanitizeHTML(page *url.URL,text string)(string,int){
	removed:=0
	text=baseTagRE.ReplaceAllStringFunc(text,func(string)string{removed++;return ""})
	text=iframeRE.ReplaceAllStringFunc(text,func(block string)string{attrs:=block;if p:=iframeRE.FindStringSubmatch(block);len(p)>1{attrs=p[1]+p[2]};sm:=iframeSrcRE.FindStringSubmatch(attrs);if len(sm)<2{return block};r,e:=url.Parse(strings.TrimSpace(sm[1]));if e!=nil{return block};u:=page.ResolveReference(r);if (u.Scheme=="http"||u.Scheme=="https")&&!m.allowedHost(u.Host){removed++;return `<div class="mirror-player-blocked">Reproductor externo no disponible en la copia local</div>`};return block})
	text=scriptRE.ReplaceAllStringFunc(text,func(block string)string{parts:=scriptRE.FindStringSubmatch(block);if len(parts)<3{return block};attrs,body:=parts[1],parts[2];if sm:=scriptSrcRE.FindStringSubmatch(attrs);len(sm)>1{r,e:=url.Parse(strings.TrimSpace(sm[1]));if e!=nil{removed++;return ""};u:=page.ResolveReference(r);if (u.Scheme=="http"||u.Scheme=="https")&&!m.allowedHost(u.Host){removed++;return ""}};if popupJS.MatchString(body)||badAdDomain.MatchString(body){removed++;return ""};return block})
	text=sanitizeEventAttrs(text,eventDQRE,&removed);text=sanitizeEventAttrs(text,eventSQRE,&removed);text=m.sanitizeNavAttrs(page,text,navDQRE,'"',&removed);text=m.sanitizeNavAttrs(page,text,navSQRE,'\'',&removed)
	inject:=localCSP+localGuard;if headOpenRE.MatchString(text){text=headOpenRE.ReplaceAllString(text,`${0}`+inject)}else{text=inject+text};return text,removed
}

func sanitizeEventAttrs(text string,re *regexp.Regexp,removed *int)string{return re.ReplaceAllStringFunc(text,func(attr string)string{m:=re.FindStringSubmatch(attr);if len(m)>1&&(popupJS.MatchString(m[1])||badAdDomain.MatchString(m[1])){(*removed)++;return ""};return attr})}
func (m *Mirror) sanitizeNavAttrs(page *url.URL,text string,re *regexp.Regexp,quote byte,removed *int)string{return re.ReplaceAllStringFunc(text,func(attr string)string{parts:=re.FindStringSubmatch(attr);if len(parts)<3{return attr};name,raw:=parts[1],strings.TrimSpace(parts[2]);lower:=strings.ToLower(raw);q:=string(quote);if strings.HasPrefix(lower,"javascript:")||badAdDomain.MatchString(raw){(*removed)++;if strings.EqualFold(name,"href"){return name+"="+q+"#"+q};return name+"="+q+q};r,e:=url.Parse(raw);if e!=nil{return attr};u:=page.ResolveReference(r);if (u.Scheme=="http"||u.Scheme=="https")&&!m.allowedHost(u.Host){(*removed)++;if strings.EqualFold(name,"href"){return name+"="+q+"#"+q};return name+"="+q+q};return attr})}
func (m *Mirror) neutralizeExternalURLs(text string)(string,int){n:=0;text=absoluteURL.ReplaceAllStringFunc(text,func(raw string)string{u,e:=url.Parse(raw);if e!=nil||m.allowedHost(u.Host){return raw};n++;return "/_blocked_external"});return text,n}
func (m *Mirror) allowedHost(host string)bool{return strings.EqualFold(host,m.base.Host)||strings.EqualFold(host,"cdn.animeav1.com")}
func (m *Mirror) skipURL(u *url.URL)bool{p:=strings.ToLower(u.Path);for _,ext:=range []string{".m3u8",".mp4",".mkv",".webm",".avi",".mov",".ts",".mp3",".m4a",".aac"}{if strings.HasSuffix(p,ext){return true}};return strings.HasPrefix(p,"/api/video")||strings.Contains(p,"/stream/")}
func (m *Mirror) resolve(base *url.URL,ref string)string{ref=strings.TrimSpace(ref);lower:=strings.ToLower(ref);if ref==""||strings.HasPrefix(ref,"#")||strings.HasPrefix(ref,"data:")||strings.HasPrefix(ref,"blob:")||strings.HasPrefix(lower,"javascript:")||strings.HasPrefix(ref,"mailto:")||badAdDomain.MatchString(ref){return ""};r,e:=url.Parse(ref);if e!=nil{return ""};u:=base.ResolveReference(r);if (u.Scheme!="http"&&u.Scheme!="https")||!m.allowedHost(u.Host)||m.skipURL(u){return ""};return u.String()}
func (m *Mirror) rewrite(text string)string{for _,prefix:=range []string{"https://"+m.base.Host+"/","http://"+m.base.Host+"/","//"+m.base.Host+"/"}{text=strings.ReplaceAll(text,prefix,"/")};for _,prefix:=range []string{"https://cdn.animeav1.com/","http://cdn.animeav1.com/","//cdn.animeav1.com/"}{text=strings.ReplaceAll(text,prefix,"/_cdn/")};return text}
func (m *Mirror) isHTML(ctype,path string)bool{return strings.Contains(strings.ToLower(ctype),"text/html")||strings.EqualFold(filepath.Ext(path),".html")||path==""||path=="/"}
func (m *Mirror) isText(ctype,path string)bool{c:=strings.ToLower(ctype);e:=strings.ToLower(filepath.Ext(path));return strings.Contains(c,"text/")||strings.Contains(c,"javascript")||strings.Contains(c,"json")||e==".js"||e==".mjs"||e==".css"||e==".html"}

func (m *Mirror) fetch(ctx context.Context,u string)([]byte,string,error){req,e:=http.NewRequestWithContext(ctx,http.MethodGet,u,nil);if e!=nil{return nil,"",e};req.Header.Set("User-Agent","Mozilla/5.0 (X11; Linux armv7l) AppleWebKit/537.36 Chrome/124 Safari/537.36");req.Header.Set("Accept","text/html,application/xhtml+xml,application/javascript,text/css,application/json,image/avif,image/webp,image/png,image/svg+xml,*/*;q=0.8");req.Header.Set("Accept-Language","es-ES,es;q=0.9");if parsed,pe:=url.Parse(u);pe==nil&&strings.EqualFold(parsed.Host,m.base.Host){m.mu.RLock();cookie:=m.sessionCookie;m.mu.RUnlock();if cookie!=""{req.Header.Set("Cookie",cookie)}};resp,e:=m.client.Do(req);if e!=nil{return nil,"",e};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=400{return nil,"",fmt.Errorf("GET %s: %s",u,resp.Status)};b,e:=io.ReadAll(io.LimitReader(resp.Body,32<<20));if e!=nil{return nil,"",e};if len(b)==0{return nil,"",fmt.Errorf("GET %s: respuesta vacia",u)};ctype:=resp.Header.Get("Content-Type");if ctype==""{ctype=mime.TypeByExtension(filepath.Ext(resp.Request.URL.Path))};return b,ctype,nil}
func (m *Mirror) save(root string,u *url.URL,body []byte,ctype string)error{p:=strings.TrimPrefix(filepath.Clean(u.Path),string(filepath.Separator));if u.Host=="cdn.animeav1.com"{p=filepath.Join("_cdn",p)};if p=="."||p==""{p="index.html"}else if m.isHTML(ctype,u.Path)&&filepath.Ext(p)==""{p=filepath.Join(p,"index.html")};full:=filepath.Join(root,p);cleanRoot:=filepath.Clean(root)+string(filepath.Separator);if !strings.HasPrefix(full,cleanRoot)&&full!=filepath.Join(root,"index.html"){return fmt.Errorf("unsafe path: %s",p)};if e:=os.MkdirAll(filepath.Dir(full),0o755);e!=nil{return e};return os.WriteFile(full,body,0o644)}
func (m *Mirror) addSanitized(n int){m.mu.Lock();m.state.Sanitized+=n;m.mu.Unlock()}
func (m *Mirror) recordError(err error){m.mu.Lock();defer m.mu.Unlock();m.state.Errors++;m.state.LastError=err.Error();m.state.ErrorLog=append(m.state.ErrorLog,ErrorEntry{Time:time.Now().Format(time.RFC3339),Message:err.Error()});if len(m.state.ErrorLog)>200{m.state.ErrorLog=append([]ErrorEntry(nil),m.state.ErrorLog[len(m.state.ErrorLog)-200:]...)}}
func (m *Mirror) fail(err error){m.recordError(err)}
