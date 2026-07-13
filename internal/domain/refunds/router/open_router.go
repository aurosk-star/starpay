package router

import (
	"github.com/gin-gonic/gin"
	refundhandler "payment-gateway/internal/domain/refunds/handler"
)

func RegisterOpen(group *gin.RouterGroup, handler refundhandler.OpenHandler) {
	group.POST("/refunds", handler.Create)
	group.GET("/refunds/:refund_no", handler.Get)
	group.GET("/refunds/by-merchant/:merchant_refund_no", handler.GetByMerchant)
}
