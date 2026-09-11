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
	"regexp"
	"sort"
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

var downloadStates = struct{ sync.RWMutex; m map[string]*seriesDownloadState }{m:map[string]*seriesDownloadState{}}
var transferDownloadRE = regexp.MustCompile(`server:"TransferIt",url:"(https://transfer\.it/t/[^"]+)"`)

func (s *Server) downloadSeriesAPI(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return}
	if err:=r.ParseForm();err!=nil{http.Error(w,err.Error(),400);return}
	slug:=strings.TrimSpace(r.FormValue("slug"));if slug==""{http.Error(w,"slug requerido",400);return}
	var item *animeav1.Item;items:=s.cachedAV1();for i:=range items{if items[i].Slug==slug{item=&items[i];break}}
	if item==nil{http.Error(w,"serie no encontrada en las listas AnimeAV1",404);return}
	downloadStates.Lock();if old:=downloadStates.m[slug];old!=nil&&old.Running{downloadStates.Unlock();http.Error(w,"ya hay una descarga de esta serie en curso",409);return}
	total:=item.Total;if total<1{total=s.db.SeriesEpisodeCount(slug)};if total<1{downloadStates.Unlock();http.Error(w,"no se pudo determinar el numero de episodios",409);return}
	st:=&seriesDownloadState{Slug:slug,Running:true,Total:total,Started:time.Now().Format(time.RFC3339)};downloadStates.m[slug]=st;downloadStates.Unlock();go s.runSeriesDownload(*item,st)
	w.Header().Set("Content-Type","application/json");w.WriteHeader(http.StatusAccepted);_=json.NewEncoder(w).Encode(st)
}

func (s *Server) downloadStatusAPI(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{http.Error(w,"method not allowed",405);return};slug:=strings.TrimSpace(r.URL.Query().Get("slug"));downloadStates.RLock();st:=downloadStates.m[slug]
	if st==nil{downloadStates.RUnlock();http.Error(w,"sin descarga",404);return};copy:=*st;downloadStates.RUnlock();w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(copy)
}
func setDownloadState(st *seriesDownloadState, fn func(*seriesDownloadState)){downloadStates.Lock();defer downloadStates.Unlock();fn(st)}

func (s *Server) runSeriesDownload(item animeav1.Item,st *seriesDownloadState){
	ctx:=context.Background();target,err:=s.downloadTargetDir(item);if err!=nil{setDownloadState(st,func(x *seriesDownloadState){x.Running=false;x.Errors++;x.LastError=err.Error();x.Finished=time.Now().Format(time.RFC3339)});return}
	for ep:=1;ep<=st.Total;ep++{setDownloadState(st,func(x *seriesDownloadState){x.Current=ep;x.CurrentFile=""});if _,_,e:=s.findLocalEpisodeFile(item.Slug,ep);e==nil{setDownloadState(st,func(x *seriesDownloadState){x.Skipped++});continue}
		transferURL,e:=s.episodeTransferURL(ctx,item.Slug,ep);if e!=nil{setDownloadState(st,func(x *seriesDownloadState){x.Errors++;x.LastError=fmt.Sprintf("Ep %d: %v",ep,e)});continue}
		path,e:=downloadTransferIt(ctx,transferURL,target);if e!=nil{setDownloadState(st,func(x *seriesDownloadState){x.Errors++;x.LastError=fmt.Sprintf("Ep %d: %v",ep,e)});continue};setDownloadState(st,func(x *seriesDownloadState){x.Downloaded++;x.CurrentFile=filepath.Base(path)})}
	if items,e:=libraryindex.Scan(s.libraryRoot);e==nil{_=s.db.ReplaceLibrary(items)};setDownloadState(st,func(x *seriesDownloadState){x.Running=false;x.CurrentFile="";x.Finished=time.Now().Format(time.RFC3339)})
}

func (s *Server) episodeTransferURL(parent context.Context,slug string,ep int)(string,error){
	ctx,cancel:=context.WithTimeout(parent,25*time.Second);defer cancel();u:=fmt.Sprintf("https://animeav1.com/media/%s/%d",slug,ep);req,e:=http.NewRequestWithContext(ctx,http.MethodGet,u,nil);if e!=nil{return "",e}
	req.Header.Set("User-Agent","Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/142 Safari/537.36");req.Header.Set("Accept","text/html,application/xhtml+xml");req.Header.Set("Accept-Language","es-ES,es;q=0.9");if c:=strings.TrimSpace(s.db.GetSetting("animeav1_session_cookie"));c!=""{req.Header.Set("Cookie",c)}
	resp,e:=http.DefaultClient.Do(req);if e!=nil{return "",e};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=400{return "",fmt.Errorf("AnimeAV1: %s",resp.Status)};b,e:=io.ReadAll(io.LimitReader(resp.Body,8<<20));if e!=nil{return "",e};m:=transferDownloadRE.FindSubmatch(b);if len(m)<2{return "",errors.New("no hay descarga TransferIt para este episodio")};return string(m[1]),nil
}

