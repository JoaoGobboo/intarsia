package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/JoaoGobboo/intarsia/internal/server"
)

func TestNew(t *testing.T) {
	const index = "<html>index</html>"
	assets := fstest.MapFS{
		"index.html":    {Data: []byte(index)},
		"assets/app.js": {Data: []byte("console.log('app')")},
	}
	handler := server.New(assets)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{"root serves the index page", "/", http.StatusOK, index},
		{"existing asset is served", "/assets/app.js", http.StatusOK, "console.log('app')"},
		{"client-side route falls back to the index page", "/sources/crm", http.StatusOK, index},
		{"directory is not listed", "/assets", http.StatusOK, index},
		{"missing asset is a 404", "/assets/missing.js", http.StatusNotFound, "404 page not found\n"},
		{"path traversal is rejected", "/../../etc/passwd", http.StatusBadRequest, "invalid URL path\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}
