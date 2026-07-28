package evaluation

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

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

func (handler Handler) ListRuns(ctx *gin.Context) {
	page, ok := evaluationPage(ctx)
	if !ok {
		return
	}
	principal, _ := access.From(ctx)
	evaluatorID := principal.ID()
	if principal.Admin() && ctx.Query("evaluator_id") != "" {
		parsed, ok := evaluationID(ctx, ctx.Query("evaluator_id"), "evaluator_id")
		if !ok {
			return
		}
		evaluatorID = parsed
	}
	datasetID, ok := optionalEvaluationID(ctx, "dataset_id")
	if !ok {
		return
	}
	scenarioID, ok := optionalEvaluationID(ctx, "scenario_id")
	if !ok {
		return
	}
	items, total, err := handler.repository.ListRuns(
		ctx.Request.Context(), page.Page, page.PageSize,
		RunFilters{
			EvaluatorID: &evaluatorID, Status: ctx.Query("status"),
			Environment: ctx.Query("environment"), DatasetID: datasetID,
			ScenarioID: scenarioID, Keyword: ctx.Query("keyword"),
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]RunPublic, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, transport.PageData[RunPublic]{
		Items: publicItems, Page: page.Page, PageSize: page.PageSize, Total: total,
	})
}

func (handler Handler) GetRun(ctx *gin.Context) {
	run, ok := handler.authorizedRun(ctx)
	if !ok {
		return
	}
	response.JSON(ctx, http.StatusOK, run.Public())
}

type runRequest struct {
	DatasetVersionID    string  `json:"dataset_version_id"`
	AgentVersion        string  `json:"agent_version"`
	Environment         string  `json:"environment"`
	PurposeNote         *string `json:"purpose_note"`
	ConfigNote          *string `json:"config_note"`
	ExpectedLockVersion uint32  `json:"expected_lock_version"`
}

func (handler Handler) CreateRun(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	idempotencyKey := ctx.GetHeader("Idempotency-Key")
	if len(idempotencyKey) > 200 {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "Idempotency-Key", Message: "幂等键最多 200 个字符"},
		))
		return
	}
	if idempotencyKey != "" {
		existingID, err := handler.idempotency.Reserve(
			ctx.Request.Context(), principal.ID(), idempotencyKey,
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
			item, err := handler.service.AuthorizedRun(
				ctx.Request.Context(), principal.ID(), principal.Admin(), *existingID,
			)
			if err != nil {
				response.ApplicationError(ctx, err)
				return
			}
			response.JSON(ctx, http.StatusOK, item.Public())
			return
		}
		defer func() {
			if !ctx.IsAborted() {
				return
			}
			handler.idempotency.Release(ctx.Request.Context(), principal.ID(), idempotencyKey)
		}()
	}
	var request runRequest
	if !bindEvaluationJSON(ctx, &request) {
		return
	}
	versionID, ok := evaluationID(ctx, request.DatasetVersionID, "dataset_version_id")
	if !ok {
		return
	}
	item, err := handler.service.CreateRun(ctx.Request.Context(), principal.ID(), RunInput{
		DatasetVersionID: versionID, AgentVersion: request.AgentVersion,
		Environment: request.Environment, PurposeNote: request.PurposeNote,
		ConfigNote: request.ConfigNote,
	})
	if err != nil {
		if idempotencyKey != "" {
			handler.idempotency.Release(ctx.Request.Context(), principal.ID(), idempotencyKey)
		}
		response.ApplicationError(ctx, err)
		return
	}
	if idempotencyKey != "" {
		if err := handler.idempotency.Commit(
			ctx.Request.Context(), principal.ID(), idempotencyKey, item.ID,
		); err != nil {
			handler.logger.Error("commit evaluation idempotency key", "error", err)
		}
	}
	handler.record(ctx, principal.ID(), "evaluation_run.created", item.ID, nil, item.Public())
	response.JSON(ctx, http.StatusCreated, item.Public())
}

