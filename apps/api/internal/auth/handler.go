package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/config"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/response"
)

type Handler struct {
	service    Service
	config     config.Config
	cookieName string
}

func NewHandler(service Service, cfg config.Config) Handler {
	return Handler{
		service:    service,
		config:     cfg,
		cookieName: cfg.Security.SessionCookie,
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (handler Handler) Login(ctx *gin.Context) {
	var request loginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return
	}

	token, session, account, err := handler.service.Login(
		ctx.Request.Context(),
		ctx.ClientIP(),
		request.Username,
		request.Password,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.setCookie(ctx, token, time.Now().Add(handler.config.Security.SessionTTL))
	response.JSON(ctx, http.StatusOK, SessionPayload(account, session, handler.config))
}

func (handler Handler) Session(ctx *gin.Context) {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		response.ApplicationError(ctx, apperror.Unauthorized())
		return
	}
	response.JSON(
		ctx,
		http.StatusOK,
		SessionPayload(principal.User, principal.Session, handler.config),
	)
}

func (handler Handler) Logout(ctx *gin.Context) {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		response.ApplicationError(ctx, apperror.Unauthorized())
		return
	}
	if err := handler.service.Logout(ctx.Request.Context(), principal.Token); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.clearCookie(ctx)
	ctx.Status(http.StatusNoContent)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (handler Handler) ChangePassword(ctx *gin.Context) {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		response.ApplicationError(ctx, apperror.Unauthorized())
		return
	}
	var request changePasswordRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return
	}
	if err := handler.service.ChangePassword(
		ctx.Request.Context(),
		principal,
		request.CurrentPassword,
		request.NewPassword,
	); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.clearCookie(ctx)
	ctx.Status(http.StatusNoContent)
}

func (handler Handler) setCookie(ctx *gin.Context, value string, expires time.Time) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     handler.cookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(handler.config.Security.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   handler.config.Security.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (handler Handler) clearCookie(ctx *gin.Context) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     handler.cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   handler.config.Security.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
