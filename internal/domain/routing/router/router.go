package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	routinghandler "payment-gateway/internal/domain/routing/handler"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler routinghandler.Handler, userService usersvc.Service, enforcer *casbin.Enforcer) {
	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(userService, enforcer))
	protected.GET("/routing-rules", handler.ListRules)
	protected.POST("/routing-rules/preview", handler.Preview)
	protected.GET("/routing-rules/:id", handler.GetRule)
	protected.POST("/routing-rules", handler.CreateRule)
	protected.PUT("/routing-rules/:id", handler.UpdateRule)
	protected.POST("/routing-rules/:id/enable", handler.EnableRule)
	protected.POST("/routing-rules/:id/disable", handler.DisableRule)
}
