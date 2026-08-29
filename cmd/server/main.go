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
	webui "github.com/dart998/animeav1-archive-ex4100/internal/web"
)

var version="dev"
func main(){cfg:=config.Load();for _,d:=range []string{"db","metadata","images","videos","site","logs","tmp"}{if e:=os.MkdirAll(filepath.Join(cfg.DataDir,d),0o755);e!=nil{log.Fatal(e)}};db,e:=database.Open(cfg.DBPath);if e!=nil{log.Fatal(e)};defer db.Close();cr:=crawler.New(cfg,db)
	if cfg.CrawlerEnabled { go func(){if e:=cr.RunAll(context.Background());e!=nil{log.Printf("initial crawl: %v",e)};t:=time.NewTicker(cfg.CrawlerInterval);defer t.Stop();for range t.C{if e:=cr.RunAll(context.Background());e!=nil{log.Printf("scheduled crawl: %v",e)}}}() }
	ui,e:=webui.New(db,cr,"/app/web");if e!=nil{log.Fatal(e)};addr:=":"+cfg.Port;log.Printf("animeav1-archive %s listening on %s",version,addr);log.Fatal(http.ListenAndServe(addr,ui.Handler()))}
