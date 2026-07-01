package router

import (
	"github.com/gin-gonic/gin"

	orderhandler "payment-gateway/internal/domain/orders/handler"
)

func RegisterCheckout(group *gin.RouterGroup, handler orderhandler.CheckoutHandler) {
	group.GET("/orders/:gateway_order_no", handler.GetOrder)
	group.GET("/orders/:gateway_order_no/methods", handler.ListPaymentMethods)
	group.POST("/orders/:gateway_order_no/pay", handler.StartPayment)
	group.GET("/paypal/return", handler.CompletePaypalPayment)
}
