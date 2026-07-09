package routingtest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent"
	"payment-gateway/ent/enttest"
	routinghandler "payment-gateway/internal/domain/routing/handler"
	routingsvc "payment-gateway/internal/domain/routing/service"
)

func TestRoutingHandlerCreatesRuleWithTargets(t *testing.T) {
	router, client, _ := newRoutingHandlerRouter(t, "routing_handler_create")
	accountID := mustCreateChannelAccount(t, client, "alipay", true)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/routing-rules", map[string]any{
		"name":           "CNY 支付宝",
		"enabled":        true,
		"priority":       100,
		"app_scope":      "include",
		"app_ids":        []string{"snsgo", "shop"},
		"payment_method": "alipay",
		"pay_modes":      []string{"page", "wap"},
		"currency":       "CNY",
		"terminal":       "any",
		"targets": []map[string]any{{
			"channel_account_id": accountID,
			"enabled":            true,
			"priority":           100,
			"weight":             100,
		}},
	}))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	data := decodeData(t, recorder)
	rule := data["routing_rule"].(map[string]any)
	if rule["payment_method"] != "alipay" || rule["currency"] != "CNY" {
		t.Fatalf("routing_rule = %#v, want alipay CNY", rule)
	}
	targets := rule["targets"].([]any)
	if len(targets) != 1 {
		t.Fatalf("targets len = %d, want 1", len(targets))
	}
}

func TestRoutingHandlerListsRules(t *testing.T) {
	_, client, service := newRoutingHandlerRouter(t, "routing_handler_list")
	if _, err := service.CreateRule(t.Context(), routingsvc.ManageRuleInput{
		Name:          "PayPal USD",
		Enabled:       true,
		Currency:      "USD",
		Terminal:      "any",
		PaymentMethod: "paypal",
		PayModes:      []string{"checkout"},
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: mustCreateChannelAccount(t, client, "paypal", true),
			Enabled:          true,
		}},
	}); err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}
	router, _, _ := newRoutingHandlerRouterWithService(service)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/routing-rules", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	data := decodeData(t, recorder)
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
}

func TestRoutingHandlerPreviewsRoute(t *testing.T) {
	router, client, service := newRoutingHandlerRouter(t, "routing_handler_preview")
	accountID := mustCreateChannelAccount(t, client, "paypal", true)
	if _, err := service.CreateRule(t.Context(), routingsvc.ManageRuleInput{
		Name:          "PayPal USD",
		Enabled:       true,
		Priority:      100,
		Currency:      "USD",
		Terminal:      "mobile",
		PaymentMethod: "paypal",
		PayModes:      []string{"checkout"},
		Targets: []routingsvc.ManageTargetInput{{
			ChannelAccountID: accountID,
			Enabled:          true,
		}},
	}); err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, jsonRequest(http.MethodPost, "/routing-rules/preview", map[string]any{
		"app_id":         "snsgo",
		"payment_method": "paypal",
		"pay_mode":       "checkout",
		"amount":         1999,
		"currency":       "USD",
		"terminal":       "mobile",
	}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	data := decodeData(t, recorder)
	candidates := data["candidates"].([]any)
	if len(candidates) != 1 {
		t.Fatalf("candidates len = %d, want 1", len(candidates))
	}
	first := candidates[0].(map[string]any)
	if first["channel"] != "paypal" || first["channel_account_id"].(float64) != float64(accountID) {
		t.Fatalf("first = %#v, want paypal account %d", first, accountID)
	}
}

func newRoutingHandlerRouter(t *testing.T, dbName string) (*gin.Engine, *ent.Client, routingsvc.Service) {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:"+dbName+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { client.Close() })
	service := routingsvc.New(client)
	router, _, _ := newRoutingHandlerRouterWithService(service)
	return router, client, service
}

func newRoutingHandlerRouterWithService(service routingsvc.Service) (*gin.Engine, *ent.Client, routingsvc.Service) {
	handler := routinghandler.New(service)
	router := gin.New()
	router.GET("/routing-rules", handler.ListRules)
	router.POST("/routing-rules", handler.CreateRule)
	router.POST("/routing-rules/preview", handler.Preview)
	return router, nil, service
}

func jsonRequest(method string, path string, body map[string]any) *http.Request {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func decodeData(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response["data"].(map[string]any)
}
