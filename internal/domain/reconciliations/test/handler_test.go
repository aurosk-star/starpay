package reconciliationstest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	reconciliationhandler "payment-gateway/internal/domain/reconciliations/handler"
	reconciliationsvc "payment-gateway/internal/domain/reconciliations/service"
)

func TestAdminHandlerListsGetsAndRetriesReconciliation(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	client := enttest.Open(t, dialect.SQLite, "file:reconciliation_handler?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	item, err := client.PaymentReconciliation.Create().SetPaymentOrderID(1).SetGatewayOrderNo("gw_1").SetChannel("alipay").SetChannelAccountID(2).SetStatus("manual_required").SetAttemptCount(8).SetProviderSnapshot(map[string]any{}).Save(t.Context())
	if err != nil {
		t.Fatalf("create reconciliation error = %v", err)
	}
	handler := reconciliationhandler.New(reconciliationsvc.New(client, reconciliationsvc.WithNow(func() time.Time { return now })))
	router := gin.New()
	router.GET("/payment-reconciliations", handler.List)
	router.GET("/payment-reconciliations/:id", handler.Get)
	router.POST("/payment-reconciliations/:id/retry", handler.Retry)

	list := performReconciliationJSON(t, router, http.MethodGet, "/payment-reconciliations?status=manual_required")
	if list["data"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("list = %#v, want one item", list)
	}
	detail := performReconciliationJSON(t, router, http.MethodGet, "/payment-reconciliations/"+strconv.Itoa(item.ID))
	if detail["data"].(map[string]any)["payment_reconciliation"].(map[string]any)["status"] != "manual_required" {
		t.Fatalf("detail = %#v", detail)
	}
	retried := performReconciliationJSON(t, router, http.MethodPost, "/payment-reconciliations/"+strconv.Itoa(item.ID)+"/retry")
	if retried["data"].(map[string]any)["payment_reconciliation"].(map[string]any)["status"] != "pending" {
		t.Fatalf("retry = %#v, want pending", retried)
	}
}

func performReconciliationJSON(t *testing.T, router *gin.Engine, method string, path string) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s %s status=%d body=%s", method, path, recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response error = %v", err)
	}
	return body
}
