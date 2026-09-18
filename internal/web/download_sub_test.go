package web

import "testing"

func TestIsDubEpisodeFileName(t *testing.T){
	cases:=map[string]bool{"354_1_DUB.mp4":true,"354_1_dubbed.mkv":true,"354_1_SUB.mp4":false,"354_1.mp4":false}
	for name,want:=range cases{if got:=isDubEpisodeFileName(name);got!=want{t.Errorf("isDubEpisodeFileName(%q)=%v, want %v",name,got,want)}}
}

func TestSelectSubMegaDownloadURLs(t *testing.T){
	dub:="https://mega.nz/file/DUBHANDLE#key";sub:="https://mega.nz/file/SUBHANDLE#key"
	body:=[]byte(`audio:"DUB",players:[{server:"Mega",url:"`+dub+`"}],audio:"SUB",players:[{server:"Mega",url:"`+sub+`"}]`)
	got:=selectSubMegaDownloadURLs(body);if len(got)!=1||got[0]!=sub{t.Fatalf("got %v, want SUB only %q",got,sub)}
	body=[]byte(`type:"SUB",players:[{server:"Mega",url:"`+sub+`"}],type:"DUB",players:[{server:"Mega",url:"`+dub+`"}]`)
	got=selectSubMegaDownloadURLs(body);if len(got)!=1||got[0]!=sub{t.Fatalf("got %v with SUB first, want %q",got,sub)}
}

func TestSelectSubMegaDownloadURLsLegacySingleUnlabelled(t *testing.T){
	sub:="https://mega.nz/file/SINGLE#key";got:=selectSubMegaDownloadURLs([]byte(`players:[{server:"Mega",url:"`+sub+`"}]`))
	if len(got)!=1||got[0]!=sub{t.Fatalf("single unlabelled source = %v, want %q",got,sub)}
}

func TestSelectSubMegaDownloadURLsRejectsAmbiguousUnlabelled(t *testing.T){
	body:=[]byte(`[{server:"Mega",url:"https://mega.nz/file/A#k"},{server:"Mega",url:"https://mega.nz/file/B#k"}]`)
	if got:=selectSubMegaDownloadURLs(body);len(got)!=0{t.Fatalf("ambiguous unlabelled Mega sources must not be selected: %v",got)}
}
