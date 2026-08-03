package auth

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/access"
	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/response"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/lzyu/QuickEval/apps/api/internal/user"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Middleware struct {
	cookieName string
	sessions   SessionStore
	users      user.Repository
}

func NewMiddleware(
	cookieName string,
	sessions SessionStore,
	users user.Repository,
) Middleware {
	return Middleware{
		cookieName: cookieName,
		sessions:   sessions,
		users:      users,
	}
}

func (middleware Middleware) Required() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie(middleware.cookieName)
		if err != nil || token == "" {
			response.ApplicationError(ctx, apperror.Unauthorized())
			return
		}
		session, err := middleware.sessions.Load(ctx.Request.Context(), token)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				response.ApplicationError(ctx, apperror.Unauthorized())
			} else {
				response.ApplicationError(ctx, err)
			}
			return
		}
		userID, err := id.Parse(session.UserID)
		if err != nil {
			_ = middleware.sessions.Revoke(ctx.Request.Context(), token)
			response.ApplicationError(ctx, apperror.Unauthorized())
			return
		}
		account, err := middleware.users.FindAccountByID(ctx.Request.Context(), userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				_ = middleware.sessions.Revoke(ctx.Request.Context(), token)
				response.ApplicationError(ctx, apperror.Unauthorized())
			} else {
				response.ApplicationError(ctx, err)
			}
			return
		}
		if !account.IsActive() {
			_ = middleware.sessions.Revoke(ctx.Request.Context(), token)
			response.ApplicationError(ctx, apperror.Unauthorized())
			return
		}

		SetPrincipal(ctx, Principal{User: account, Session: session, Token: token})
		access.Set(ctx, access.Principal{UserID: account.ID, Role: account.Role})
		ctx.Next()
	}
}

func (middleware Middleware) CSRF() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.Method == http.MethodGet ||
			ctx.Request.Method == http.MethodHead ||
			ctx.Request.Method == http.MethodOptions {
			ctx.Next()
			return
		}

		principal, ok := PrincipalFrom(ctx)
		if !ok {
			response.ApplicationError(ctx, apperror.Unauthorized())
			return
		}
		provided := ctx.GetHeader("X-CSRF-Token")
		if len(provided) != len(principal.Session.CSRFToken) ||
			subtle.ConstantTimeCompare([]byte(provided), []byte(principal.Session.CSRFToken)) != 1 {
			response.ApplicationError(
				ctx,
				apperror.New(http.StatusForbidden, "CSRF_INVALID", "安全校验失败，请刷新页面后重试"),
			)
			return
		}
		ctx.Next()
	}
}

func RequireOperationsAdmin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal, ok := PrincipalFrom(ctx)
		if !ok {
			response.ApplicationError(ctx, apperror.Unauthorized())
			return
		}
		if !principal.Admin() {
			response.ApplicationError(ctx, apperror.Forbidden())
			return
		}
		ctx.Next()
	}
}

// RequireAdmin keeps existing business-handler terminology compatible while
// requiring either an operations administrator or a super administrator.
func RequireAdmin() gin.HandlerFunc {
	return RequireOperationsAdmin()
}

func RequireSuperAdmin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal, ok := PrincipalFrom(ctx)
		if !ok {
			response.ApplicationError(ctx, apperror.Unauthorized())
			return
		}
		if !principal.SuperAdmin() {
			response.ApplicationError(ctx, apperror.Forbidden())
			return
		}
		ctx.Next()
	}
}

func RequirePasswordChangeComplete() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal, ok := PrincipalFrom(ctx)
		if !ok {
			response.ApplicationError(ctx, apperror.Unauthorized())
			return
		}
		if principal.User.PasswordChangeRequired {
			response.ApplicationError(ctx, apperror.New(
				http.StatusForbidden,
				"PASSWORD_CHANGE_REQUIRED",
				"请先修改初始密码后再使用系统",
			))
			return
		}
		ctx.Next()
	}
}
