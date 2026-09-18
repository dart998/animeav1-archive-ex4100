package web

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

type localFolderCandidate struct { Item database.LibraryItem; Rank int }

var seasonSuffixRE = regexp.MustCompile(`(?i)\b(?:\d+(?:st|nd|rd|th)\s+season|season\s*\d+|temporada\s*\d+|part\s*\d+|parte\s*\d+|cour\s*\d+|act[ ._-]*\d+|ovas?|oads?|specials?|especiales?|extras?|movie|pelicula|film|spin[- ]?off)\b`)
var trailingSeasonRE = regexp.MustCompile(`(?i)[\s._-]+([1-9][0-9]?)\s*$`)
var seasonEpisodeRE = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:s|t)\s*0*([1-9][0-9]?)\s*e\s*0*[0-9]+(?:[^0-9]|$)`)
var seasonNumberRE = regexp.MustCompile(`(?i)(?:^|\b)(?:season|temporada|part|parte|cour|act)\s*[ ._-]*0*([1-9][0-9]?)\b|\b([1-9][0-9]?)(?:st|nd|rd|th)\s+season\b`)

func seriesStem(s string) string {
	x := seasonSuffixRE.ReplaceAllString(s, " ")
	x = trailingSeasonRE.ReplaceAllString(x, " ")
	return normalizeName(x)
}
func seasonMarkerNumber(s string) int {
	if m:=seasonEpisodeRE.FindStringSubmatch(s);len(m)>1{n,_:=strconv.Atoi(m[1]);return n}
	m:=seasonNumberRE.FindStringSubmatch(s);if len(m)>0{for _,v:=range m[1:]{if v!=""{n,_:=strconv.Atoi(v);return n}}}
	return 0
}
func seasonNumber(s string) int { if n:=seasonMarkerNumber(s);n>0{return n};if m:=trailingSeasonRE.FindStringSubmatch(strings.TrimSpace(s));len(m)>1{n,_:=strconv.Atoi(m[1]);return n};return 0 }
func itemSeasonNumber(it animeav1.Item) int {
	if n:=seasonNumber(it.Title);n>0{return n}
	aliases:=make([]string,0,len(it.Aliases));for _,a:=range it.Aliases{aliases=append(aliases,a)};sort.Strings(aliases)
	for _,a:=range aliases{if n:=seasonNumber(a);n>0{return n}}
	return 1
}
func significantAlias(s string) bool { n:=normalizeName(s);return len(n)>=5 && n!="season" && n!="movie" && n!="special" }

func localFolderCandidates(it animeav1.Item, lib []database.LibraryItem) []localFolderCandidate {
	candidates:=[]string{it.Title};for _,v:=range it.Aliases{if strings.TrimSpace(v)!=""{candidates=append(candidates,v)}}
	exact:=map[string]bool{};stems:=map[string]bool{};aliases:=[]string{}
	for idx,c:=range candidates{n:=normalizeName(c);if n!=""{exact[n]=true};st:=seriesStem(c);if st!=""{stems[st]=true};if idx>0&&significantAlias(c){aliases=append(aliases,n)}}
	out:=make([]localFolderCandidate,0)
	for _,li:=range lib{
		n:=normalizeName(li.Name);rank:=99
		if exact[n]{rank=0}else if st:=seriesStem(li.Name);st!=""&&stems[st]{rank=1}else{for _,a:=range aliases{if strings.HasPrefix(n,a)||strings.Contains(n,a){rank=2;break}}}
		if rank<99{out=append(out,localFolderCandidate{Item:li,Rank:rank})}
	}
	sort.SliceStable(out,func(i,j int)bool{if out[i].Rank!=out[j].Rank{return out[i].Rank<out[j].Rank};if out[i].Item.Files!=out[j].Item.Files{return out[i].Item.Files>out[j].Item.Files};return strings.ToLower(out[i].Item.Name)<strings.ToLower(out[j].Item.Name)})
	return out
}

type episodeFileCandidate struct { Path string; FolderRank,FileRank,SeasonRank int }

func episodeFileRank(name string, mediaID animeav1.IDString, episode int) int {
	base:=strings.TrimSuffix(filepath.Base(name),filepath.Ext(name));if id:=strings.TrimSpace(string(mediaID));id!=""{strong:=regexp.MustCompile(`(?i)(?:^|[^0-9])`+regexp.QuoteMeta(id)+`[_ .-]+0*`+strconv.Itoa(episode)+`(?:[_ .-]|$)`);if strong.MatchString(base){return 0}}
	explicit:=regexp.MustCompile(fmt.Sprintf(`(?i)(?:^|[^0-9])(?:[st][0-9]{1,2}e|ep(?:isode)?[ ._-]*|e[ ._-]*)0*%d(?:[^0-9]|$)`,episode));if explicit.MatchString(base){return 1};standalone:=regexp.MustCompile(fmt.Sprintf(`(?:^|[^0-9])0*%d(?:[^0-9]|$)`,episode));if standalone.MatchString(base){return 2};return 99
}

func seasonRankForPath(path string, requested int) int {
	if requested<=0{requested=1}
	parts:=strings.Split(filepath.ToSlash(path),"/");seen:=0
	for i,p:=range parts{n:=0;if i==len(parts)-1{n=seasonMarkerNumber(strings.TrimSuffix(p,filepath.Ext(p)))}else{n=seasonNumber(p)};if n>0{seen=n}}
	if seen==requested{return 0};if seen==0{return 1};return 9
}

type localSeasonStat struct { Files int; Bytes int64 }
type localFolderSeasonSummary struct { Stats map[int]localSeasonStat; Seasonal bool }

func scanLocalFolderSeasons(root string) localFolderSeasonSummary {
	out:=localFolderSeasonSummary{Stats:map[int]localSeasonStat{}}
	rootSeason:=seasonNumber(filepath.Base(filepath.Clean(root)))
	_ = filepath.Walk(root,func(path string,info os.FileInfo,err error) error {
		if err!=nil||info==nil||info.IsDir(){return nil}
		switch strings.ToLower(filepath.Ext(info.Name())){case ".mkv",".mp4",".avi",".webm",".m4v",".mov":default:return nil}
		season:=0
		if rel,e:=filepath.Rel(root,path);e==nil{parts:=strings.Split(filepath.ToSlash(rel),"/");for i,p:=range parts{n:=0;if i==len(parts)-1{n=seasonMarkerNumber(strings.TrimSuffix(p,filepath.Ext(p)))}else{n=seasonNumber(p)};if n>0{season=n;out.Seasonal=true}}}
		if season==0&&rootSeason>0{season=rootSeason;out.Seasonal=true}
		if season==0{season=1}
		st:=out.Stats[season];st.Files++;st.Bytes+=info.Size();out.Stats[season]=st
		return nil
	})
	return out
}