func (s *Server) downloadTargetDir(item animeav1.Item)(string,error){
	lib,e:=s.db.Library();if e!=nil{return "",e};root:="";if c:=localFolderCandidates(item,lib);len(c)>0{root=c[0].Item.Path};if root==""{root=filepath.Join(s.libraryRoot,safeDirName(item.Title));if e=os.MkdirAll(root,0o755);e!=nil{return "",e}}
	season:=seasonNumber(item.Title);if season==0{for _,a:=range item.Aliases{if n:=seasonNumber(a);n>0{season=n;break}}};if season==0{season=1};entries,_:=os.ReadDir(root);hasSeasonDirs:=false
	for _,entry:=range entries{if !entry.IsDir(){continue};n:=seasonNumber(entry.Name());if n>0{hasSeasonDirs=true;if n==season{return filepath.Join(root,entry.Name()),nil}}};if hasSeasonDirs||season>1{dir:=filepath.Join(root,fmt.Sprintf("Temporada %d",season));if e=os.MkdirAll(dir,0o755);e!=nil{return "",e};return dir,nil};return root,nil
}
func safeDirName(s string)string{s=strings.TrimSpace(s);if s==""{return "Anime"};r:=strings.NewReplacer("/","-","\\","-",":"," -","*","","?","","\"","","<","",">","","|","-");s=strings.TrimSpace(r.Replace(s));if s==""{return "Anime"};return s}

type transferNode struct{H string `json:"h"`;P string `json:"p"`;T int `json:"t"`;A string `json:"a"`;K string `json:"k"`;S int64 `json:"s"`}
type transferNodeResp struct{F []transferNode `json:"f"`}
type transferInfo struct{PW int `json:"pw"`}

