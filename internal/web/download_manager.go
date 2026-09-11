package web

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	libraryindex "github.com/dart998/animeav1-archive-ex4100/internal/library"
)

type seriesDownloadState struct {
	Slug string `json:"slug"`
	Running bool `json:"running"`
	Current int `json:"current"`
	Total int `json:"total"`
	Downloaded int `json:"downloaded"`
	Skipped int `json:"skipped"`
	Errors int `json:"errors"`
	CurrentFile string `json:"current_file,omitempty"`
	LastError string `json:"last_error,omitempty"`
	Started string `json:"started,omitempty"`
	Finished string `json:"finished,omitempty"`
}

var downloadStates=struct{sync.RWMutex;m map[string]*seriesDownloadState}{m:map[string]*seriesDownloadState{}}
var downloadQueueMu sync.Mutex

func (s *Server) downloadSeriesAPI(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return}
	if err:=r.ParseForm();err!=nil{http.Error(w,err.Error(),400);return}
	slug:=strings.TrimSpace(r.FormValue("slug"));item,ok:=s.itemForSlug(slug);if !ok{http.Error(w,"serie no encontrada",404);return}
	total:=item.Total;if total<1{total=s.db.SeriesEpisodeCount(slug)};if total<1{http.Error(w,"sin episodios",409);return}
	downloadStates.Lock();if x:=downloadStates.m[slug];x!=nil&&x.Running{downloadStates.Unlock();http.Error(w,"descarga en curso",409);return}
	st:=&seriesDownloadState{Slug:slug,Running:true,Total:total,Started:time.Now().Format(time.RFC3339)};downloadStates.m[slug]=st;downloadStates.Unlock()
	go s.runMegaSeries(item,st);w.Header().Set("Content-Type","application/json");w.WriteHeader(http.StatusAccepted);_=json.NewEncoder(w).Encode(st)
}

func (s *Server) downloadStatusAPI(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{http.Error(w,"method not allowed",405);return}
	slug:=r.URL.Query().Get("slug");downloadStates.RLock();x:=downloadStates.m[slug];if x==nil{downloadStates.RUnlock();http.Error(w,"sin descarga",404);return};c:=*x;downloadStates.RUnlock();w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(c)
}
func setDL(st *seriesDownloadState,f func(*seriesDownloadState)){downloadStates.Lock();defer downloadStates.Unlock();f(st)}

func (s *Server) runMegaSeries(item animeav1.Item,st *seriesDownloadState){
	downloadQueueMu.Lock();defer downloadQueueMu.Unlock()
	target,err:=s.downloadTargetDir(item);if err!=nil{setDL(st,func(x *seriesDownloadState){x.Running=false;x.Errors++;x.LastError=err.Error();x.Finished=time.Now().Format(time.RFC3339)});return}
	for ep:=1;ep<=st.Total;ep++{
		setDL(st,func(x *seriesDownloadState){x.Current=ep;x.CurrentFile=""})
		if _,_,e:=s.findLocalEpisodeFile(item.Slug,ep);e==nil{setDL(st,func(x *seriesDownloadState){x.Skipped++});continue}
		players,e:=s.fetchOnlinePlayers(context.Background(),item.Slug,ep);if e!=nil{setDL(st,func(x *seriesDownloadState){x.Errors++;x.LastError=fmt.Sprintf("Ep %d: %v",ep,e)});continue}
		mega:="";for _,p:=range players{if strings.EqualFold(p.Server,"Mega"){mega=p.URL;break}}
		if mega==""{setDL(st,func(x *seriesDownloadState){x.Errors++;x.LastError=fmt.Sprintf("Ep %d: sin Mega",ep)});continue}
		p,e:=downloadMega(context.Background(),mega,target);if e!=nil{setDL(st,func(x *seriesDownloadState){x.Errors++;x.LastError=fmt.Sprintf("Ep %d: %v",ep,e)});continue}
		setDL(st,func(x *seriesDownloadState){x.Downloaded++;x.CurrentFile=filepath.Base(p)})
	}
	if items,e:=libraryindex.Scan(s.libraryRoot);e==nil{_=s.db.ReplaceLibrary(items)}
	setDL(st,func(x *seriesDownloadState){x.Running=false;x.CurrentFile="";x.Finished=time.Now().Format(time.RFC3339)})
}

