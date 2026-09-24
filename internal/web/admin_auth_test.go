package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dart998/animeav1-archive-ex4100/internal/database"
)

func testAuthServer(t *testing.T)*Server{
	t.Helper();db,e:=database.Open(filepath.Join(t.TempDir(),"auth.sqlite"));if e!=nil{t.Fatal(e)};t.Cleanup(func(){db.Close()});return &Server{db:db}
}

func TestAdminCredentialsAndPersistentSession(t *testing.T){
	s:=testAuthServer(t);if s.adminConfigured(){t.Fatal("fresh db must not be configured")}
	if e:=s.configureAdmin("adminlocal","password-segura");e!=nil{t.Fatal(e)}
	if !s.adminConfigured(){t.Fatal("admin should be configured")}
	if !s.verifyAdminPassword("adminlocal","password-segura"){t.Fatal("valid password rejected")}
	if s.verifyAdminPassword("adminlocal","incorrecta"){t.Fatal("invalid password accepted")}
	now:=time.Unix(1800000000,0);token,e:=s.newAdminSessionToken(now);if e!=nil{t.Fatal(e)}
	if !s.validAdminSession(token,now.Add(24*time.Hour)){t.Fatal("session should still be valid")}
	if s.validAdminSession(token,now.Add(adminSessionDuration+time.Second)){t.Fatal("expired session accepted")}
	s2:=&Server{db:s.db};if !s2.validAdminSession(token,now.Add(48*time.Hour)){t.Fatal("persisted secret should survive server recreation")}
}

func TestRequireAdminRedirectsToSetupThenLogin(t *testing.T){
	s:=testAuthServer(t);h:=s.requireAdmin(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(http.StatusNoContent)})
	r:=httptest.NewRequest(http.MethodGet,"http://nas/admin",nil);w:=httptest.NewRecorder();h(w,r);if w.Code!=http.StatusSeeOther||w.Header().Get("Location")!="/admin/setup"{t.Fatalf("fresh redirect=%d %q",w.Code,w.Header().Get("Location"))}
	if e:=s.configureAdmin("adminlocal","password-segura");e!=nil{t.Fatal(e)}
	r=httptest.NewRequest(http.MethodGet,"http://nas/admin",nil);w=httptest.NewRecorder();h(w,r);if w.Code!=http.StatusSeeOther||!strings.HasPrefix(w.Header().Get("Location"),"/admin/login?"){t.Fatalf("login redirect=%d %q",w.Code,w.Header().Get("Location"))}
}
