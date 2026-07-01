package webhookstest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"payment-gateway/ent/enttest"
	webhookhandler "payment-gateway/internal/domain/webhooks/handler"
	webhookrepo "payment-gateway/internal/domain/webhooks/repository"
	webhooksvc "payment-gateway/internal/domain/webhooks/service"
)

func TestWebhookAdminHandlerListsGetsAndRetriesDeliveries(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	ctx := context.Background()
	client := enttest.Open(t, dialect.SQLite, "file:webhook_admin_handler?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	createApp(t, client, "snsgo", "https://merchant.example.com/webhooks/payment")
	order := createPaidOrder(t, client, "snsgo", "biz_admin_handler")
	if _, err := webhooksvc.New(client).RecordPaymentSucceeded(ctx, order); err != nil {
		t.Fatalf("RecordPaymentSucceeded() error = %v", err)
	}
	deliveries, _, err := webhookrepo.New(client).ListDeliveries(ctx, webhookrepo.ListDeliveriesInput{})
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	deliveryID := deliveries[0].ID
	if _, err := client.WebhookDelivery.UpdateOneID(deliveryID).SetStatus("failed").SetAttemptCount(3).Save(ctx); err != nil {
		t.Fatalf("seed failed delivery error = %v", err)
	}

	router := gin.New()
	handler := webhookhandler.New(webhooksvc.New(client))
	router.GET("/webhook-deliveries", handler.ListDeliveries)
	router.GET("/webhook-deliveries/:id", handler.GetDelivery)
	router.POST("/webhook-deliveries/:id/retry", handler.RetryDelivery)

	listBody := performJSON(t, router, http.MethodGet, "/webhook-deliveries?status=failed")
	data := listBody["data"].(map[string]any)
	if data["total"].(float64) != 1 {
		t.Fatalf("list data = %#v, want one failed delivery", data)
	}

	detailBody := performJSON(t, router, http.MethodGet, "/webhook-deliveries/"+strconv.Itoa(deliveryID))
	detail := detailBody["data"].(map[string]any)["webhook_delivery"].(map[string]any)
	if int(detail["id"].(float64)) != deliveryID || detail["status"] != "failed" {
		t.Fatalf("detail = %#v, want failed delivery %d", detail, deliveryID)
	}

	retryBody := performJSON(t, router, http.MethodPost, "/webhook-deliveries/"+strconv.Itoa(deliveryID)+"/retry")
	retried := retryBody["data"].(map[string]any)["webhook_delivery"].(map[string]any)
	persisted, err := webhookrepo.New(client).FindDeliveryByID(ctx, deliveryID)
	if err != nil {
		t.Fatalf("FindDeliveryByID() error = %v", err)
	}
	if retried["status"] != "pending" || persisted.AttemptCount != 0 {
		t.Fatalf("retry = %#v, want reset pending delivery", retried)
	}
}

func performJSON(t *testing.T, router *gin.Engine, method string, path string) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d body = %s", method, path, recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "ok" {
		t.Fatalf("response = %#v, want ok", body)
	}
	return body
}
