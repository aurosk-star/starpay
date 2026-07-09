package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	apphandler "payment-gateway/internal/domain/apps/handler"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler apphandler.Handler, userService usersvc.Service, enforcer *casbin.Enforcer) {
	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(userService, enforcer))
	protected.GET("/apps", handler.ListApps)
	protected.GET("/apps/:id", handler.GetApp)
	protected.POST("/apps", handler.CreateApp)
	protected.PUT("/apps/:id", handler.UpdateApp)
	protected.POST("/apps/:id/enable", handler.EnableApp)
	protected.POST("/apps/:id/disable", handler.DisableApp)
	protected.POST("/apps/:id/reset-secret", handler.ResetSecret)
}
