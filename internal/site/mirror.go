package site

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type State struct {
	Running bool
	Started string
	Finished string
	Fetched int
	Errors int
	Current string
	LastError string
}

type Mirror struct {
	base *url.URL
	root string
	client *http.Client
	mu sync.RWMutex
	state State
}

var refRE = regexp.MustCompile(`(?i)(?:href|src)=["']([^"'#]+)["']`)

func New(baseURL, root string) (*Mirror,error){
	u,err:=url.Parse(baseURL);if err!=nil{return nil,err}
	return &Mirror{base:u,root:root,client:&http.Client{Timeout:30*time.Second}},nil
}

func (m *Mirror) Snapshot() State { m.mu.RLock();defer m.mu.RUnlock();return m.state }

func (m *Mirror) Start(ctx context.Context) bool {
	m.mu.Lock();if m.state.Running{m.mu.Unlock();return false};m.state=State{Running:true,Started:time.Now().Format(time.RFC3339)};m.mu.Unlock()
	go m.run(ctx)
	return true
}

func (m *Mirror) run(ctx context.Context){
	defer func(){m.mu.Lock();m.state.Running=false;m.state.Finished=time.Now().Format(time.RFC3339);m.mu.Unlock()}()
	_ = os.MkdirAll(m.root,0o755)
	queue:=[]string{m.base.String()}
	seen:=map[string]bool{}
	for len(queue)>0{
		select{case <-ctx.Done():return;default:}
		raw:=queue[0];queue=queue[1:]
		u,err:=url.Parse(raw);if err!=nil{continue}
		u.Fragment=""
		if u.Host!=m.base.Host{continue}
		u.RawQuery=""
		key:=u.String();if seen[key]{continue};seen[key]=true
		m.mu.Lock();m.state.Current=key;m.mu.Unlock()
		body,ctype,err:=m.fetch(ctx,key)
		if err!=nil{m.mu.Lock();m.state.Errors++;m.state.LastError=err.Error();m.mu.Unlock();continue}
		if strings.Contains(ctype,"text/html"){
			text:=string(body)
			for _,match:=range refRE.FindAllStringSubmatch(text,-1){
				ref,err:=url.Parse(match[1]);if err!=nil{continue}
				abs:=u.ResolveReference(ref);if abs.Host==m.base.Host && (abs.Scheme=="http"||abs.Scheme=="https"){queue=append(queue,abs.String())}
			}
			body=[]byte(text)
		}
		if err=m.save(u,body,ctype);err!=nil{m.mu.Lock();m.state.Errors++;m.state.LastError=err.Error();m.mu.Unlock();continue}
		m.mu.Lock();m.state.Fetched++;m.mu.Unlock()
		time.Sleep(200*time.Millisecond)
	}
}

func (m *Mirror) fetch(ctx context.Context,u string)([]byte,string,error){
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,u,nil);if err!=nil{return nil,"",err}
	req.Header.Set("User-Agent","Mozilla/5.0 AnimeAV1ArchiveMirror/0.1")
	resp,err:=m.client.Do(req);if err!=nil{return nil,"",err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=400{return nil,"",fmt.Errorf("GET %s: %s",u,resp.Status)}
	b,err:=io.ReadAll(io.LimitReader(resp.Body,32<<20));return b,resp.Header.Get("Content-Type"),err
}

func (m *Mirror) save(u *url.URL,body []byte,ctype string) error {
	p:=strings.TrimPrefix(filepath.Clean(u.Path),string(filepath.Separator))
	if p=="."||p==""{p="index.html"}else if strings.Contains(ctype,"text/html") && filepath.Ext(p)==""{p=filepath.Join(p,"index.html")}
	full:=filepath.Join(m.root,p)
	if !strings.HasPrefix(full,filepath.Clean(m.root)+string(filepath.Separator)) && full!=filepath.Join(m.root,"index.html"){return fmt.Errorf("unsafe path")}
	if err:=os.MkdirAll(filepath.Dir(full),0o755);err!=nil{return err}
	return os.WriteFile(full,body,0o644)
}
