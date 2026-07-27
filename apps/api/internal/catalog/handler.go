package catalog

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

type namedRequest struct {
	Name                string  `json:"name"`
	Description         *string `json:"description"`
	ExpectedLockVersion uint32  `json:"expected_lock_version"`
}

type statusRequest struct {
	ExpectedLockVersion uint32 `json:"expected_lock_version"`
}

func (handler Handler) ListTargets(ctx *gin.Context) {
	page, ok := bindPage(ctx)
	if !ok {
		return
	}
	status := visibleStatus(ctx)
	if authPrincipal, _ := access.From(ctx); authPrincipal.Admin() {
		status = ctx.Query("status")
	}
	items, total, err := handler.repository.ListTargets(
		ctx.Request.Context(),
		page.Page,
		page.PageSize,
		status,
		ctx.Query("keyword"),
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]TargetPublic, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, transport.PageData[TargetPublic]{
		Items: publicItems, Page: page.Page, PageSize: page.PageSize, Total: total,
	})
}

func (handler Handler) GetTarget(ctx *gin.Context) {
	targetID, ok := bindID(ctx, "target_id")
	if !ok {
		return
	}
	item, err := handler.repository.GetTarget(ctx.Request.Context(), targetID)
	if err != nil {
		response.ApplicationError(ctx, mapNotFound(err))
		return
	}
	principal, _ := access.From(ctx)
	if !principal.Admin() && item.Status != StatusActive {
		response.ApplicationError(ctx, apperror.NotFound())
		return
	}
	response.JSON(ctx, http.StatusOK, item.Public())
}

func (handler Handler) CreateTarget(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	var request namedRequest
	if !bindJSON(ctx, &request) {
		return
	}
	item, err := handler.service.CreateTarget(ctx.Request.Context(), principal.ID(), NamedInput{
		Name: request.Name, Description: request.Description,
	})
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "evaluation_target.created", "evaluation_target", item.ID, nil, item.Public())
	response.JSON(ctx, http.StatusCreated, item.Public())
}

