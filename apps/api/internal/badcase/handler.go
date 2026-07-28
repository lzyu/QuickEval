package badcase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
	targetID, ok := optionalID(ctx, "evaluation_target_id")
	if !ok {
		return
	}
	datasetID, ok := optionalID(ctx, "dataset_id")
	if !ok {
		return
	}
	versionID, ok := optionalID(ctx, "dataset_version_id")
	if !ok {
		return
	}
	evaluatorID, ok := optionalID(ctx, "evaluator_id")
	if !ok {
		return
	}
	assigneeID, ok := optionalID(ctx, "assignee_id")
	if !ok {
		return
	}
	occurredFrom, ok := optionalTime(ctx, "occurred_from")
	if !ok {
		return
	}
	occurredTo, ok := optionalTime(ctx, "occurred_to")
	if !ok {
		return
	}
	items, total, err := handler.repository.List(
		ctx.Request.Context(), page.Page, page.PageSize,
		Filters{
			Status: ctx.Query("status"), SourceType: ctx.Query("source_type"),
			Open:     ctx.Query("open") == "1",
			Validity: ctx.Query("validity"), Environment: ctx.Query("environment"),
			AgentVersion: ctx.Query("agent_version"), TargetID: targetID,
			ScenarioID: scenarioID, DatasetID: datasetID, VersionID: versionID,
			EvaluatorID: evaluatorID, AssigneeID: assigneeID,
			IssueTagID: issueTagID, OccurredFrom: occurredFrom,
			OccurredTo: occurredTo, Keyword: ctx.Query("keyword"),
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
	response.JSON(ctx, http.StatusOK, item.Public())
}

func (handler Handler) Options(ctx *gin.Context) {
	users, err := handler.repository.ActiveUsers(ctx.Request.Context())
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	tags, err := handler.repository.ActiveIssueTagOptions(ctx.Request.Context())
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, map[string]any{
		"assignees": users, "issue_tags": tags,
	})
}

func (handler Handler) Page(ctx *gin.Context) {
	badcaseID, ok := pathID(ctx, "badcase_id")
	if !ok {
		return
	}
	principal, _ := access.From(ctx)
	item, err := handler.service.Page(
		ctx.Request.Context(), principal.ID(), principal.Admin(), badcaseID,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, item)
}

type businessRequest struct {
	ScenarioID          string   `json:"scenario_id"`
	Title               string   `json:"title"`
	Description         *string  `json:"description"`
	AgentResponseText   *string  `json:"agent_response_text"`
	AgentVersion        *string  `json:"agent_version"`
	Environment         string   `json:"environment"`
	OccurredAt          string   `json:"occurred_at"`
	BusinessReference   *string  `json:"business_reference"`
	SessionID           *string  `json:"session_id"`
	IssueTagIDs         []string `json:"issue_tag_ids"`
	ExpectedLockVersion uint32   `json:"expected_lock_version"`
}

func (handler Handler) CreateBusiness(ctx *gin.Context) {
	var request businessRequest
	if !bindJSON(ctx, &request) {
		return
	}
	scenarioID, ok := parseID(ctx, request.ScenarioID, "scenario_id")
	if !ok {
		return
	}
	occurredAt, ok := parseTime(ctx, request.OccurredAt, "occurred_at")
	if !ok {
		return
	}
	tagIDs, ok := parseIDs(ctx, request.IssueTagIDs, "issue_tag_ids")
	if !ok {
		return
	}
	principal, _ := access.From(ctx)
	key, digest, existingID, proceed := handler.reserveOperation(
		ctx, principal.ID(), "create-business", request,
	)
	if !proceed {
		if existingID != nil {
			item, err := handler.repository.Get(ctx.Request.Context(), *existingID)
			if err != nil {
				response.ApplicationError(ctx, mapNotFound(err))
				return
			}
			response.JSON(ctx, http.StatusOK, item.Public())
		}
		return
	}
	item, err := handler.service.CreateBusiness(
		ctx.Request.Context(), principal.ID(),
		BusinessInput{
			ScenarioID: scenarioID, Title: request.Title, Description: request.Description,
			AgentResponseText: request.AgentResponseText, AgentVersion: request.AgentVersion,
			Environment: request.Environment, OccurredAt: occurredAt,
			BusinessReference: request.BusinessReference, SessionID: request.SessionID,
			IssueTagIDs: tagIDs,
		},
	)
	if err != nil {
		handler.releaseOperation(ctx, principal.ID(), "create-business", key)
		response.ApplicationError(ctx, err)
		return
	}
	handler.commitOperation(
		ctx, principal.ID(), "create-business", key, digest, item.ID,
	)
	handler.recordAction(ctx, principal.ID(), "badcase.created", item.ID, nil, item.Public())
	response.JSON(ctx, http.StatusCreated, item.Public())
}

