package apperror

import (
	"errors"
	"net/http"
)

type FieldError struct {
	Field   string
	Message string
}

type Error struct {
	Status      int
	Code        string
	Message     string
	FieldErrors []FieldError
	Details     map[string]any
	Cause       error
}

func (err *Error) Error() string {
	if err.Cause != nil {
		return err.Code + ": " + err.Cause.Error()
	}
	return err.Code + ": " + err.Message
}

func (err *Error) Unwrap() error {
	return err.Cause
}

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func Validation(fields ...FieldError) *Error {
	return &Error{
		Status:      http.StatusUnprocessableEntity,
		Code:        "VALIDATION_FAILED",
		Message:     "请检查输入内容",
		FieldErrors: fields,
	}
}

func Unauthorized() *Error {
	return New(http.StatusUnauthorized, "AUTH_REQUIRED", "登录状态已失效，请重新登录")
}

func Forbidden() *Error {
	return New(http.StatusForbidden, "FORBIDDEN", "当前用户无权执行此操作")
}

func NotFound() *Error {
	return New(http.StatusNotFound, "RESOURCE_NOT_FOUND", "请求的资源不存在")
}

func Conflict(code, message string) *Error {
	return New(http.StatusConflict, code, message)
}

func As(err error) *Error {
	var appError *Error
	if errors.As(err, &appError) {
		return appError
	}
	return &Error{
		Status:  http.StatusInternalServerError,
		Code:    "INTERNAL_ERROR",
		Message: "服务暂时不可用，请稍后重试",
		Cause:   err,
	}
}
