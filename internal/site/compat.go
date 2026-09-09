package site

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type animeAV1RefererTransport struct { base http.RoundTripper; seedRoot string }

func (t *animeAV1RefererTransport) RoundTrip(req *http.Request) (*http.Response,error){
	base:=t.base;if base==nil{base=http.DefaultTransport};request:=req
	if strings.EqualFold(req.URL.Hostname(),"cdn.animeav1.com"){
		clone:=req.Clone(req.Context());clone.Header=req.Header.Clone()
		clone.Header.Set("Referer","https://animeav1.com/")
		clone.Header.Set("User-Agent","Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36 Edg/142.0.0.0")
		clone.Header.Set("Accept-Language","es-ES,es;q=0.9,en;q=0.8")
		clone.Header.Set("Sec-Fetch-Site","same-site");clone.Header.Set("Sec-Fetch-Mode","no-cors")
		ext:=strings.ToLower(filepath.Ext(req.URL.Path));switch ext{case ".jpg",".jpeg",".png",".webp",".avif",".gif",".svg",".ico":clone.Header.Set("Accept","image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8");clone.Header.Set("Sec-Fetch-Dest","image");default:clone.Header.Set("Sec-Fetch-Dest","empty")}
		request=clone
	}
	resp,err:=base.RoundTrip(request);if err==nil&&resp!=nil&&resp.StatusCode!=http.StatusForbidden&&resp.StatusCode!=http.StatusNotFound{return resp,nil}
	seedResp,seedErr:=t.seedResponse(req);if seedErr==nil&&seedResp!=nil{if resp!=nil&&resp.Body!=nil{_=resp.Body.Close()};return seedResp,nil};return resp,err
}

func (t *animeAV1RefererTransport) seedResponse(req *http.Request)(*http.Response,error){root:=t.seedRoot;if root==""{root="/data/seed"};host:=strings.ToLower(req.URL.Hostname());path:=filepath.Clean("/"+strings.TrimPrefix(req.URL.Path,"/"));if path=="/"||strings.Contains(path,".."){return nil,os.ErrNotExist};rel:=strings.TrimPrefix(path,"/");if host=="cdn.animeav1.com"{rel=filepath.Join("_cdn",rel)}else if host!="animeav1.com"&&host!="www.animeav1.com"{return nil,os.ErrNotExist};full:=filepath.Join(root,rel);cleanRoot:=filepath.Clean(root)+string(filepath.Separator);if !strings.HasPrefix(filepath.Clean(full)+string(filepath.Separator),cleanRoot){return nil,os.ErrNotExist};body,err:=os.ReadFile(full);if err!=nil{return nil,err};ctype:=mime.TypeByExtension(filepath.Ext(full));if ctype==""{ctype="application/octet-stream"};return &http.Response{Status:"200 OK (seed)",StatusCode:http.StatusOK,Proto:"HTTP/1.1",ProtoMajor:1,ProtoMinor:1,Header:http.Header{"Content-Type":[]string{ctype},"X-AnimeAV1-Seed":[]string{"1"}},Body:io.NopCloser(bytes.NewReader(body)),ContentLength:int64(len(body)),Request:req},nil}

func init(){absoluteURL=regexp.MustCompile(`(?i)https?://(?:[a-z0-9-]+\.)*(?:runative-syndicate\.com|runative\.com|popads[^/]*|adsterra[^/]*)[^\s"'<>)]*`);base:=http.DefaultTransport;http.DefaultTransport=&animeAV1RefererTransport{base:base,seedRoot:"/data/seed"}}
