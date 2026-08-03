package catalog

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/access"
	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/response"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

func (handler Handler) ListCaseTags(ctx *gin.Context) {
	targetID, ok := bindID(ctx, "target_id")
	if !ok {
		return
	}
	status := StatusActive
	principal, _ := access.From(ctx)
	if principal.Admin() {
		status = ctx.Query("status")
	}
	items, err := handler.repository.ListCaseTags(
		ctx.Request.Context(),
		CaseTagScopeTarget,
		&targetID,
		status,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]TagPublic, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, map[string]any{"items": publicItems})
}

func (handler Handler) ListManagedCaseTags(ctx *gin.Context) {
	scope := ctx.Query("scope")
	if scope != CaseTagScopeGlobal && scope != CaseTagScopeTarget {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "scope", Message: "请选择全局或对象专属标签"},
		))
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
	if err := validateCaseTagScope(scope, targetID); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	status := StatusActive
	principal, _ := access.From(ctx)
	if principal.Admin() {
		status = ctx.Query("status")
	}
	items, err := handler.repository.ListCaseTags(ctx.Request.Context(), scope, targetID, status)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, map[string]any{"items": publicCaseTags(items)})
}

func (handler Handler) ListAvailableCaseTags(ctx *gin.Context) {
	targetID, ok := bindID(ctx, "target_id")
	if !ok {
		return
	}
	target, err := handler.repository.GetTarget(ctx.Request.Context(), targetID)
	if err != nil {
		response.ApplicationError(ctx, mapNotFound(err))
		return
	}
	principal, _ := access.From(ctx)
	if !principal.Admin() && target.Status != StatusActive {
		response.ApplicationError(ctx, apperror.NotFound())
		return
	}
	status := StatusActive
	if principal.Admin() {
		status = ctx.Query("status")
		if status == "" {
			status = StatusActive
		}
	}
	items, err := handler.repository.ListAvailableCaseTags(ctx.Request.Context(), targetID, status)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	global := make([]TagPublic, 0)
	targetItems := make([]TagPublic, 0)
	for _, item := range items {
		if item.Scope == CaseTagScopeGlobal {
			global = append(global, item.Public())
		} else {
			targetItems = append(targetItems, item.Public())
		}
	}
	response.JSON(ctx, http.StatusOK, map[string]any{
		"global": global,
		"target": targetItems,
	})
}

func (handler Handler) CreateCaseTag(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	targetID, ok := bindID(ctx, "target_id")
	if !ok {
		return
	}
	var request namedRequest
	if !bindJSON(ctx, &request) {
		return
	}
	item, err := handler.service.CreateCaseTag(
		ctx.Request.Context(),
		principal.ID(),
		CaseTagInput{
			Scope: CaseTagScopeTarget, EvaluationTargetID: &targetID,
			Name: request.Name, Description: request.Description,
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "case_tag.created", "case_tag", item.ID, nil, item.Public())
	response.JSON(ctx, http.StatusCreated, item.Public())
}

type caseTagRequest struct {
	Scope               string  `json:"scope"`
	EvaluationTargetID  *string `json:"evaluation_target_id"`
	Name                string  `json:"name"`
	Description         *string `json:"description"`
	ExpectedLockVersion uint32  `json:"expected_lock_version"`
}

func (handler Handler) CreateManagedCaseTag(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	input, ok := bindCaseTagInput(ctx)
	if !ok {
		return
	}
	item, err := handler.service.CreateCaseTag(ctx.Request.Context(), principal.ID(), input)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "case_tag.created", "case_tag", item.ID, nil, item.Public())
	response.JSON(ctx, http.StatusCreated, item.Public())
}

func bindCaseTagInput(ctx *gin.Context) (CaseTagInput, bool) {
	var request caseTagRequest
	if !bindJSON(ctx, &request) {
		return CaseTagInput{}, false
	}
	var targetID *id.UUID
	if request.EvaluationTargetID != nil && *request.EvaluationTargetID != "" {
		parsed, err := id.Parse(*request.EvaluationTargetID)
		if err != nil {
			response.ApplicationError(ctx, apperror.Validation(
				apperror.FieldError{Field: "evaluation_target_id", Message: "评测对象 ID 格式错误"},
			))
			return CaseTagInput{}, false
		}
		targetID = &parsed
	}
	return CaseTagInput{
		Scope: request.Scope, EvaluationTargetID: targetID,
		Name: request.Name, Description: request.Description,
		ExpectedLockVersion: request.ExpectedLockVersion,
	}, true
}

