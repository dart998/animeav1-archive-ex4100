package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dart998/animeav1-archive-ex4100/internal/config"
	"github.com/dart998/animeav1-archive-ex4100/internal/crawler"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
	libraryindex "github.com/dart998/animeav1-archive-ex4100/internal/library"
	sitemirror "github.com/dart998/animeav1-archive-ex4100/internal/site"
	webui "github.com/dart998/animeav1-archive-ex4100/internal/web"
)

var version="dev"
var commitSHA="unknown"
func main(){
	cfg:=config.Load()
	for _,d:=range []string{"db","metadata","images","site","logs","tmp"}{if e:=os.MkdirAll(filepath.Join(cfg.DataDir,d),0o755);e!=nil{log.Fatal(e)}}
	db,e:=database.Open(cfg.DBPath);if e!=nil{log.Fatal(e)};defer db.Close()
	libraryRoot:="/library"
	if items,err:=libraryindex.Scan(libraryRoot);err!=nil{log.Printf("library scan: %v",err)}else if err=db.ReplaceLibrary(items);err!=nil{log.Printf("library index save: %v",err)}else{log.Printf("library indexed: %d series folders",len(items))}
	cr:=crawler.New(cfg,db)
	if cfg.CrawlerEnabled { go func(){if e:=cr.RunAll(context.Background());e!=nil{log.Printf("initial crawl: %v",e)};t:=time.NewTicker(cfg.CrawlerInterval);defer t.Stop();for range t.C{if e:=cr.RunAll(context.Background());e!=nil{log.Printf("scheduled crawl: %v",e)}}}() }
	go func(){t:=time.NewTicker(6*time.Hour);defer t.Stop();for range t.C{if items,err:=libraryindex.Scan(libraryRoot);err==nil{_ = db.ReplaceLibrary(items)}}}()
	mirror,e:=sitemirror.New(cfg.BaseURL,filepath.Join(cfg.DataDir,"site"));if e!=nil{log.Fatal(e)}
	ui,e:=webui.New(db,cr,mirror,"/app/web",libraryRoot,filepath.Join(cfg.DataDir,"site"),version,commitSHA);if e!=nil{log.Fatal(e)}
	addr:=":"+cfg.Port;log.Printf("animeav1-archive %s (%s) listening on %s",version,commitSHA,addr);log.Fatal(http.ListenAndServe(addr,ui.Handler()))
}
