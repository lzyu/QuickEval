package requestid

import (
	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

const (
	Header = "X-Request-ID"
	key    = "request_id"
)

func Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader(Header)
		if requestID == "" || len(requestID) > 64 {
			requestID = id.MustNew().String()
		}

		ctx.Set(key, requestID)
		ctx.Header(Header, requestID)
		ctx.Next()
	}
}

func From(ctx *gin.Context) string {
	value, exists := ctx.Get(key)
	if !exists {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}
