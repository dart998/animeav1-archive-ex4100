package animeav1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const librarySuperformID = "0op3vsvo"

type libraryFormState struct {
	Status int
	Rating *int
	Episode int
	Notes string
	StartDate *string
	EndDate *string
	Private bool
	MediaID IDString
}

func (c *Client) MarkWatched(ctx context.Context, cookie string, mediaID IDString, episode int) error {
	if strings.TrimSpace(cookie)=="" { return errors.New("cookie de AnimeAV1 no configurada") }
	if strings.TrimSpace(string(mediaID))=="" || episode < 1 { return errors.New("mediaId o episodio invalido") }
	state,err:=c.libraryFormState(ctx,cookie,mediaID); if err!=nil{return err}
	if episode < state.Episode { episode=state.Episode }
	state.Episode=episode
	payload,err:=encodeLibraryForm(state); if err!=nil{return err}
	form:=url.Values{}
	form.Set("__superform_json",payload)
	form.Set("__superform_id",librarySuperformID)
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,c.base+"/cuenta/listas?/library",strings.NewReader(form.Encode()));if err!=nil{return err}
	req.Header.Set("Accept","application/json")
	req.Header.Set("Content-Type","application/x-www-form-urlencoded")
	req.Header.Set("Origin",c.base)
	req.Header.Set("Referer",c.base+"/cuenta/listas/viendo")
	req.Header.Set("X-SvelteKit-Action","true")
	req.Header.Set("User-Agent","Mozilla/5.0 (X11; Linux armv7l) AppleWebKit/537.36 Chrome/124 Safari/537.36")
	req.Header.Set("Cookie",cookie)
	resp,err:=c.http.Do(req);if err!=nil{return err};defer resp.Body.Close()
	b,_:=io.ReadAll(io.LimitReader(resp.Body,1<<20))
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("AnimeAV1 actualizar visto: HTTP %d: %s",resp.StatusCode,strings.TrimSpace(string(b)))}
	return nil
}

func (c *Client) libraryFormState(ctx context.Context,cookie string,mediaID IDString)(libraryFormState,error){
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,c.base+"/cuenta/listas",nil);if err!=nil{return libraryFormState{},err}
	req.Header.Set("User-Agent","Mozilla/5.0 (X11; Linux armv7l) AppleWebKit/537.36 Chrome/124 Safari/537.36")
	req.Header.Set("Accept","text/html,application/xhtml+xml")
	req.Header.Set("Cookie",cookie)
	resp,err:=c.http.Do(req);if err!=nil{return libraryFormState{},err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=400{return libraryFormState{},fmt.Errorf("AnimeAV1 respondio HTTP %d",resp.StatusCode)}
	b,err:=io.ReadAll(io.LimitReader(resp.Body,8<<20));if err!=nil{return libraryFormState{},err}
	body:=string(b);i:=strings.Index(body,"libraryEntries:");if i<0{return libraryFormState{},errors.New("libraryEntries no encontrado")};j:=strings.Index(body[i:],"[");if j<0{return libraryFormState{},errors.New("libraryEntries sin array")};arr,err:=balanced(body,i+j,'[',']');if err!=nil{return libraryFormState{},err}
	for _,obj:=range splitTopObjects(arr){
		id:=fieldID(obj,"mediaId");if string(id)!=string(mediaID){continue}
		s:=libraryFormState{Status:fieldInt(obj,"status"),Episode:fieldInt(obj,"episode"),Notes:fieldString(obj,"notes"),Private:fieldBool(obj,"private"),MediaID:id}
		if m:=reIntField("rating").FindStringSubmatch(obj);len(m)>1&&m[1]!="null"{v,_:=strconv.Atoi(m[1]);s.Rating=&v}else if m:=reIntField("score").FindStringSubmatch(obj);len(m)>1&&m[1]!="null"{v,_:=strconv.Atoi(m[1]);if v>0{s.Rating=&v}}
		if v:=fieldString(obj,"startDate");v!=""{s.StartDate=&v};if v:=fieldString(obj,"endDate");v!=""{s.EndDate=&v}
		return s,nil
	}
	return libraryFormState{},fmt.Errorf("mediaId %s no encontrado en AnimeAV1",mediaID)
}

func encodeLibraryForm(s libraryFormState)(string,error){
	values:=[]any{map[string]int{"status":1,"rating":2,"episode":3,"notes":4,"startDate":5,"endDate":6,"private":7,"mediaId":8},s.Status,nil,s.Episode,s.Notes,nil,nil,s.Private,nil}
	if s.Rating!=nil{values[2]=*s.Rating};if s.StartDate!=nil{values[5]=*s.StartDate};if s.EndDate!=nil{values[6]=*s.EndDate}
	if n,err:=strconv.ParseInt(string(s.MediaID),10,64);err==nil{values[8]=n}else{values[8]=string(s.MediaID)}
	b,err:=json.Marshal(values);if err!=nil{return "",err};return string(b),nil
}