func (handler Handler) UpdateBusiness(ctx *gin.Context) {
	badcaseID, ok := pathID(ctx, "badcase_id")
	if !ok {
		return
	}
	var request businessRequest
	if !bindJSON(ctx, &request) {
		return
	}
	occurredAt, ok := parseTime(ctx, request.OccurredAt, "occurred_at")
	if !ok {
		return
	}
	principal, _ := access.From(ctx)
	before, err := handler.repository.Get(ctx.Request.Context(), badcaseID)
	if err != nil {
		response.ApplicationError(ctx, mapNotFound(err))
		return
	}
	item, err := handler.service.UpdateBusiness(
		ctx.Request.Context(), principal.ID(), principal.Admin(), badcaseID,
		BusinessInput{
			Title: request.Title, Description: request.Description,
			AgentResponseText: request.AgentResponseText, AgentVersion: request.AgentVersion,
			Environment: request.Environment, OccurredAt: occurredAt,
			BusinessReference: request.BusinessReference, SessionID: request.SessionID,
			ExpectedLockVersion: request.ExpectedLockVersion,
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.recordAction(
		ctx, principal.ID(), "badcase.updated", item.ID, before.Public(), item.Public(),
	)
	response.JSON(ctx, http.StatusOK, item.Public())
}

type tagsRequest struct {
	IssueTagIDs         []string `json:"issue_tag_ids"`
	ExpectedLockVersion uint32   `json:"expected_lock_version"`
}

func (handler Handler) UpdateIssueTags(ctx *gin.Context) {
	badcaseID, ok := pathID(ctx, "badcase_id")
	if !ok {
		return
	}
	var request tagsRequest
	if !bindJSON(ctx, &request) {
		return
	}
	tagIDs, ok := parseIDs(ctx, request.IssueTagIDs, "issue_tag_ids")
	if !ok {
		return
	}
	principal, _ := access.From(ctx)
	item, err := handler.service.UpdateIssueTags(
		ctx.Request.Context(), principal.ID(), badcaseID,
		request.ExpectedLockVersion, tagIDs,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.recordAction(ctx, principal.ID(), "badcase.tags_updated", item.ID, nil, item.Public())
	response.JSON(ctx, http.StatusOK, item.Public())
}

type workflowRequest struct {
	ExpectedLockVersion uint32  `json:"expected_lock_version"`
	AssigneeID          *string `json:"assignee_id"`
	Reason              string  `json:"reason"`
	Note                string  `json:"note"`
}

func (handler Handler) Assign(ctx *gin.Context) {
	badcaseID, request, principal, ok := handler.workflowContext(ctx)
	if !ok {
		return
	}
	if request.AssigneeID == nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "assignee_id", Message: "请选择负责人"},
		))
		return
	}
	assigneeID, parsed := parseID(ctx, *request.AssigneeID, "assignee_id")
	if !parsed {
		return
	}
	item, err := handler.service.Assign(
		ctx.Request.Context(), principal.ID(), badcaseID,
		request.ExpectedLockVersion, &assigneeID,
	)
	handler.workflowResponse(ctx, principal.ID(), "badcase.assigned", item, err)
}

