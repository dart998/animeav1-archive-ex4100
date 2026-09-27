package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteAdminActionErrorJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/admin/sync-av1", nil)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	writeAdminActionError(w, r, errTestAdminAction("database is locked"))
	if w.Code != http.StatusInternalServerError { t.Fatalf("status=%d", w.Code) }
	if !strings.Contains(w.Body.String(), "database is locked") { t.Fatalf("body=%q", w.Body.String()) }
	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") { t.Fatalf("content-type=%q", w.Header().Get("Content-Type")) }
}

type errTestAdminAction string
func (e errTestAdminAction) Error() string { return string(e) }
