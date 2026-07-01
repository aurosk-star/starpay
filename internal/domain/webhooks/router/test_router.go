package router

import (
	"github.com/gin-gonic/gin"

	webhookhandler "payment-gateway/internal/domain/webhooks/handler"
)

func RegisterTest(group *gin.RouterGroup, handler *webhookhandler.TestReceiver) {
	test := group.Group("/test/webhook")
	test.POST("/ping", handler.Ping)
	test.GET("/requests", handler.List)
}
