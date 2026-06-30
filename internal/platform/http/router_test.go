package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzReturnsOK(t *testing.T) {
	router := NewBaseRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != `{"code":"ok","data":{"status":"ok"},"error":null,"message":"ok"}` {
		t.Fatalf("body = %q, want health JSON", recorder.Body.String())
	}
}
