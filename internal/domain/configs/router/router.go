package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	confighandler "payment-gateway/internal/domain/configs/handler"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler confighandler.Handler, userService usersvc.Service, enforcer *casbin.Enforcer) {
	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(userService, enforcer))
	protected.GET("/config/gateway", handler.GetGatewayConfig)
	protected.PUT("/config/gateway", handler.UpdateGatewayConfig)
}
