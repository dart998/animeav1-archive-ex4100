package animeav1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type SeriesState struct {
	Finalized bool
	LastEpisode int
	HasEpisodeZero bool
}


func parseSeriesState(body,slug string)(SeriesState,error){
	slug=strings.TrimSpace(slug);if slug==""{return SeriesState{},errors.New("slug vacío")}
	re:=regexp.MustCompile(`/media/`+regexp.QuoteMeta(slug)+`/(\d+)(?:["'/?#<]|$)`)
	low:=strings.ToLower(body);finalized:=false;if i:=strings.Index(low,"<h1");i>=0{end:=i+4000;if end>len(low){end=len(low)};finalized=strings.Contains(low[i:end],"finalizado")};state:=SeriesState{Finalized:finalized,LastEpisode:-1}
	for _,m:=range re.FindAllStringSubmatch(body,-1){n,e:=strconv.Atoi(m[1]);if e!=nil{continue};if n==0{state.HasEpisodeZero=true};if n>state.LastEpisode{state.LastEpisode=n}}
	if state.LastEpisode<0{return state,fmt.Errorf("no se encontraron episodios para %s",slug)}
	return state,nil
}

func (c *Client) SeriesState(ctx context.Context,cookie,slug string)(SeriesState,error){
	slug=strings.TrimSpace(slug);if slug==""{return SeriesState{},errors.New("slug vacío")}
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,c.base+"/media/"+url.PathEscape(slug),nil);if err!=nil{return SeriesState{},err}
	req.Header.Set("User-Agent","Mozilla/5.0 (X11; Linux armv7l) AppleWebKit/537.36 Chrome/124 Safari/537.36");req.Header.Set("Accept","text/html,application/xhtml+xml");req.Header.Set("Accept-Language","es-ES,es;q=0.9");if strings.TrimSpace(cookie)!=""{req.Header.Set("Cookie",cookie)}
	resp,err:=c.http.Do(req);if err!=nil{return SeriesState{},err};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=400{return SeriesState{},fmt.Errorf("AnimeAV1 ficha %s: HTTP %d",slug,resp.StatusCode)}
	b,err:=io.ReadAll(io.LimitReader(resp.Body,8<<20));if err!=nil{return SeriesState{},err};return parseSeriesState(string(b),slug)
}
