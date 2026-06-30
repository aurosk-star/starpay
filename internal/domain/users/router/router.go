package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	userhandler "payment-gateway/internal/domain/users/handler"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler userhandler.Handler, service usersvc.Service, enforcer *casbin.Enforcer) {
	group.POST("/setup", handler.Setup)

	auth := group.Group("/auth")
	auth.POST("/login", handler.Login)
	auth.POST("/refresh", handler.Refresh)
	auth.POST("/logout", handler.Logout)

	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(service, enforcer))
	protected.GET("/auth/me", handler.Me)
	protected.GET("/users", handler.ListUsers)
	protected.GET("/roles", handler.ListRoles)
}