func (s *Server) downloadTargetDir(item animeav1.Item)(string,error){
	lib,e:=s.db.Library();if e!=nil{return "",e};if c:=localFolderCandidates(item,lib);len(c)>0{return c[0].Item.Path,nil}
	name:=safeDirName(item.Title);if name==""{name="Anime"};dir:=filepath.Join(s.libraryRoot,name);return dir,os.MkdirAll(dir,0755)
}
func safeDirName(s string)string{return strings.TrimSpace(strings.NewReplacer("/","-","\\","-",":"," -","*","","?","","\"","","<","",">","","|","-").Replace(s))}

type megaResp struct{G string `json:"g"`;S int64 `json:"s"`;AT string `json:"at"`}

func downloadMega(ctx context.Context,raw,target string)(string,error){
	u,e:=url.Parse(raw);if e!=nil{return "",e};parts:=strings.Split(strings.Trim(u.Path,"/"),"/");if len(parts)<2{return "",errors.New("URL Mega invalida")};handle:=parts[len(parts)-1]
	keyRaw,e:=base64.RawURLEncoding.DecodeString(strings.TrimSpace(u.Fragment));if e!=nil||len(keyRaw)!=32{return "",errors.New("clave Mega invalida")}
	key:=make([]byte,16);for i:=0;i<16;i++{key[i]=keyRaw[i]^keyRaw[i+16]};iv:=make([]byte,16);copy(iv,keyRaw[16:24])
	payload:=fmt.Sprintf(`[{"a":"g","g":1,"p":"%s"}]`,handle);payload=strings.ReplaceAll(payload,`\"`,`"`)
	api:="https://g.api.mega.co.nz/cs?id="+strconv.FormatInt(time.Now().UnixNano(),10);req,e:=http.NewRequestWithContext(ctx,http.MethodPost,api,strings.NewReader(payload));if e!=nil{return "",e};req.Header.Set("Content-Type","application/json")
	resp,e:=http.DefaultClient.Do(req);if e!=nil{return "",e};b,e:=io.ReadAll(io.LimitReader(resp.Body,2<<20));resp.Body.Close();if e!=nil{return "",e}
	var rr []megaResp;if json.Unmarshal(b,&rr)!=nil||len(rr)==0||rr[0].G==""{return "",fmt.Errorf("Mega API: %s",strings.TrimSpace(string(b)))}
	name:="mega-"+handle+".mp4";if rr[0].AT!=""{if n:=megaName(rr[0].AT,key);n!=""{name=filepath.Base(n)}}
	if err:=os.MkdirAll(target,0755);err!=nil{return "",err};dest:=filepath.Join(target,name);if _,e:=os.Stat(dest);e==nil{return dest,nil};part:=dest+".part"
	get,e:=http.NewRequestWithContext(ctx,http.MethodGet,rr[0].G,nil);if e!=nil{return "",e};r,e:=http.DefaultClient.Do(get);if e!=nil{return "",e};defer r.Body.Close();if r.StatusCode<200||r.StatusCode>=300{return "",fmt.Errorf("Mega descarga: %s",r.Status)}
	f,e:=os.Create(part);if e!=nil{return "",e};block,e:=aes.NewCipher(key);if e!=nil{f.Close();return "",e};_,copyErr:=io.Copy(f,&cipher.StreamReader{S:cipher.NewCTR(block,iv),R:r.Body});closeErr:=f.Close();if copyErr!=nil{return "",copyErr};if closeErr!=nil{return "",closeErr}
	if rr[0].S>0{st,e:=os.Stat(part);if e!=nil{return "",e};if st.Size()!=rr[0].S{return "",fmt.Errorf("descarga Mega incompleta: %d/%d bytes",st.Size(),rr[0].S)}}
	return dest,os.Rename(part,dest)
}

func megaName(at string,key []byte)string{
	b,e:=base64.RawURLEncoding.DecodeString(at);if e!=nil||len(b)==0||len(b)%16!=0{return ""};block,e:=aes.NewCipher(key);if e!=nil{return ""};out:=make([]byte,len(b));cipher.NewCBCDecrypter(block,make([]byte,16)).CryptBlocks(out,b);s:=strings.TrimRight(string(out),"\x00");if !strings.HasPrefix(s,"MEGA"){return ""};var x struct{N string `json:"n"`};if json.Unmarshal([]byte(s[4:]),&x)!=nil{return ""};return x.N
}
