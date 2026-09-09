package web

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dart998/animeav1-archive-ex4100/internal/animeav1"
	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

type localFolderCandidate struct {
	Item database.LibraryItem
	Rank int
}

var seasonSuffixRE = regexp.MustCompile(`(?i)\b(?:\d+(?:st|nd|rd|th)\s+season|season\s*\d+|temporada\s*\d+|part\s*\d+|parte\s*\d+|cour\s*\d+|ovas?|oads?|specials?|especiales?|extras?)\b`)

func seriesStem(s string) string {
	s = seasonSuffixRE.ReplaceAllString(s, " ")
	return normalizeName(s)
}

func localFolderCandidates(it animeav1.Item, lib []database.LibraryItem) []localFolderCandidate {
	candidates := []string{it.Title}
	for _, v := range it.Aliases { if strings.TrimSpace(v) != "" { candidates = append(candidates, v) } }
	exact := map[string]bool{}
	stems := map[string]bool{}
	for _, c := range candidates {
		n := normalizeName(c); if n != "" { exact[n] = true }
		st := seriesStem(c); if st != "" { stems[st] = true }
	}
	out := make([]localFolderCandidate, 0)
	for _, li := range lib {
		n := normalizeName(li.Name)
		rank := 99
		if exact[n] { rank = 0 } else if st := seriesStem(li.Name); st != "" && stems[st] { rank = 1 }
		if rank < 99 { out = append(out, localFolderCandidate{Item: li, Rank: rank}) }
	}
	sort.SliceStable(out, func(i,j int) bool { if out[i].Rank != out[j].Rank { return out[i].Rank < out[j].Rank }; return strings.ToLower(out[i].Item.Name) < strings.ToLower(out[j].Item.Name) })
	return out
}

type episodeFileCandidate struct {
	Path string
	FolderRank int
	FileRank int
}

func episodeFileRank(name string, mediaID animeav1.IDString, episode int) int {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if id := strings.TrimSpace(string(mediaID)); id != "" {
		strong := regexp.MustCompile(`(?i)(?:^|[^0-9])`+regexp.QuoteMeta(id)+`[_ .-]+0*`+strconv.Itoa(episode)+`(?:[_ .-]|$)`)
		if strong.MatchString(base) { return 0 }
	}
	explicit := regexp.MustCompile(fmt.Sprintf(`(?i)(?:^|[^0-9])(?:s[0-9]{1,2}e|ep(?:isode)?[ ._-]*|e[ ._-]*)0*%d(?:[^0-9]|$)`,episode))
	if explicit.MatchString(base) { return 1 }
	standalone := regexp.MustCompile(fmt.Sprintf(`(?:^|[^0-9])0*%d(?:[^0-9]|$)`,episode))
	if standalone.MatchString(base) { return 2 }
	return 99
}
