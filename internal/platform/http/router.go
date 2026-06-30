package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"payment-gateway/ent"
	userhandler "payment-gateway/internal/domain/users/handler"
	userrouter "payment-gateway/internal/domain/users/router"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/config"
	"payment-gateway/internal/platform/rbac"
)

func NewRouter(client *ent.Client, cfg config.Config) http.Handler {
	router := NewBaseRouter()

	enforcer, err := rbac.NewEnforcer()
	if err != nil {
		panic(err)
	}
	userService := usersvc.New(client, cfg.Auth)
	userHandler := userhandler.New(userService, cfg.Auth)
	userrouter.Register(router.Group("/v1/admin"), userHandler, userService, enforcer)

	return router
}

func NewBaseRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := router.Group("/v1")
	v1.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	return router
}
