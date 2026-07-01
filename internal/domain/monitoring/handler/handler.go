package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	monitorsvc "payment-gateway/internal/domain/monitoring/service"
	"payment-gateway/internal/platform/httpx"
)

type Handler struct {
	service monitorsvc.Service
}

func New(service monitorsvc.Service) Handler {
	return Handler{service: service}
}

func (h Handler) Overview(ctx *gin.Context) {
	httpx.JSONOK(ctx, http.StatusOK, gin.H{
		"monitoring": h.service.Overview(ctx.Request.Context()),
	})
}
