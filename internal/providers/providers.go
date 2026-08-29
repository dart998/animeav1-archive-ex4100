package providers

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

var urlRE = regexp.MustCompile(`https?://[^\s"'<>\\]+`)

func Detect(body string, order []string) []database.Source {
	lower := strings.ToLower(body)
	prio := map[string]int{}; for i,p := range order { prio[p]=i+1 }
	seen := map[string]bool{}; out := []database.Source{}
	for _, raw := range urlRE.FindAllString(body,-1) {
		raw = html.UnescapeString(strings.TrimRight(raw, "),.;"))
		p := classify(raw); if p=="" { continue }; key:=p+"|"+raw; if seen[key] {continue}; seen[key]=true
		pv:=prio[p]; if pv==0 {pv=99}; out=append(out,database.Source{Provider:p,URL:raw,Priority:pv,Status:"detected"})
	}
	for _,p := range order { if strings.Contains(lower,p) && !hasProvider(out,p) { out=append(out,database.Source{Provider:p,Priority:prio[p],Status:"detected",Error:"provider visible but media URL is not exposed in page HTML"}) } }
	sort.SliceStable(out,func(i,j int)bool{return out[i].Priority<out[j].Priority}); return out
}

func classify(raw string) string {
	u,err:=url.Parse(raw); if err!=nil{return ""}; h:=strings.ToLower(u.Host); p:=strings.ToLower(u.Path)
	switch { case strings.Contains(p,".m3u8"): return "hls"; case strings.Contains(h,"upnshare"): return "upnshare"; case strings.Contains(h,"mega.nz")||strings.Contains(h,"mega.co.nz"): return "mega"; case strings.Contains(h,"mp4upload"): return "mp4upload" }; return ""
}
func hasProvider(s []database.Source,p string)bool{for _,x:=range s{if x.Provider==p{return true}};return false}

func Validate(ctx context.Context, src database.Source) (bool,string) {
	if src.URL=="" { return false,"no directly exposed source URL" }
	c:=&http.Client{Timeout:12*time.Second}; req,err:=http.NewRequestWithContext(ctx,http.MethodGet,src.URL,nil); if err!=nil{return false,err.Error()}; req.Header.Set("Range","bytes=0-2048"); req.Header.Set("User-Agent","Mozilla/5.0 AnimeAV1Archive/0.1")
	resp,err:=c.Do(req); if err!=nil{return false,err.Error()}; defer resp.Body.Close(); _,_=io.CopyN(io.Discard,resp.Body,2048)
	if resp.StatusCode>=200&&resp.StatusCode<400{return true,""}; return false,resp.Status
}
