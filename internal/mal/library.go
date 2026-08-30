package mal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

type Client struct{ http *http.Client }

type rawItem struct {
	AnimeID int `json:"anime_id"`
	AnimeTitle string `json:"anime_title"`
	AnimeNumEpisodes int `json:"anime_num_episodes"`
	NumWatchedEpisodes int `json:"num_watched_episodes"`
	Status int `json:"status"`
}

func New() *Client { return &Client{http:&http.Client{Timeout:30*time.Second}} }

func (c *Client) Watching(ctx context.Context, username string, library []database.LibraryItem) ([]database.MALWatchingItem,error) {
	username=strings.TrimSpace(username)
	if username=="" { return nil,fmt.Errorf("usuario MAL vacío") }
	var all []rawItem
	for offset:=0; ; offset+=300 {
		u:=fmt.Sprintf("https://myanimelist.net/animelist/%s/load.json?status=1&offset=%d",username,offset)
		var page []rawItem
		if err:=c.getJSON(ctx,u,&page);err!=nil {
			// Some MAL layouts intermittently reject status-filtered calls. Fall back
			// to the public all-list endpoint and filter locally.
			if offset!=0 { return nil,err }
			return c.watchingFromAll(ctx,username,library)
		}
		all=append(all,page...)
		if len(page)<300 { break }
		time.Sleep(1200*time.Millisecond)
	}
	return build(all,library),nil
}

func (c *Client) watchingFromAll(ctx context.Context, username string, library []database.LibraryItem) ([]database.MALWatchingItem,error) {
	var all []rawItem
	for offset:=0; ; offset+=300 {
		u:=fmt.Sprintf("https://myanimelist.net/animelist/%s/load.json?status=7&offset=%d",username,offset)
		var page []rawItem
		if err:=c.getJSON(ctx,u,&page);err!=nil{return nil,err}
		for _,x:=range page{if x.Status==1{all=append(all,x)}}
		if len(page)<300{break}
		time.Sleep(1200*time.Millisecond)
	}
	return build(all,library),nil
}

func (c *Client) getJSON(ctx context.Context,u string,v any)error{
	req,e:=http.NewRequestWithContext(ctx,http.MethodGet,u,nil);if e!=nil{return e}
	req.Header.Set("User-Agent","Mozilla/5.0 (X11; Linux armv7l) AppleWebKit/537.36 Chrome/126 Safari/537.36")
	req.Header.Set("Accept","application/json,text/plain,*/*")
	req.Header.Set("Referer","https://myanimelist.net/")
	resp,e:=c.http.Do(req);if e!=nil{return e};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("MAL respondió %s",resp.Status)}
	return json.NewDecoder(resp.Body).Decode(v)
}

func build(raw []rawItem, library []database.LibraryItem) []database.MALWatchingItem {
	now:=time.Now().Format(time.RFC3339)
	out:=make([]database.MALWatchingItem,0,len(raw))
	for _,r:=range raw{
		local,score:=bestLocal(r.AnimeTitle,library)
		x:=database.MALWatchingItem{MALID:r.AnimeID,Title:r.AnimeTitle,Watched:r.NumWatchedEpisodes,Total:r.AnimeNumEpisodes,Status:r.Status,MatchScore:score,LastSeen:now}
		if score>=70{x.LocalName=local.Name;x.LocalPath=local.Path}
		out=append(out,x)
	}
	sort.Slice(out,func(i,j int)bool{return strings.ToLower(out[i].Title)<strings.ToLower(out[j].Title)})
	return out
}

var seasonWords=regexp.MustCompile(`\b(?:season|temporada|part|parte|cour|2nd|3rd|4th|ii|iii|iv)\b`)
func normalize(s string)string{
	s=strings.ToLower(s)
	var b strings.Builder
	for _,r:=range s{
		if unicode.IsLetter(r)||unicode.IsDigit(r){b.WriteRune(r)}else{b.WriteByte(' ')}
	}
	s=strings.Join(strings.Fields(b.String())," ")
	return strings.TrimSpace(s)
}
func tokens(s string)map[string]bool{m:=map[string]bool{};for _,w:=range strings.Fields(normalize(s)){if len(w)>1&&!seasonWords.MatchString(w){m[w]=true}};return m}
func similarity(a,b string)int{
	na,nb:=normalize(a),normalize(b);if na==nb{return 100}
	if strings.Contains(na,nb)||strings.Contains(nb,na){return 88}
	ta,tb:=tokens(a),tokens(b);if len(ta)==0||len(tb)==0{return 0}
	inter:=0;for k:=range ta{if tb[k]{inter++}}
	union:=len(ta)+len(tb)-inter
	if union==0{return 0};return inter*100/union
}
func bestLocal(title string,library []database.LibraryItem)(database.LibraryItem,int){var best database.LibraryItem;score:=0;for _,x:=range library{s:=similarity(title,x.Name);if s>score{score=s;best=x}};return best,score}
