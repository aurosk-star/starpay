package httpx

import "github.com/gin-gonic/gin"

const (
	ContextUserID = "user_id"
	ContextRoles  = "roles"
)

func JSONError(ctx *gin.Context, status int, code string, message string) {
	ctx.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