func (handler Handler) UpdateTarget(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	targetID, ok := bindID(ctx, "target_id")
	if !ok {
		return
	}
	var request namedRequest
	if !bindJSON(ctx, &request) {
		return
	}
	before, after, err := handler.service.UpdateTarget(
		ctx.Request.Context(),
		principal.ID(),
		targetID,
		NamedInput{
			Name: request.Name, Description: request.Description,
			ExpectedLockVersion: request.ExpectedLockVersion,
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "evaluation_target.updated", "evaluation_target", targetID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) DisableTarget(ctx *gin.Context) {
	handler.setTargetStatus(ctx, StatusDisabled, "evaluation_target.disabled")
}

func (handler Handler) EnableTarget(ctx *gin.Context) {
	handler.setTargetStatus(ctx, StatusActive, "evaluation_target.enabled")
}

func (handler Handler) setTargetStatus(ctx *gin.Context, status, action string) {
	principal, _ := access.From(ctx)
	targetID, ok := bindID(ctx, "target_id")
	if !ok {
		return
	}
	var request statusRequest
	if !bindJSON(ctx, &request) {
		return
	}
	before, after, err := handler.service.SetTargetStatus(
		ctx.Request.Context(),
		principal.ID(),
		targetID,
		status,
		request.ExpectedLockVersion,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), action, "evaluation_target", targetID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

type scenarioRequest struct {
	EvaluationTargetID  string  `json:"evaluation_target_id"`
	Name                string  `json:"name"`
	Description         *string `json:"description"`
	ExpectedLockVersion uint32  `json:"expected_lock_version"`
}

func (handler Handler) ListScenarios(ctx *gin.Context) {
	page, ok := bindPage(ctx)
	if !ok {
		return
	}
	var targetID *id.UUID
	if value := ctx.Query("evaluation_target_id"); value != "" {
		parsed, err := id.Parse(value)
		if err != nil {
			response.ApplicationError(ctx, apperror.Validation(
				apperror.FieldError{Field: "evaluation_target_id", Message: "评测对象 ID 格式错误"},
			))
			return
		}
		targetID = &parsed
	}
	status := visibleStatus(ctx)
	principal, _ := access.From(ctx)
	if principal.Admin() {
		status = ctx.Query("status")
	}
	items, total, err := handler.repository.ListScenarios(
		ctx.Request.Context(),
		page.Page,
		page.PageSize,
		targetID,
		status,
		ctx.Query("keyword"),
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]ScenarioPublic, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, transport.PageData[ScenarioPublic]{
		Items: publicItems, Page: page.Page, PageSize: page.PageSize, Total: total,
	})
}

func (handler Handler) GetScenario(ctx *gin.Context) {
	scenarioID, ok := bindID(ctx, "scenario_id")
	if !ok {
		return
	}
	item, err := handler.repository.GetScenario(ctx.Request.Context(), scenarioID)
	if err != nil {
		response.ApplicationError(ctx, mapNotFound(err))
		return
	}
	principal, _ := access.From(ctx)
	if !principal.Admin() && item.Status != StatusActive {
		response.ApplicationError(ctx, apperror.NotFound())
		return
	}
	response.JSON(ctx, http.StatusOK, item.Public())
}

func (handler Handler) CreateScenario(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	input, ok := bindScenarioInput(ctx)
	if !ok {
		return
	}
	item, err := handler.service.CreateScenario(ctx.Request.Context(), principal.ID(), input)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "scenario.created", "scenario", item.ID, nil, item.Public())
	response.JSON(ctx, http.StatusCreated, item.Public())
}

func (handler Handler) UpdateScenario(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	scenarioID, ok := bindID(ctx, "scenario_id")
	if !ok {
		return
	}
	input, ok := bindScenarioInput(ctx)
	if !ok {
		return
	}
	before, after, err := handler.service.UpdateScenario(
		ctx.Request.Context(),
		principal.ID(),
		scenarioID,
		input,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "scenario.updated", "scenario", scenarioID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) DisableScenario(ctx *gin.Context) {
	handler.setScenarioStatus(ctx, StatusDisabled, "scenario.disabled")
}

func (handler Handler) EnableScenario(ctx *gin.Context) {
	handler.setScenarioStatus(ctx, StatusActive, "scenario.enabled")
}

func (handler Handler) setScenarioStatus(ctx *gin.Context, status, action string) {
	principal, _ := access.From(ctx)
	scenarioID, ok := bindID(ctx, "scenario_id")
	if !ok {
		return
	}
	var request statusRequest
	if !bindJSON(ctx, &request) {
		return
	}
	before, after, err := handler.service.SetScenarioStatus(
		ctx.Request.Context(),
		principal.ID(),
		scenarioID,
		status,
		request.ExpectedLockVersion,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), action, "scenario", scenarioID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

func bindScenarioInput(ctx *gin.Context) (ScenarioInput, bool) {
	var request scenarioRequest
	if !bindJSON(ctx, &request) {
		return ScenarioInput{}, false
	}
	targetID, err := id.Parse(request.EvaluationTargetID)
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "evaluation_target_id", Message: "请选择评测对象"},
		))
		return ScenarioInput{}, false
	}
	return ScenarioInput{
		EvaluationTargetID:  targetID,
		Name:                request.Name,
		Description:         request.Description,
		ExpectedLockVersion: request.ExpectedLockVersion,
	}, true
}

func bindPage(ctx *gin.Context) (transport.Page, bool) {
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

func bindID(ctx *gin.Context, name string) (id.UUID, bool) {
	value, err := id.Parse(ctx.Param(name))
	if err != nil {
		response.ApplicationError(ctx, apperror.NotFound())
		return id.UUID{}, false
	}
	return value, true
}

func bindJSON(ctx *gin.Context, target any) bool {
	if err := ctx.ShouldBindJSON(target); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return false
	}
	return true
}

func visibleStatus(_ *gin.Context) string {
	return StatusActive
}

func (handler Handler) record(
	ctx *gin.Context,
	actorID id.UUID,
	action, entityType string,
	entityID id.UUID,
	before, after any,
) {
	if err := handler.audit.Record(
		ctx.Request.Context(),
		&actorID,
		action,
		entityType,
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
