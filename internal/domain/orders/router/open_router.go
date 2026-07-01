package router

import (
	"github.com/gin-gonic/gin"

	orderhandler "payment-gateway/internal/domain/orders/handler"
)

func RegisterOpen(group *gin.RouterGroup, handler orderhandler.OpenHandler) {
	group.POST("/orders", handler.CreateOrder)
	group.GET("/orders/:gateway_order_no", handler.GetOrder)
	group.GET("/orders/by-merchant/:merchant_order_no", handler.GetOrderByMerchant)
	group.POST("/orders/:gateway_order_no/close", handler.CloseOrder)
}
