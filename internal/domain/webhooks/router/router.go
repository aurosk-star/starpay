package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	usersvc "payment-gateway/internal/domain/users/service"
	webhookhandler "payment-gateway/internal/domain/webhooks/handler"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler webhookhandler.Handler, userService usersvc.Service, enforcer *casbin.Enforcer) {
	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(userService, enforcer))
	protected.GET("/webhook-deliveries", handler.ListDeliveries)
	protected.GET("/webhook-deliveries/:id", handler.GetDelivery)
	protected.POST("/webhook-deliveries/:id/retry", handler.RetryDelivery)
}
