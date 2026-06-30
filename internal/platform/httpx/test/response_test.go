package httpxtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"payment-gateway/internal/platform/httpx"
)

func TestJSONOKUsesGlobalResponseShape(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/ok", func(ctx *gin.Context) {
		httpx.JSONOK(ctx, http.StatusOK, gin.H{"name": "gateway"})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ok", nil))

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "ok" || body["message"] != "ok" {
		t.Fatalf("response = %#v, want global success shape", body)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["name"] != "gateway" {
		t.Fatalf("data = %#v, want payload under data", body["data"])
	}
}

func TestJSONErrorUsesGlobalResponseShape(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/bad", func(ctx *gin.Context) {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", "bad request")
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/bad", nil))

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "invalid_request" || body["message"] != "bad request" {
		t.Fatalf("response = %#v, want top-level error code and message", body)
	}
	errorBody, ok := body["error"].(map[string]any)
	if !ok || errorBody["code"] != "invalid_request" {
		t.Fatalf("error = %#v, want nested error detail", body["error"])
	}
}