func (handler Handler) UpdateRun(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	runID, ok := evaluationPathID(ctx, "run_id")
	if !ok {
		return
	}
	var request runRequest
	if !bindEvaluationJSON(ctx, &request) {
		return
	}
	before, after, err := handler.service.UpdateRun(
		ctx.Request.Context(), principal.ID(), principal.Admin(), runID,
		RunInput{
			AgentVersion: request.AgentVersion, Environment: request.Environment,
			PurposeNote: request.PurposeNote, ConfigNote: request.ConfigNote,
			ExpectedLockVersion: request.ExpectedLockVersion,
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "evaluation_run.updated", runID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) DeleteRun(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	runID, ok := evaluationPathID(ctx, "run_id")
	if !ok {
		return
	}
	expected, err := parseExpectedVersion(ctx)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	if err := handler.service.Delete(
		ctx.Request.Context(), principal.ID(), principal.Admin(), runID, expected,
	); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "evaluation_run.deleted", runID, nil, nil)
	ctx.Status(http.StatusNoContent)
}

type commandRequest struct {
	Reason              string `json:"reason"`
	ExpectedLockVersion uint32 `json:"expected_lock_version"`
}

func (handler Handler) CompleteRun(ctx *gin.Context) {
	handler.lifecycle(ctx, "complete")
}

func (handler Handler) ReopenRun(ctx *gin.Context) {
	handler.lifecycle(ctx, "reopen")
}

func (handler Handler) VoidRun(ctx *gin.Context) {
	handler.lifecycle(ctx, "void")
}

func (handler Handler) lifecycle(ctx *gin.Context, command string) {
	principal, _ := access.From(ctx)
	runID, ok := evaluationPathID(ctx, "run_id")
	if !ok {
		return
	}
	var request commandRequest
	if !bindEvaluationJSON(ctx, &request) {
		return
	}
	var before, after Run
	var err error
	switch command {
	case "complete":
		before, after, err = handler.service.Complete(
			ctx.Request.Context(), principal.ID(), principal.Admin(), runID,
			request.ExpectedLockVersion,
		)
	case "reopen":
		before, after, err = handler.service.Reopen(
			ctx.Request.Context(), principal.ID(), principal.Admin(), runID,
			request.Reason, request.ExpectedLockVersion,
		)
	case "void":
		before, after, err = handler.service.Void(
			ctx.Request.Context(), principal.ID(), principal.Admin(), runID,
			request.Reason, request.ExpectedLockVersion,
		)
	}
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	afterData := map[string]any{"run": after.Public()}
	if request.Reason != "" {
		afterData["reason"] = request.Reason
	}
	action := map[string]string{
		"complete": "evaluation_run.completed",
		"reopen":   "evaluation_run.reopened",
		"void":     "evaluation_run.voided",
	}[command]
	handler.record(ctx, principal.ID(), action, runID, before.Public(), afterData)
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) ListResults(ctx *gin.Context) {
	run, ok := handler.authorizedRun(ctx)
	if !ok {
		return
	}
	page, ok := evaluationPage(ctx)
	if !ok {
		return
	}
	items, total, err := handler.repository.ListResults(
		ctx.Request.Context(), run.ID, page.Page, page.PageSize, ctx.Query("status"),
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]ResultPublic, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, transport.PageData[ResultPublic]{
		Items: publicItems, Page: page.Page, PageSize: page.PageSize, Total: total,
	})
}

func (handler Handler) Workbench(ctx *gin.Context) {
	run, ok := handler.authorizedRun(ctx)
	if !ok {
		return
	}
	page, ok := evaluationPage(ctx)
	if !ok {
		return
	}
	items, total, err := handler.repository.ListResults(
		ctx.Request.Context(), run.ID, page.Page, page.PageSize, ctx.Query("status"),
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]ResultPublic, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, map[string]any{
		"run": run.Public(),
		"results": transport.PageData[ResultPublic]{
			Items: publicItems, Page: page.Page, PageSize: page.PageSize, Total: total,
		},
	})
}

