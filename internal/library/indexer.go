package library

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

var mediaExt=map[string]bool{".mkv":true,".mp4":true,".avi":true,".webm":true,".m4v":true,".mov":true}

func Scan(root string)([]database.LibraryItem,error){
	entries,err:=os.ReadDir(root);if err!=nil{return nil,err}
	now:=time.Now().Format(time.RFC3339)
	out:=make([]database.LibraryItem,0,len(entries))
	for _,entry:=range entries{
		if !entry.IsDir(){continue}
		base:=filepath.Join(root,entry.Name())
		item:=database.LibraryItem{Name:entry.Name(),Path:base,LastScan:now}
		_ = filepath.Walk(base,func(path string,info os.FileInfo,err error)error{
			if err!=nil||info==nil||info.IsDir(){return nil}
			if mediaExt[strings.ToLower(filepath.Ext(info.Name()))]{item.Files++;item.Bytes+=info.Size()}
			return nil
		})
		out=append(out,item)
	}
	return out,nil
}
