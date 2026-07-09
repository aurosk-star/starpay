package appstest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	apphandler "payment-gateway/internal/domain/apps/handler"
	appsvc "payment-gateway/internal/domain/apps/service"
)

func TestCreateAppHandlerGeneratesAppID(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:create_app_handler?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	router := gin.New()
	handler := apphandler.New(appsvc.New(client))
	router.POST("/apps", handler.CreateApp)

	body := []byte(`{"name":"Demo App","status":"enabled","allowed_ips":[]}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/apps", bytes.NewReader(body)))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			App struct {
				AppID string `json:"app_id"`
			} `json:"app"`
			AppSecret string `json:"app_secret"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !strings.HasPrefix(response.Data.App.AppID, "app_") {
		t.Fatalf("app_id = %q, want generated app_ prefix", response.Data.App.AppID)
	}
	if response.Data.AppSecret == "" {
		t.Fatal("app_secret is empty")
	}
}

func TestGetAppHandlerReturnsAppDetail(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:get_app_handler?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	service := appsvc.New(client)
	created, err := service.CreateApp(t.Context(), appsvc.ManageAppInput{
		AppID:            "app_detail",
		Name:             "Detail App",
		NotifyURL:        "https://merchant.example.com/webhook",
		DefaultReturnURL: "https://merchant.example.com/pay/return",
		AllowedIPs:       []string{"10.0.0.8"},
		Status:           "enabled",
	})
	if err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}

	router := gin.New()
	handler := apphandler.New(service)
	router.GET("/apps/:id", handler.GetApp)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/apps/"+strconv.Itoa(created.App.ID), nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			App struct {
				AppID string `json:"app_id"`
				Name  string `json:"name"`
			} `json:"app"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Data.App.AppID != "app_detail" || response.Data.App.Name != "Detail App" {
		t.Fatalf("app = %#v", response.Data.App)
	}
}