func (handler Handler) UpdateCaseTag(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	tagID, ok := bindID(ctx, "tag_id")
	if !ok {
		return
	}
	var request namedRequest
	if !bindJSON(ctx, &request) {
		return
	}
	before, after, err := handler.service.UpdateCaseTag(
		ctx.Request.Context(),
		principal.ID(),
		tagID,
		NamedInput{
			Name: request.Name, Description: request.Description,
			ExpectedLockVersion: request.ExpectedLockVersion,
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "case_tag.updated", "case_tag", tagID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) DisableCaseTag(ctx *gin.Context) {
	handler.setCaseTagStatus(ctx, StatusDisabled, "case_tag.disabled")
}

func (handler Handler) EnableCaseTag(ctx *gin.Context) {
	handler.setCaseTagStatus(ctx, StatusActive, "case_tag.enabled")
}

func (handler Handler) setCaseTagStatus(ctx *gin.Context, status, action string) {
	principal, _ := access.From(ctx)
	tagID, ok := bindID(ctx, "tag_id")
	if !ok {
		return
	}
	var request statusRequest
	if !bindJSON(ctx, &request) {
		return
	}
	before, after, err := handler.service.SetCaseTagStatus(
		ctx.Request.Context(),
		principal.ID(),
		tagID,
		status,
		request.ExpectedLockVersion,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), action, "case_tag", tagID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) ReorderCaseTags(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	targetID, ok := bindID(ctx, "target_id")
	if !ok {
		return
	}
	items, ok := bindReorder(ctx)
	if !ok {
		return
	}
	if err := handler.service.ReorderCaseTags(
		ctx.Request.Context(),
		principal.ID(),
		targetID,
		items,
	); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "case_tag.reordered", "evaluation_target", targetID, nil, map[string]any{
		"item_count": len(items),
	})
	ctx.Status(http.StatusNoContent)
}

func publicCaseTags(items []CaseTag) []TagPublic {
	publicItems := make([]TagPublic, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	return publicItems
}

func (handler Handler) ListIssueTags(ctx *gin.Context) {
	status := StatusActive
	principal, _ := access.From(ctx)
	if principal.Admin() {
		status = ctx.Query("status")
	}
	items, err := handler.repository.ListIssueTags(
		ctx.Request.Context(),
		status,
		ctx.Query("keyword"),
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]TagPublic, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, map[string]any{"items": publicItems})
}

func (handler Handler) CreateIssueTag(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	var request namedRequest
	if !bindJSON(ctx, &request) {
		return
	}
	item, err := handler.service.CreateIssueTag(
		ctx.Request.Context(),
		principal.ID(),
		NamedInput{Name: request.Name, Description: request.Description},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "issue_tag.created", "issue_tag", item.ID, nil, item.Public())
	response.JSON(ctx, http.StatusCreated, item.Public())
}

func (handler Handler) UpdateIssueTag(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	tagID, ok := bindID(ctx, "tag_id")
	if !ok {
		return
	}
	var request namedRequest
	if !bindJSON(ctx, &request) {
		return
	}
	before, after, err := handler.service.UpdateIssueTag(
		ctx.Request.Context(),
		principal.ID(),
		tagID,
		NamedInput{
			Name: request.Name, Description: request.Description,
			ExpectedLockVersion: request.ExpectedLockVersion,
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "issue_tag.updated", "issue_tag", tagID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) DisableIssueTag(ctx *gin.Context) {
	handler.setIssueTagStatus(ctx, StatusDisabled, "issue_tag.disabled")
}

func (handler Handler) EnableIssueTag(ctx *gin.Context) {
	handler.setIssueTagStatus(ctx, StatusActive, "issue_tag.enabled")
}

func (handler Handler) setIssueTagStatus(ctx *gin.Context, status, action string) {
	principal, _ := access.From(ctx)
	tagID, ok := bindID(ctx, "tag_id")
	if !ok {
		return
	}
	var request statusRequest
	if !bindJSON(ctx, &request) {
		return
	}
	before, after, err := handler.service.SetIssueTagStatus(
		ctx.Request.Context(),
		principal.ID(),
		tagID,
		status,
		request.ExpectedLockVersion,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), action, "issue_tag", tagID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) ReorderIssueTags(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	items, ok := bindReorder(ctx)
	if !ok {
		return
	}
	if err := handler.service.ReorderIssueTags(
		ctx.Request.Context(),
		principal.ID(),
		items,
	); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "issue_tag.reordered", "issue_tag", items[0].ID, nil, map[string]any{
		"item_count": len(items),
	})
	ctx.Status(http.StatusNoContent)
}

type reorderRequest struct {
	Items []struct {
		ID                  string `json:"id"`
		SortOrder           uint32 `json:"sort_order"`
		ExpectedLockVersion uint32 `json:"expected_lock_version"`
	} `json:"items"`
}

func bindReorder(ctx *gin.Context) ([]ReorderItem, bool) {
	var request reorderRequest
	if !bindJSON(ctx, &request) {
		return nil, false
	}
	items := make([]ReorderItem, 0, len(request.Items))
	for index, item := range request.Items {
		itemID, err := id.Parse(item.ID)
		if err != nil {
			response.ApplicationError(ctx, apperror.Validation(
				apperror.FieldError{
					Field:   "items",
					Message: "第 " + strconv.Itoa(index+1) + " 项 ID 格式错误",
				},
			))
			return nil, false
		}
		items = append(items, ReorderItem{
			ID:                  itemID,
			SortOrder:           item.SortOrder,
			ExpectedLockVersion: item.ExpectedLockVersion,
		})
	}
	return items, true
}
