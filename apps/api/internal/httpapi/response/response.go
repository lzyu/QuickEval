package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/requestid"
)

type Meta struct {
	RequestID string `json:"request_id"`
}

type Success struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ErrorBody struct {
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	FieldErrors []FieldError   `json:"field_errors"`
	Details     map[string]any `json:"details,omitempty"`
}

type Failure struct {
	Error ErrorBody `json:"error"`
	Meta  Meta      `json:"meta"`
}

func JSON(ctx *gin.Context, status int, data any) {
	ctx.JSON(status, Success{
		Data: data,
		Meta: Meta{RequestID: requestid.From(ctx)},
	})
}

func Error(ctx *gin.Context, status int, code, message string) {
	ctx.AbortWithStatusJSON(status, Failure{
		Error: ErrorBody{
			Code:        code,
			Message:     message,
			FieldErrors: []FieldError{},
		},
		Meta: Meta{RequestID: requestid.From(ctx)},
	})
}

func ApplicationError(ctx *gin.Context, err error) {
	appError := apperror.As(err)
	fieldErrors := make([]FieldError, 0, len(appError.FieldErrors))
	for _, fieldError := range appError.FieldErrors {
		fieldErrors = append(fieldErrors, FieldError{
			Field:   fieldError.Field,
			Message: fieldError.Message,
		})
	}
	ctx.AbortWithStatusJSON(appError.Status, Failure{
		Error: ErrorBody{
			Code:        appError.Code,
			Message:     appError.Message,
			FieldErrors: fieldErrors,
			Details:     appError.Details,
		},
		Meta: Meta{RequestID: requestid.From(ctx)},
	})
}

func NotFound(ctx *gin.Context) {
	Error(ctx, http.StatusNotFound, "RESOURCE_NOT_FOUND", "请求的资源不存在")
}
