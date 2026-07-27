package audit

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/response"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/lzyu/QuickEval/apps/api/internal/transport"
)

type Handler struct {
	recorder Recorder
}

func NewHandler(recorder Recorder) Handler {
	return Handler{recorder: recorder}
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

	var actorID *id.UUID
	if value := ctx.Query("actor_user_id"); value != "" {
		parsed, err := id.Parse(value)
		if err != nil {
			response.ApplicationError(ctx, apperror.Validation(
				apperror.FieldError{Field: "actor_user_id", Message: "用户 ID 格式错误"},
			))
			return
		}
		actorID = &parsed
	}

	entries, total, err := handler.recorder.List(
		ctx.Request.Context(),
		page.Page,
		page.PageSize,
		actorID,
		ctx.Query("action"),
		ctx.Query("entity_type"),
		ctx.Query("entity_id"),
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	items := make([]Public, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry.Public())
	}
	response.JSON(ctx, http.StatusOK, transport.PageData[Public]{
		Items:    items,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	})
}
