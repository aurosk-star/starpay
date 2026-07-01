package httpx

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func IntQuery(ctx *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(ctx.Query(key))
	if err != nil {
		return fallback
	}
	return value
}
