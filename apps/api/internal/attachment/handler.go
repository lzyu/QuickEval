package attachment

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/access"
	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/response"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

type Handler struct {
	service        Service
	idempotency    IdempotencyStore
	logger         *slog.Logger
	maxRequestSize int64
}

func NewHandler(
	service Service,
	idempotency IdempotencyStore,
	logger *slog.Logger,
	maxFileSize int64,
	maxFiles int,
) Handler {
	return Handler{
		service: service, idempotency: idempotency, logger: logger,
		maxRequestSize: int64(maxFiles)*maxFileSize + 2*1024*1024,
	}
}

func (handler Handler) UploadResult(ctx *gin.Context) {
	handler.upload(ctx, "result", "result_id")
}

func (handler Handler) UploadBadcase(ctx *gin.Context) {
	handler.upload(ctx, "badcase", "badcase_id")
}

func (handler Handler) upload(ctx *gin.Context, kind, key string) {
	ownerID, ok := attachmentPathID(ctx, key)
	if !ok {
		return
	}
	principal, _ := access.From(ctx)
	idempotencyKey := ctx.GetHeader("Idempotency-Key")
	if len(idempotencyKey) > 200 {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "Idempotency-Key", Message: "幂等键最多 200 个字符"},
		))
		return
	}
	if idempotencyKey != "" {
		receipt, err := handler.idempotency.Reserve(
			ctx.Request.Context(), principal.ID(), kind, ownerID, idempotencyKey,
		)
		if errors.Is(err, ErrRequestInProgress) {
			response.ApplicationError(ctx, apperror.Conflict(
				"REQUEST_IN_PROGRESS", "相同上传正在处理中，请稍后重试",
			))
			return
		}
		if err != nil {
			response.ApplicationError(ctx, err)
			return
		}
		if receipt != nil {
			response.JSON(ctx, http.StatusOK, receipt)
			return
		}
	}
	release := func() {
		if idempotencyKey != "" {
			handler.idempotency.Release(
				ctx.Request.Context(), principal.ID(), kind, ownerID, idempotencyKey,
			)
		}
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, handler.maxRequestSize)
	if err := ctx.Request.ParseMultipartForm(32 << 20); err != nil {
		release()
		response.ApplicationError(ctx, apperror.New(
			http.StatusRequestEntityTooLarge,
			"ATTACHMENT_LIMIT_EXCEEDED",
			"上传请求过大",
		))
		return
	}
	defer ctx.Request.MultipartForm.RemoveAll()
	expected, err := strconv.ParseUint(ctx.Request.FormValue("expected_owner_lock_version"), 10, 32)
	if err != nil {
		release()
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{
				Field: "expected_owner_lock_version", Message: "缺少有效所属记录锁版本",
			},
		))
		return
	}
	files := ctx.Request.MultipartForm.File["files"]
	uploads := make([]Upload, 0, len(files))
	for _, file := range files {
		upload, err := handler.service.Stage(file)
		if err != nil {
			for _, staged := range uploads {
				handler.service.storage.RemoveTemp(staged.TempPath)
			}
			release()
			response.ApplicationError(ctx, err)
			return
		}
		uploads = append(uploads, upload)
	}
	var items []Attachment
	var ownerVersion uint32
	if kind == "result" {
		items, ownerVersion, err = handler.service.UploadResult(
			ctx.Request.Context(), principal.ID(), principal.Admin(), ownerID,
			uint32(expected), uploads,
		)
	} else {
		items, ownerVersion, err = handler.service.UploadBadcase(
			ctx.Request.Context(), principal.ID(), principal.Admin(), ownerID,
			uint32(expected), uploads,
		)
	}
	if err != nil {
		release()
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]Public, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	receipt := UploadReceipt{Items: publicItems, OwnerLockVersion: ownerVersion}
	if idempotencyKey != "" {
		if err := handler.idempotency.Commit(
			ctx.Request.Context(), principal.ID(), kind, ownerID, idempotencyKey, receipt,
		); err != nil {
			handler.logger.Error("commit attachment idempotency key", "error", err)
		}
	}
	response.JSON(ctx, http.StatusCreated, receipt)
}

func (handler Handler) Content(ctx *gin.Context) {
	attachmentID, ok := attachmentPathID(ctx, "attachment_id")
	if !ok {
		return
	}
	principal, _ := access.From(ctx)
	item, err := handler.service.Content(
		ctx.Request.Context(), principal.ID(), principal.Admin(), attachmentID,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	file, err := handler.service.Open(item)
	if err != nil {
		handler.logger.Error("open attachment content", "attachment_id", item.ID.String(), "error", err)
		response.ApplicationError(ctx, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	ctx.Header("Content-Type", item.MediaType)
	ctx.Header("Cache-Control", "private, max-age=3600")
	ctx.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(ctx.Writer, ctx.Request, item.OriginalName, info.ModTime(), file)
}

func (handler Handler) Delete(ctx *gin.Context) {
	attachmentID, ok := attachmentPathID(ctx, "attachment_id")
	if !ok {
		return
	}
	expected, err := strconv.ParseUint(ctx.Query("expected_owner_lock_version"), 10, 32)
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{
				Field: "expected_owner_lock_version", Message: "缺少有效所属记录锁版本",
			},
		))
		return
	}
	principal, _ := access.From(ctx)
	ownerVersion, err := handler.service.Delete(
		ctx.Request.Context(), principal.ID(), principal.Admin(), attachmentID, uint32(expected),
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, map[string]any{"owner_lock_version": ownerVersion})
}

type reorderRequest struct {
	ExpectedOwnerLockVersion uint32 `json:"expected_owner_lock_version"`
	Items                    []struct {
		ID        string `json:"id"`
		SortOrder uint32 `json:"sort_order"`
	} `json:"items"`
}

func (handler Handler) ReorderResult(ctx *gin.Context) {
	handler.reorder(ctx, "result", "result_id")
}

func (handler Handler) ReorderBadcase(ctx *gin.Context) {
	handler.reorder(ctx, "badcase", "badcase_id")
}

func (handler Handler) reorder(ctx *gin.Context, kind, key string) {
	ownerID, ok := attachmentPathID(ctx, key)
	if !ok {
		return
	}
	var request reorderRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return
	}
	items := make([]ReorderItem, 0, len(request.Items))
	for _, item := range request.Items {
		itemID, err := id.Parse(item.ID)
		if err != nil {
			response.ApplicationError(ctx, apperror.Validation(
				apperror.FieldError{Field: "items", Message: "附件 ID 格式错误"},
			))
			return
		}
		items = append(items, ReorderItem{ID: itemID, SortOrder: item.SortOrder})
	}
	principal, _ := access.From(ctx)
	ownerVersion, err := handler.service.Reorder(
		ctx.Request.Context(), principal.ID(), principal.Admin(), kind, ownerID,
		request.ExpectedOwnerLockVersion, items,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, map[string]any{"owner_lock_version": ownerVersion})
}

func attachmentPathID(ctx *gin.Context, key string) (id.UUID, bool) {
	value, err := id.Parse(ctx.Param(key))
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: key, Message: "ID 格式错误"},
		))
		return id.UUID{}, false
	}
	return value, true
}
