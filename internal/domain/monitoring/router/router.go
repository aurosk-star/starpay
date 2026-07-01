package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	monitorhandler "payment-gateway/internal/domain/monitoring/handler"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler monitorhandler.Handler, userService usersvc.Service, enforcer *casbin.Enforcer) {
	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(userService, enforcer))
	protected.GET("/monitoring/overview", handler.Overview)
}
