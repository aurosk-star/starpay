package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	reconciliationhandler "payment-gateway/internal/domain/reconciliations/handler"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler reconciliationhandler.Handler, users usersvc.Service, enforcer *casbin.Enforcer) {
	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(users, enforcer))
	protected.GET("/payment-reconciliations", handler.List)
	protected.GET("/payment-reconciliations/:id", handler.Get)
	protected.POST("/payment-reconciliations/:id/retry", handler.Retry)
}
