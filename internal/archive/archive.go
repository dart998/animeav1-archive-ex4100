package archive

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func Save(ctx context.Context, videoDir, slug string, episode int, sourceURL string) (string,string,int64,error) {
	dir:=filepath.Join(videoDir,slug); if err:=os.MkdirAll(dir,0o755);err!=nil{return "","",0,err}
	tmp:=filepath.Join(dir,fmt.Sprintf(".%03d.part.mp4",episode)); final:=filepath.Join(dir,fmt.Sprintf("%03d.mp4",episode)); _=os.Remove(tmp)
	cmd:=exec.CommandContext(ctx,"ffmpeg","-y","-loglevel","error","-i",sourceURL,"-c","copy",tmp); if b,err:=cmd.CombinedOutput();err!=nil{return "","",0,fmt.Errorf("ffmpeg: %w: %s",err,string(b))}
	f,err:=os.Open(tmp); if err!=nil{return "","",0,err}; h:=sha256.New(); n,err:=io.Copy(h,f); f.Close(); if err!=nil{return "","",0,err}; sum:=fmt.Sprintf("%x",h.Sum(nil))
	if _,err=os.Stat(final);err==nil { old,oe:=fileHash(final); if oe==nil&&old==sum { _=os.Remove(tmp); return final,sum,n,nil } }
	if err=os.Rename(tmp,final);err!=nil{return "","",0,err}; return final,sum,n,nil
}
func fileHash(path string)(string,error){f,e:=os.Open(path);if e!=nil{return "",e};defer f.Close();h:=sha256.New();_,e=io.Copy(h,f);if e!=nil{return "",e};return fmt.Sprintf("%x",h.Sum(nil)),nil}
