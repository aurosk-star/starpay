package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	orderhandler "payment-gateway/internal/domain/orders/handler"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler orderhandler.Handler, userService usersvc.Service, enforcer *casbin.Enforcer) {
	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(userService, enforcer))
	protected.GET("/orders", handler.ListOrders)
	protected.GET("/orders/:id", handler.GetOrder)
	protected.POST("/orders", handler.CreateOrder)
	protected.PUT("/orders/:id", handler.UpdateOrder)
	protected.POST("/orders/:id/close", handler.CloseOrder)
}