func downloadTransferIt(parent context.Context,rawURL,target string)(string,error){
	u,e:=url.Parse(rawURL);if e!=nil{return "",e};parts:=strings.Split(strings.Trim(u.Path,"/"),"/");if len(parts)!=2||parts[0]!="t"||parts[1]==""{return "",errors.New("URL TransferIt invalida")};handle:=parts[1];info,e:=transferInfoFor(parent,handle);if e!=nil{return "",e};if info.PW!=0{return "",errors.New("TransferIt protegido por password no soportado")};nodes,e:=transferNodesFor(parent,handle);if e!=nil{return "",e}
	type fileNode struct{node transferNode;name string};files:=[]fileNode{};for _,n:=range nodes{if n.T!=0{continue};name,e:=decryptTransferNodeName(n);if e!=nil{continue};files=append(files,fileNode{n,name})};if len(files)==0{return "",errors.New("TransferIt no contiene archivos descargables")}
	sort.SliceStable(files,func(i,j int)bool{impi:=strings.EqualFold(filepath.Ext(files[i].name),".mp4");impj:=strings.EqualFold(filepath.Ext(files[j].name),".mp4");if impi!=impj{return impi};return files[i].node.S>files[j].node.S});f:=files[0];if !strings.EqualFold(filepath.Ext(f.name),".mp4"){return "",fmt.Errorf("TransferIt no contiene MP4 (encontrado %s)",f.name)};if e=os.MkdirAll(target,0o755);e!=nil{return "",e};dest:=filepath.Join(target,filepath.Base(f.name));return dest,downloadTransferNode(parent,handle,f.node,f.name,dest)
}
func transferInfoFor(ctx context.Context,handle string)(transferInfo,error){var out []transferInfo;e:=transferCall(ctx,"",[]map[string]any{{"a":"xi","xh":handle}},&out);if e!=nil{return transferInfo{},e};if len(out)==0{return transferInfo{},errors.New("TransferIt sin metadata")};return out[0],nil}
func transferNodesFor(ctx context.Context,handle string)([]transferNode,error){var out []transferNodeResp;e:=transferCall(ctx,handle,[]map[string]any{{"a":"f","c":1,"r":1}},&out);if e!=nil{return nil,e};if len(out)==0{return nil,errors.New("TransferIt sin nodos")};return out[0].F,nil}
func transferCall(ctx context.Context,handle string,payload any,out any)error{b,_:=json.Marshal(payload);u:="https://bt7.api.mega.co.nz/cs?id="+strconv.FormatInt(time.Now().UnixMilli(),10);if handle!=""{u+="&x="+url.QueryEscape(handle)};req,e:=http.NewRequestWithContext(ctx,http.MethodPost,u,strings.NewReader(string(b)));if e!=nil{return e};req.Header.Set("Content-Type","application/json");req.Header.Set("User-Agent","Mozilla/5.0");resp,e:=http.DefaultClient.Do(req);if e!=nil{return e};defer resp.Body.Close();raw,e:=io.ReadAll(io.LimitReader(resp.Body,4<<20));if e!=nil{return e};if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("TransferIt API: %s",resp.Status)};trim:=strings.TrimSpace(string(raw));if strings.HasPrefix(trim,"-"){return fmt.Errorf("TransferIt API error %s",trim)};return json.Unmarshal(raw,out)}
func decodeURLBase64(s string)([]byte,error){s=strings.TrimSpace(s);if i:=strings.LastIndex(s,":");i>=0{s=s[i+1:]};return base64.RawURLEncoding.DecodeString(s)}
func decryptTransferNodeName(n transferNode)(string,error){keyRaw,e:=decodeURLBase64(n.K);if e!=nil{return "",e};var key []byte;switch len(keyRaw){case 16:key=append([]byte(nil),keyRaw...);case 32:key=make([]byte,16);for i:=0;i<16;i++{key[i]=keyRaw[i]^keyRaw[i+16]};default:return "",fmt.Errorf("clave TransferIt inesperada: %d bytes",len(keyRaw))};enc,e:=decodeURLBase64(n.A);if e!=nil{return "",e};if len(enc)==0||len(enc)%aes.BlockSize!=0{return "",errors.New("atributos TransferIt invalidos")};block,e:=aes.NewCipher(key);if e!=nil{return "",e};plain:=make([]byte,len(enc));iv:=make([]byte,aes.BlockSize);cipher.NewCBCDecrypter(block,iv).CryptBlocks(plain,enc);plain=[]byte(strings.TrimRight(string(plain),"\x00"));if !strings.HasPrefix(string(plain),"MEGA"){return "",errors.New("atributos TransferIt sin cabecera MEGA")};var attr struct{N string `json:"n"`};if e=json.Unmarshal(plain[4:],&attr);e!=nil{return "",e};if strings.TrimSpace(attr.N)==""{return "",errors.New("nombre TransferIt vacio")};return filepath.Base(attr.N),nil}
func downloadTransferNode(ctx context.Context,handle string,n transferNode,name,dest string)error{if st,e:=os.Stat(dest);e==nil&&st.Size()==n.S{return nil};part:=dest+".part";offset:=int64(0);if st,e:=os.Stat(part);e==nil{offset=st.Size();if offset>n.S{_=os.Remove(part);offset=0}};q:=url.Values{};q.Set("x",handle);q.Set("n",n.H);q.Set("fn",name);u:="https://bt7.api.mega.co.nz/cs/g?"+q.Encode();req,e:=http.NewRequestWithContext(ctx,http.MethodGet,u,nil);if e!=nil{return e};req.Header.Set("User-Agent","Mozilla/5.0");req.Header.Set("Referer","https://transfer.it/t/"+handle);req.Header.Set("Origin","https://transfer.it");if offset>0{req.Header.Set("Range",fmt.Sprintf("bytes=%d-",offset))};client:=&http.Client{Timeout:0};resp,e:=client.Do(req);if e!=nil{return e};defer resp.Body.Close();flags:=os.O_CREATE|os.O_WRONLY;if offset>0&&resp.StatusCode==http.StatusPartialContent{flags|=os.O_APPEND}else{offset=0;flags|=os.O_TRUNC};if resp.StatusCode!=http.StatusOK&&resp.StatusCode!=http.StatusPartialContent{return fmt.Errorf("TransferIt descarga: %s",resp.Status)};f,e:=os.OpenFile(part,flags,0o644);if e!=nil{return e};_,copyErr:=io.Copy(f,resp.Body);closeErr:=f.Close();if copyErr!=nil{return copyErr};if closeErr!=nil{return closeErr};st,e:=os.Stat(part);if e!=nil{return e};if n.S>0&&st.Size()!=n.S{return fmt.Errorf("TransferIt descarga incompleta: %d/%d bytes",st.Size(),n.S)};return os.Rename(part,dest)}
