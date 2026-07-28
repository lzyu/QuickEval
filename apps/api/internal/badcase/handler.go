package badcase

import (
	"errors"
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
	service     Service
	repository  Repository
	idempotency IdempotencyStore
	audit       audit.Recorder
	logger      *slog.Logger
}

func NewHandler(
	service Service,
	repository Repository,
	idempotency IdempotencyStore,
	auditRecorder audit.Recorder,
	logger *slog.Logger,
) Handler {
	return Handler{
		service: service, repository: repository, idempotency: idempotency,
		audit: auditRecorder, logger: logger,
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
	scenarioID, ok := optionalID(ctx, "scenario_id")
	if !ok {
		return
	}
	issueTagID, ok := optionalID(ctx, "issue_tag_id")
	if !ok {
		return
	}
	items, total, err := handler.repository.List(
		ctx.Request.Context(), page.Page, page.PageSize,
		Filters{
			Status: ctx.Query("status"), ScenarioID: scenarioID,
			IssueTagID: issueTagID, Keyword: ctx.Query("keyword"),
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]Public, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, transport.PageData[Public]{
		Items: publicItems, Page: page.Page, PageSize: page.PageSize, Total: total,
	})
}

func (handler Handler) Get(ctx *gin.Context) {
	badcaseID, ok := pathID(ctx, "badcase_id")
	if !ok {
		return
	}
	item, err := handler.repository.Get(ctx.Request.Context(), badcaseID)
	if err != nil {
		response.ApplicationError(ctx, mapNotFound(err))
		return
	}
	if item.InvalidatedAt != nil {
		response.ApplicationError(ctx, apperror.NotFound())
		return
	}
	response.JSON(ctx, http.StatusOK, item.Public())
}

type resultPatchRequest struct {
	Status     string  `json:"status"`
	AnswerText *string `json:"answer_text"`
	Score      *uint8  `json:"score"`
	Comment    *string `json:"comment"`
}

type markRequest struct {
	ExpectedResultLockVersion uint32              `json:"expected_result_lock_version"`
	ResultPatch               *resultPatchRequest `json:"result_patch"`
	Badcase                   struct {
		Title       string   `json:"title"`
		Description *string  `json:"description"`
		IssueTagIDs []string `json:"issue_tag_ids"`
	} `json:"badcase"`
}

func (handler Handler) MarkEvaluation(ctx *gin.Context) {
	resultID, ok := pathID(ctx, "result_id")
	if !ok {
		return
	}
	principal, _ := access.From(ctx)
	key := ctx.GetHeader("Idempotency-Key")
	if len(key) > 200 {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "Idempotency-Key", Message: "幂等键最多 200 个字符"},
		))
		return
	}
	if key != "" {
		existingID, err := handler.idempotency.Reserve(
			ctx.Request.Context(), principal.ID(), key,
		)
		if errors.Is(err, ErrRequestInProgress) {
			response.ApplicationError(ctx, apperror.Conflict(
				"REQUEST_IN_PROGRESS", "相同请求正在处理中，请稍后重试",
			))
			return
		}
		if err != nil {
			response.ApplicationError(ctx, err)
			return
		}
		if existingID != nil {
			handler.respondMark(ctx, http.StatusOK, *existingID)
			return
		}
	}
	var request markRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		handler.release(ctx, principal.ID(), key)
		response.ApplicationError(ctx, apperror.Validation())
		return
	}
	tagIDs := make([]id.UUID, 0, len(request.Badcase.IssueTagIDs))
	for _, rawID := range request.Badcase.IssueTagIDs {
		tagID, err := id.Parse(rawID)
		if err != nil {
			handler.release(ctx, principal.ID(), key)
			response.ApplicationError(ctx, apperror.Validation(
				apperror.FieldError{Field: "badcase.issue_tag_ids", Message: "问题标签 ID 格式错误"},
			))
			return
		}
		tagIDs = append(tagIDs, tagID)
	}
	var patch *ResultPatch
	if request.ResultPatch != nil {
		patch = &ResultPatch{
			Status: request.ResultPatch.Status, AnswerText: request.ResultPatch.AnswerText,
			Score: request.ResultPatch.Score, Comment: request.ResultPatch.Comment,
		}
	}
	result, err := handler.service.MarkEvaluation(
		ctx.Request.Context(), principal.ID(), principal.Admin(), resultID,
		MarkInput{
			ExpectedResultLockVersion: request.ExpectedResultLockVersion,
			ResultPatch:               patch, Title: request.Badcase.Title,
			Description: request.Badcase.Description, IssueTagIDs: tagIDs,
		},
	)
	if err != nil {
		handler.release(ctx, principal.ID(), key)
		response.ApplicationError(ctx, err)
		return
	}
	if key != "" {
		if err := handler.idempotency.Commit(
			ctx.Request.Context(), principal.ID(), key, result.Badcase.ID,
		); err != nil {
			handler.logger.Error("commit badcase idempotency key", "error", err)
		}
	}
	handler.record(ctx, principal.ID(), result.Badcase.ID, result.Badcase.Public())
	handler.writeMark(ctx, http.StatusCreated, result)
}

func (handler Handler) respondMark(ctx *gin.Context, status int, badcaseID id.UUID) {
	result, err := handler.service.LoadMarkResult(ctx.Request.Context(), badcaseID)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.writeMark(ctx, status, result)
}

func (handler Handler) writeMark(ctx *gin.Context, status int, result MarkResult) {
	response.JSON(ctx, status, map[string]any{
		"badcase":          result.Badcase.Public(),
		"result":           result.Result.Public(),
		"progress":         result.Run.Public().Progress,
		"run_lock_version": result.Run.LockVersion,
	})
}

func (handler Handler) release(ctx *gin.Context, actorID id.UUID, key string) {
	if key != "" {
		handler.idempotency.Release(ctx.Request.Context(), actorID, key)
	}
}

func (handler Handler) record(
	ctx *gin.Context,
	actorID, badcaseID id.UUID,
	after Public,
) {
	if err := handler.audit.Record(
		ctx.Request.Context(), &actorID, "badcase.created", "badcase", badcaseID,
		nil, after, requestid.From(ctx), ctx.ClientIP(), ctx.Request.UserAgent(),
	); err != nil {
		handler.logger.Error("record audit log", "action", "badcase.created", "error", err)
	}
}

func optionalID(ctx *gin.Context, key string) (*id.UUID, bool) {
	if ctx.Query(key) == "" {
		return nil, true
	}
	value, ok := parseID(ctx, ctx.Query(key), key)
	if !ok {
		return nil, false
	}
	return &value, true
}

func pathID(ctx *gin.Context, key string) (id.UUID, bool) {
	return parseID(ctx, ctx.Param(key), key)
}

func parseID(ctx *gin.Context, value, field string) (id.UUID, bool) {
	parsed, err := id.Parse(value)
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: field, Message: "ID 格式错误"},
		))
		return id.UUID{}, false
	}
	return parsed, true
}
