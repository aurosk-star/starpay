package httpx

import (
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	usersvc "payment-gateway/internal/domain/users/service"
)

func AdminAuthMiddleware(userService usersvc.Service, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			JSONError(ctx, http.StatusUnauthorized, "unauthorized", "missing access token")
			ctx.Abort()
			return
		}
		claims, err := userService.ParseAccessToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			JSONError(ctx, http.StatusUnauthorized, "unauthorized", "invalid access token")
			ctx.Abort()
			return
		}

		allowed := false
		for _, role := range claims.Roles {
			ok, err := enforcer.Enforce(role, ctx.FullPath(), ctx.Request.Method)
			if err != nil {
				JSONError(ctx, http.StatusInternalServerError, "rbac_error", "permission check failed")
				ctx.Abort()
				return
			}
			if ok {
				allowed = true
				break
			}
		}
		if !allowed {
			JSONError(ctx, http.StatusForbidden, "forbidden", "permission denied")
			ctx.Abort()
			return
		}

		ctx.Set(ContextUserID, claims.UserID)
		ctx.Set(ContextRoles, claims.Roles)
		ctx.Next()
	}
}
