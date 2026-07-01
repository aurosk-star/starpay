package router

import (
	"github.com/gin-gonic/gin"

	paymenthandler "payment-gateway/internal/domain/payments/handler"
)

func RegisterNotify(group *gin.RouterGroup, handler paymenthandler.NotifyHandler) {
	group.POST("/notify", handler.Handle)
}
