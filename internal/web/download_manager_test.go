package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCreateSeriesDirKeepsNormalTitle(t *testing.T){
	root:=t.TempDir();title:="Koko wa Ore ni Makasete Saki ni Ike to Itte kara 10-nen ga Tattara Densetsu ni Natteita"
	dir,err:=createSeriesDir(root,title);if err!=nil{t.Fatal(err)}
	if filepath.Base(dir)!=title{t.Fatalf("folder = %q, want full title %q",filepath.Base(dir),title)}
}

func TestCreateSeriesDirShortensOnlyWhenRequired(t *testing.T){
	root:=t.TempDir();title:=strings.Repeat("abc",120)
	dir,err:=createSeriesDir(root,title);if err!=nil{t.Fatal(err)}
	name:=filepath.Base(dir);if name==title{t.Fatal("expected overlong title to be shortened")}
	if len(name)>240{t.Fatalf("shortened folder uses %d bytes, want <= 240",len(name))}
	if !utf8.ValidString(name){t.Fatalf("shortened folder is not valid UTF-8: %q",name)}
	if _,err:=os.Stat(dir);err!=nil{t.Fatal(err)}
}

func TestCompactDirNameDeterministic(t *testing.T){
	name:="Serie muy larga "+strings.Repeat("á",100)
	a:=compactDirName(name,80);b:=compactDirName(name,80)
	if a!=b{t.Fatalf("compactDirName not deterministic: %q != %q",a,b)}
	if len(a)>80||!utf8.ValidString(a){t.Fatalf("invalid compact name %q (%d bytes)",a,len(a))}
}

func TestMegaAPIResponseParsing(t *testing.T){
	info,code,err:=parseMegaInfoResponse([]byte(`[{"g":"https://example.invalid/file","s":123,"at":"x"}]`))
	if err!=nil||code!=0||info.G==""||info.S!=123{t.Fatalf("unexpected parsed response: info=%+v code=%d err=%v",info,code,err)}
	_,code,err=parseMegaInfoResponse([]byte(`[-6]`));if err!=nil||code!=-6||!megaAPIShouldRetry(code){t.Fatalf("expected retryable -6, code=%d err=%v",code,err)}
	_,code,err=parseMegaInfoResponse([]byte(`-3`));if err!=nil||code!=-3||!megaAPIShouldRetry(code){t.Fatalf("expected retryable direct -3, code=%d err=%v",code,err)}
	_,code,err=parseMegaInfoResponse([]byte(`[-17]`));if err!=nil||code!=-17||megaAPIShouldRetry(code){t.Fatalf("expected non-retryable -17, code=%d err=%v",code,err)}
}