func (handler Handler) Unassign(ctx *gin.Context) {
	badcaseID, request, principal, ok := handler.workflowContext(ctx)
	if !ok {
		return
	}
	item, err := handler.service.Assign(
		ctx.Request.Context(), principal.ID(), badcaseID,
		request.ExpectedLockVersion, nil,
	)
	handler.workflowResponse(ctx, principal.ID(), "badcase.unassigned", item, err)
}

func (handler Handler) StartProcessing(ctx *gin.Context) {
	handler.transition(ctx, StatusProcessing, "badcase.processing_started")
}

func (handler Handler) Resolve(ctx *gin.Context) {
	handler.transition(ctx, StatusResolved, "badcase.resolved")
}

func (handler Handler) Defer(ctx *gin.Context) {
	handler.transition(ctx, StatusDeferred, "badcase.deferred")
}

func (handler Handler) Reopen(ctx *gin.Context) {
	handler.transition(ctx, StatusPending, "badcase.reopened")
}

func (handler Handler) transition(ctx *gin.Context, status, action string) {
	badcaseID, request, principal, ok := handler.workflowContext(ctx)
	if !ok {
		return
	}
	item, err := handler.service.Transition(
		ctx.Request.Context(), principal.ID(), badcaseID,
		request.ExpectedLockVersion, status, request.Reason,
	)
	handler.workflowResponse(ctx, principal.ID(), action, item, err)
}

func (handler Handler) AddNote(ctx *gin.Context) {
	badcaseID, request, principal, ok := handler.workflowContext(ctx)
	if !ok {
		return
	}
	scope := "add-note:" + badcaseID.String()
	key, digest, existingID, proceed := handler.reserveOperation(
		ctx, principal.ID(), scope, request,
	)
	if !proceed {
		if existingID != nil {
			item, err := handler.repository.Get(ctx.Request.Context(), *existingID)
			if err != nil {
				response.ApplicationError(ctx, mapNotFound(err))
				return
			}
			response.JSON(ctx, http.StatusOK, item.Public())
		}
		return
	}
	item, err := handler.service.AddNote(
		ctx.Request.Context(), principal.ID(), badcaseID,
		request.ExpectedLockVersion, request.Note,
	)
	if err != nil {
		handler.releaseOperation(ctx, principal.ID(), scope, key)
		response.ApplicationError(ctx, err)
		return
	}
	handler.commitOperation(ctx, principal.ID(), scope, key, digest, item.ID)
	handler.workflowResponse(ctx, principal.ID(), "badcase.note_added", item, nil)
}

func (handler Handler) Invalidate(ctx *gin.Context) {
	handler.setValidity(ctx, false, "badcase.invalidated")
}

func (handler Handler) Reactivate(ctx *gin.Context) {
	handler.setValidity(ctx, true, "badcase.reactivated")
}

func (handler Handler) setValidity(ctx *gin.Context, reactivate bool, action string) {
	badcaseID, request, principal, ok := handler.workflowContext(ctx)
	if !ok {
		return
	}
	item, err := handler.service.SetValidity(
		ctx.Request.Context(), principal.ID(), principal.Admin(), badcaseID,
		request.ExpectedLockVersion, reactivate, request.Reason,
	)
	handler.workflowResponse(ctx, principal.ID(), action, item, err)
}

func (handler Handler) workflowContext(
	ctx *gin.Context,
) (id.UUID, workflowRequest, access.Principal, bool) {
	badcaseID, ok := pathID(ctx, "badcase_id")
	if !ok {
		return id.UUID{}, workflowRequest{}, access.Principal{}, false
	}
	var request workflowRequest
	if !bindJSON(ctx, &request) {
		return id.UUID{}, workflowRequest{}, access.Principal{}, false
	}
	principal, _ := access.From(ctx)
	return badcaseID, request, principal, true
}

