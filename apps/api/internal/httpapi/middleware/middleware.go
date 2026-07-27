package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/requestid"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/response"
)

func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		startedAt := time.Now()
		ctx.Next()

		logger.InfoContext(
			ctx.Request.Context(),
			"http request",
			"request_id", requestid.From(ctx),
			"method", ctx.Request.Method,
			"path", ctx.Request.URL.Path,
			"status", ctx.Writer.Status(),
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"client_ip", ctx.ClientIP(),
		)
	}
}

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(
					ctx.Request.Context(),
					"panic recovered",
					"request_id", requestid.From(ctx),
					"panic", recovered,
					"stack", string(debug.Stack()),
				)
				response.Error(
					ctx,
					http.StatusInternalServerError,
					"INTERNAL_ERROR",
					"服务暂时不可用，请稍后重试",
				)
			}
		}()
		ctx.Next()
	}
}

func SecurityHeaders() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("X-Content-Type-Options", "nosniff")
		ctx.Header("X-Frame-Options", "DENY")
		ctx.Header("Referrer-Policy", "same-origin")
		ctx.Next()
	}
}
