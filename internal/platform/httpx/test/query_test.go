package httpxtest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"payment-gateway/internal/platform/httpx"
)

func TestIntQueryReturnsParsedValue(t *testing.T) {
	ctx := queryContext("/test?page=3")

	if got := httpx.IntQuery(ctx, "page", 1); got != 3 {
		t.Fatalf("IntQuery() = %d, want 3", got)
	}
}

func TestIntQueryReturnsFallbackForMissingOrInvalidValue(t *testing.T) {
	ctx := queryContext("/test?page=bad")

	if got := httpx.IntQuery(ctx, "page", 1); got != 1 {
		t.Fatalf("IntQuery(invalid) = %d, want fallback", got)
	}
	if got := httpx.IntQuery(ctx, "page_size", 20); got != 20 {
		t.Fatalf("IntQuery(missing) = %d, want fallback", got)
	}
}

func queryContext(target string) *gin.Context {
	gin.SetMode(gin.ReleaseMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	return ctx
}
