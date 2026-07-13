package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestRegisterServesStaticAssetsAndSPARoutes(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":    {Data: []byte("<html>admin ui</html>")},
		"static/app.js": {Data: []byte("console.log('ok')")},
	}
	router := gin.New()
	router.GET("/v1/ping", func(ctx *gin.Context) { ctx.String(http.StatusOK, "pong") })
	if err := Register(router, assets); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	t.Run("static asset", func(t *testing.T) {
		response := performRequest(router, http.MethodGet, "/static/app.js")
		if response.Code != http.StatusOK || response.Body.String() != "console.log('ok')" {
			t.Fatalf("response = %d %q, want static asset", response.Code, response.Body.String())
		}
		if got := response.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Fatalf("Cache-Control = %q, want immutable", got)
		}
	})

	t.Run("SPA route", func(t *testing.T) {
		response := performRequest(router, http.MethodGet, "/orders/42")
		if response.Code != http.StatusOK || response.Body.String() != "<html>admin ui</html>" {
			t.Fatalf("response = %d %q, want index", response.Code, response.Body.String())
		}
		if got := response.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("Cache-Control = %q, want no-cache", got)
		}
	})

	t.Run("registered API", func(t *testing.T) {
		response := performRequest(router, http.MethodGet, "/v1/ping")
		if response.Code != http.StatusOK || response.Body.String() != "pong" {
			t.Fatalf("response = %d %q, want API", response.Code, response.Body.String())
		}
	})

	t.Run("unknown API", func(t *testing.T) {
		response := performRequest(router, http.MethodGet, "/v1/missing")
		if response.Code != http.StatusNotFound || response.Body.String() == "<html>admin ui</html>" {
			t.Fatalf("response = %d %q, want API 404", response.Code, response.Body.String())
		}
	})

	t.Run("missing asset", func(t *testing.T) {
		response := performRequest(router, http.MethodGet, "/static/missing.js")
		if response.Code != http.StatusNotFound || response.Body.String() == "<html>admin ui</html>" {
			t.Fatalf("response = %d %q, want asset 404", response.Code, response.Body.String())
		}
	})

	t.Run("non read method", func(t *testing.T) {
		response := performRequest(router, http.MethodPost, "/orders")
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", response.Code)
		}
	})

	t.Run("HEAD route", func(t *testing.T) {
		response := performRequest(router, http.MethodHead, "/refunds")
		if response.Code != http.StatusOK || response.Body.Len() != 0 {
			t.Fatalf("response = %d %q, want empty HEAD response", response.Code, response.Body.String())
		}
	})
}

func TestRegisterRequiresIndex(t *testing.T) {
	router := gin.New()
	err := Register(router, fstest.MapFS{"static/app.js": {Data: []byte("ok")}})
	if err == nil {
		t.Fatal("Register() error = nil, want missing index error")
	}
}

func performRequest(handler http.Handler, method string, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}
