package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	refundhandler "payment-gateway/internal/domain/refunds/handler"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler refundhandler.AdminHandler, users usersvc.Service, enforcer *casbin.Enforcer) {
	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(users, enforcer))
	protected.GET("/refunds", handler.List)
	protected.GET("/refunds/:id", handler.Get)
	protected.POST("/refunds", handler.Create)
	protected.POST("/refunds/:id/retry", handler.Retry)
}
