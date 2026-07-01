package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"payment-gateway/internal/platform/httpx"
)

type TestReceiver struct {
	mu       sync.Mutex
	requests []TestWebhookRequest
}

type TestWebhookRequest struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
	At      time.Time           `json:"at"`
}

func NewTestReceiver() *TestReceiver {
	return &TestReceiver{}
}

func (r *TestReceiver) Ping(ctx *gin.Context) {
	body := ctx.GetString(gin.BodyBytesKey)
	if body == "" && ctx.Request.Body != nil {
		raw, _ := ctx.GetRawData()
		body = string(raw)
	}
	r.mu.Lock()
	r.requests = append(r.requests, TestWebhookRequest{
		Method:  ctx.Request.Method,
		Path:    ctx.Request.URL.Path,
		Headers: ctx.Request.Header.Clone(),
		Body:    body,
		At:      time.Now().UTC(),
	})
	r.mu.Unlock()
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"saved": true})
}

func (r *TestReceiver) List(ctx *gin.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]TestWebhookRequest, len(r.requests))
	copy(items, r.requests)
	httpx.JSONOK(ctx, http.StatusOK, gin.H{
		"items": items,
		"total": len(items),
	})
}
