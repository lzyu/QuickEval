package user

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/access"
	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/audit"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/requestid"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/response"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/lzyu/QuickEval/apps/api/internal/transport"
)

type Handler struct {
	service    Service
	repository Repository
	audit      audit.Recorder
	logger     *slog.Logger
}

func NewHandler(
	service Service,
	repository Repository,
	auditRecorder audit.Recorder,
	logger *slog.Logger,
) Handler {
	return Handler{
		service:    service,
		repository: repository,
		audit:      auditRecorder,
		logger:     logger,
	}
}

func (handler Handler) List(ctx *gin.Context) {
	var page transport.Page
	if err := ctx.ShouldBindQuery(&page); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return
	}
	page = page.Normalize()
	if !page.Valid() {
		response.ApplicationError(ctx, apperror.Validation())
		return
	}
	accounts, total, err := handler.repository.List(
		ctx.Request.Context(),
		page.Page,
		page.PageSize,
		ctx.Query("status"),
		ctx.Query("role"),
		ctx.Query("keyword"),
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	items := make([]Public, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, account.ToPublic())
	}
	response.JSON(ctx, http.StatusOK, transport.PageData[Public]{
		Items:    items,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	})
}

func (handler Handler) Get(ctx *gin.Context) {
	userID, err := parseID(ctx.Param("user_id"))
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	account, err := handler.repository.FindAccountByID(ctx.Request.Context(), userID)
	if err != nil {
		response.ApplicationError(ctx, mapNotFound(err))
		return
	}
	response.JSON(ctx, http.StatusOK, account.ToPublic())
}

type createRequest struct {
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	Email       *string `json:"email"`
	Role        string  `json:"role"`
}

func (handler Handler) Create(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	var request createRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return
	}
	account, err := handler.service.Create(ctx.Request.Context(), principal.ID(), CreateInput{
		Username:    request.Username,
		DisplayName: request.DisplayName,
		Email:       request.Email,
		Role:        request.Role,
	})
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "user.created", account.ID, nil, account.ToPublic())
	response.JSON(ctx, http.StatusCreated, account.ToPublic())
}

type updateRequest struct {
	DisplayName         string  `json:"display_name"`
	Email               *string `json:"email"`
	Role                string  `json:"role"`
	ExpectedLockVersion uint32  `json:"expected_lock_version"`
}

func (handler Handler) Update(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	userID, err := parseID(ctx.Param("user_id"))
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	var request updateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return
	}
	before, after, err := handler.service.Update(
		ctx.Request.Context(),
		principal.ID(),
		userID,
		UpdateInput{
			DisplayName:         request.DisplayName,
			Email:               request.Email,
			Role:                request.Role,
			ExpectedLockVersion: request.ExpectedLockVersion,
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "user.updated", userID, before.ToPublic(), after.ToPublic())
	response.JSON(ctx, http.StatusOK, after.ToPublic())
}

type stateRequest struct {
	ExpectedLockVersion uint32 `json:"expected_lock_version"`
}

func (handler Handler) Disable(ctx *gin.Context) {
	handler.setStatus(ctx, StatusDisabled, "user.disabled")
}

func (handler Handler) Enable(ctx *gin.Context) {
	handler.setStatus(ctx, StatusActive, "user.enabled")
}

func (handler Handler) setStatus(ctx *gin.Context, status, action string) {
	principal, _ := access.From(ctx)
	userID, err := parseID(ctx.Param("user_id"))
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	var request stateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return
	}
	before, after, err := handler.service.SetStatus(
		ctx.Request.Context(),
		principal.ID(),
		userID,
		status,
		request.ExpectedLockVersion,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), action, userID, before.ToPublic(), after.ToPublic())
	response.JSON(ctx, http.StatusOK, after.ToPublic())
}

func (handler Handler) ResetPassword(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	userID, err := parseID(ctx.Param("user_id"))
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	if err := handler.service.ResetPassword(
		ctx.Request.Context(),
		userID,
	); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "user.password_reset", userID, nil, map[string]any{
		"password_reset":           true,
		"password_change_required": true,
	})
	ctx.Status(http.StatusNoContent)
}

func (handler Handler) record(
	ctx *gin.Context,
	actorID id.UUID,
	action string,
	entityID id.UUID,
	before, after any,
) {
	if err := handler.audit.Record(
		ctx.Request.Context(),
		&actorID,
		action,
		"user",
		entityID,
		before,
		after,
		requestid.From(ctx),
		ctx.ClientIP(),
		ctx.Request.UserAgent(),
	); err != nil {
		handler.logger.ErrorContext(ctx.Request.Context(), "write audit log", "error", err)
	}
}

func parseID(value string) (id.UUID, error) {
	userID, err := id.Parse(value)
	if err != nil {
		return id.UUID{}, apperror.NotFound()
	}
	return userID, nil
}