func (handler Handler) GetResult(ctx *gin.Context) {
	resultID, ok := evaluationPathID(ctx, "result_id")
	if !ok {
		return
	}
	item, err := handler.repository.GetResult(ctx.Request.Context(), resultID)
	if err != nil {
		response.ApplicationError(ctx, mapNotFound(err))
		return
	}
	principal, _ := access.From(ctx)
	if _, err := handler.service.AuthorizedRun(
		ctx.Request.Context(), principal.ID(), principal.Admin(), item.EvaluationRunID,
	); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, item.Public())
}

type resultRequest struct {
	Status              string  `json:"status"`
	AnswerText          *string `json:"answer_text"`
	Score               *uint8  `json:"score"`
	Comment             *string `json:"comment"`
	SkipReason          *string `json:"skip_reason"`
	ExpectedLockVersion uint32  `json:"expected_lock_version"`
}

func (handler Handler) UpdateResult(ctx *gin.Context) {
	resultID, ok := evaluationPathID(ctx, "result_id")
	if !ok {
		return
	}
	var request resultRequest
	if !bindEvaluationJSON(ctx, &request) {
		return
	}
	principal, _ := access.From(ctx)
	item, run, err := handler.service.UpdateResult(
		ctx.Request.Context(), principal.ID(), principal.Admin(), resultID,
		ResultInput{
			Status: request.Status, AnswerText: request.AnswerText,
			Score: request.Score, Comment: request.Comment, SkipReason: request.SkipReason,
			ExpectedLockVersion: request.ExpectedLockVersion,
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, map[string]any{
		"result": item.Public(), "progress": run.Public().Progress,
		"run_lock_version": run.LockVersion,
	})
}

func (handler Handler) authorizedRun(ctx *gin.Context) (Run, bool) {
	runID, ok := evaluationPathID(ctx, "run_id")
	if !ok {
		return Run{}, false
	}
	principal, _ := access.From(ctx)
	run, err := handler.service.AuthorizedRun(
		ctx.Request.Context(), principal.ID(), principal.Admin(), runID,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return Run{}, false
	}
	return run, true
}

func evaluationPage(ctx *gin.Context) (transport.Page, bool) {
	var page transport.Page
	if err := ctx.ShouldBindQuery(&page); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return transport.Page{}, false
	}
	page = page.Normalize()
	if !page.Valid() {
		response.ApplicationError(ctx, apperror.Validation())
		return transport.Page{}, false
	}
	return page, true
}

func optionalEvaluationID(ctx *gin.Context, key string) (*id.UUID, bool) {
	if ctx.Query(key) == "" {
		return nil, true
	}
	value, ok := evaluationID(ctx, ctx.Query(key), key)
	if !ok {
		return nil, false
	}
	return &value, true
}

func evaluationPathID(ctx *gin.Context, key string) (id.UUID, bool) {
	return evaluationID(ctx, ctx.Param(key), key)
}

func evaluationID(ctx *gin.Context, value, field string) (id.UUID, bool) {
	parsed, err := id.Parse(value)
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: field, Message: "ID 格式错误"},
		))
		return id.UUID{}, false
	}
	return parsed, true
}

func parseExpectedVersion(ctx *gin.Context) (uint32, error) {
	value, err := strconv.ParseUint(ctx.Query("expected_lock_version"), 10, 32)
	if err != nil {
		return 0, apperror.Validation(
			apperror.FieldError{Field: "expected_lock_version", Message: "缺少有效锁版本"},
		)
	}
	return uint32(value), nil
}

func bindEvaluationJSON(ctx *gin.Context, target any) bool {
	if err := ctx.ShouldBindJSON(target); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return false
	}
	return true
}

func (handler Handler) record(
	ctx *gin.Context,
	actorID id.UUID,
	action string,
	entityID id.UUID,
	before, after any,
) {
	if err := handler.audit.Record(
		ctx.Request.Context(), &actorID, action, "evaluation_run", entityID,
		before, after, requestid.From(ctx), ctx.ClientIP(), ctx.Request.UserAgent(),
	); err != nil {
		handler.logger.Error("record audit log", "action", action, "error", err)
	}
}
