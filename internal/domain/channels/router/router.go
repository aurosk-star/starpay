package router

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	channelhandler "payment-gateway/internal/domain/channels/handler"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/httpx"
)

func Register(group *gin.RouterGroup, handler channelhandler.Handler, userService usersvc.Service, enforcer *casbin.Enforcer) {
	protected := group.Group("")
	protected.Use(httpx.AdminAuthMiddleware(userService, enforcer))
	protected.GET("/channels", handler.ListChannelAccounts)
	protected.GET("/channels/:id", handler.GetChannelAccount)
	protected.POST("/channels", handler.CreateChannelAccount)
	protected.PUT("/channels/:id", handler.UpdateChannelAccount)
	protected.POST("/channels/:id/enable", handler.EnableChannelAccount)
	protected.POST("/channels/:id/disable", handler.DisableChannelAccount)
}
