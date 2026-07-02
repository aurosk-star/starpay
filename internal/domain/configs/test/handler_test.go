package configstest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	confighandler "payment-gateway/internal/domain/configs/handler"
	configsvc "payment-gateway/internal/domain/configs/service"
)

func TestPublicSiteConfigReturnsSafeFields(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:gateway_config_public?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	svc := configsvc.New(client)
	if _, err := svc.UpdateGatewayConfig(t.Context(), configsvc.UpdateGatewayConfigInput{
		SiteName:          "绘星支付中心",
		GatewayBaseURL:    "https://pay.example.com",
		PaymentNotifyPath: "/v1/channel/notify",
		DefaultCurrency:   "USD",
		DefaultLocale:     "en",
		RequestIDEnabled:  true,
		MaintenanceMode:   true,
		Extra: map[string]any{
			"secret_note": "hidden",
		},
	}); err != nil {
		t.Fatalf("UpdateGatewayConfig() error = %v", err)
	}

	handler := confighandler.New(svc)
	router := gin.New()
	router.GET("/v1/public/site-config", handler.GetPublicSiteConfig)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/public/site-config", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	want := `{"code":"ok","data":{"site_config":{"default_locale":"en","site_name":"绘星支付中心"}},"error":null,"message":"ok"}`
	if recorder.Body.String() != want {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), want)
	}
}
