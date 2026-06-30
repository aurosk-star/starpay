package httpx

import "github.com/gin-gonic/gin"

const (
	ContextUserID    = "user_id"
	ContextRoles     = "roles"
	ContextAppID     = "app_id"
	ContextAppDBID   = "app_db_id"
	ContextRequestID = "request_id"
)

func JSONError(ctx *gin.Context, status int, code string, message string) {
	ctx.JSON(status, gin.H{
		"code":    code,
		"message": message,
		"data":    nil,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}

func JSONOK(ctx *gin.Context, status int, data any) {
	ctx.JSON(status, gin.H{
		"code":    "ok",
		"message": "ok",
		"data":    data,
		"error":   nil,
	})
}

func JSONNoContent(ctx *gin.Context) {
	ctx.JSON(200, gin.H{
		"code":    "ok",
		"message": "ok",
		"data":    gin.H{},
		"error":   nil,
	})
}