func (handler Handler) workflowResponse(
	ctx *gin.Context,
	actorID id.UUID,
	action string,
	item Badcase,
	err error,
) {
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.recordAction(ctx, actorID, action, item.ID, nil, item.Public())
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
	handler.recordAction(
		ctx, principal.ID(), "badcase.created", result.Badcase.ID, nil, result.Badcase.Public(),
	)
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

func (handler Handler) recordAction(
	ctx *gin.Context,
	actorID id.UUID,
	action string,
	badcaseID id.UUID,
	before, after any,
) {
	if err := handler.audit.Record(
		ctx.Request.Context(), &actorID, action, "badcase", badcaseID,
		before, after, requestid.From(ctx), ctx.ClientIP(), ctx.Request.UserAgent(),
	); err != nil {
		handler.logger.Error("record audit log", "action", action, "error", err)
	}
}

func optionalTime(ctx *gin.Context, key string) (*time.Time, bool) {
	if ctx.Query(key) == "" {
		return nil, true
	}
	value, ok := parseTime(ctx, ctx.Query(key), key)
	if !ok {
		return nil, false
	}
	return &value, true
}

func parseTime(ctx *gin.Context, value, field string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: field, Message: "时间必须为 RFC3339 格式"},
		))
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func parseIDs(ctx *gin.Context, values []string, field string) ([]id.UUID, bool) {
	items := make([]id.UUID, 0, len(values))
	for _, value := range values {
		parsed, ok := parseID(ctx, value, field)
		if !ok {
			return nil, false
		}
		items = append(items, parsed)
	}
	return items, true
}

func bindJSON(ctx *gin.Context, target any) bool {
	if err := ctx.ShouldBindJSON(target); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return false
	}
	return true
}

func (handler Handler) reserveOperation(
	ctx *gin.Context,
	actorID id.UUID,
	scope string,
	request any,
) (string, string, *id.UUID, bool) {
	key := strings.TrimSpace(ctx.GetHeader("Idempotency-Key"))
	if key == "" || len(key) > 200 {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{
				Field: "Idempotency-Key", Message: "请提供不超过 200 个字符的幂等键",
			},
		))
		return "", "", nil, false
	}
	value, err := json.Marshal(request)
	if err != nil {
		response.ApplicationError(ctx, err)
		return "", "", nil, false
	}
	sum := sha256.Sum256(value)
	digest := hex.EncodeToString(sum[:])
	existingID, err := handler.idempotency.ReserveOperation(
		ctx.Request.Context(), actorID, scope, key, digest,
	)
	if errors.Is(err, ErrRequestInProgress) {
		response.ApplicationError(ctx, apperror.Conflict(
			"REQUEST_IN_PROGRESS", "相同请求正在处理中，请稍后重试",
		))
		return "", "", nil, false
	}
	if errors.Is(err, ErrIdempotencyKeyReused) {
		response.ApplicationError(ctx, apperror.Conflict(
			"IDEMPOTENCY_KEY_REUSED", "幂等键已用于不同请求",
		))
		return "", "", nil, false
	}
	if err != nil {
		response.ApplicationError(ctx, err)
		return "", "", nil, false
	}
	return key, digest, existingID, existingID == nil
}

func (handler Handler) releaseOperation(
	ctx *gin.Context,
	actorID id.UUID,
	scope, key string,
) {
	if key != "" {
		handler.idempotency.ReleaseOperation(ctx.Request.Context(), actorID, scope, key)
	}
}

func (handler Handler) commitOperation(
	ctx *gin.Context,
	actorID id.UUID,
	scope, key, digest string,
	itemID id.UUID,
) {
	if err := handler.idempotency.CommitOperation(
		ctx.Request.Context(), actorID, scope, key, digest, itemID,
	); err != nil {
		handler.logger.Error("commit badcase operation idempotency", "scope", scope, "error", err)
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
