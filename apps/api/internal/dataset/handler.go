package dataset

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
	imports    ImportPreviewStore
	audit      audit.Recorder
	logger     *slog.Logger
}

func NewHandler(
	service Service,
	repository Repository,
	imports ImportPreviewStore,
	auditRecorder audit.Recorder,
	logger *slog.Logger,
) Handler {
	return Handler{
		service: service, repository: repository, imports: imports,
		audit: auditRecorder, logger: logger,
	}
}

func (handler Handler) ListDatasets(ctx *gin.Context) {
	page, ok := datasetPage(ctx)
	if !ok {
		return
	}
	targetID, ok := optionalQueryID(ctx, "evaluation_target_id")
	if !ok {
		return
	}
	scenarioID, ok := optionalQueryID(ctx, "scenario_id")
	if !ok {
		return
	}
	principal, _ := access.From(ctx)
	status := DatasetActive
	if principal.Admin() {
		status = ctx.Query("status")
	}
	items, total, err := handler.repository.ListDatasets(
		ctx.Request.Context(), page.Page, page.PageSize,
		targetID, scenarioID, status, ctx.Query("keyword"),
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]DatasetPublic, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, transport.PageData[DatasetPublic]{
		Items: publicItems, Page: page.Page, PageSize: page.PageSize, Total: total,
	})
}

func (handler Handler) GetDataset(ctx *gin.Context) {
	item, ok := handler.visibleDataset(ctx)
	if !ok {
		return
	}
	versions, err := handler.repository.ListVersions(ctx.Request.Context(), item.ID)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	principal, _ := access.From(ctx)
	publicVersions := []VersionPublic{}
	for _, version := range versions {
		if !principal.Admin() && version.Status == VersionDraft {
			continue
		}
		publicVersions = append(publicVersions, version.Public())
	}
	response.JSON(ctx, http.StatusOK, map[string]any{
		"dataset": item.Public(), "versions": publicVersions,
	})
}

type datasetRequest struct {
	TargetID            string  `json:"evaluation_target_id"`
	Name                string  `json:"name"`
	Description         *string `json:"description"`
	ExpectedLockVersion uint32  `json:"expected_lock_version"`
}

func (handler Handler) CreateDataset(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	var request datasetRequest
	if !bindDatasetJSON(ctx, &request) {
		return
	}
	targetID, ok := requestID(ctx, request.TargetID, "evaluation_target_id")
	if !ok {
		return
	}
	result, err := handler.service.CreateDataset(ctx.Request.Context(), principal.ID(), DatasetInput{
		TargetID: targetID, Name: request.Name, Description: request.Description,
	})
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "dataset.created", "dataset", result.Dataset.ID, nil, result.Dataset.Public())
	response.JSON(ctx, http.StatusCreated, map[string]any{
		"dataset": result.Dataset.Public(), "draft": result.Draft.Public(),
	})
}

func (handler Handler) UpdateDataset(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	datasetID, ok := pathID(ctx, "dataset_id")
	if !ok {
		return
	}
	var request datasetRequest
	if !bindDatasetJSON(ctx, &request) {
		return
	}
	targetID, ok := requestID(ctx, request.TargetID, "evaluation_target_id")
	if !ok {
		return
	}
	before, after, err := handler.service.UpdateDataset(
		ctx.Request.Context(), principal.ID(), datasetID,
		DatasetInput{
			TargetID: targetID, Name: request.Name, Description: request.Description,
			ExpectedLockVersion: request.ExpectedLockVersion,
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "dataset.updated", "dataset", datasetID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

type lockRequest struct {
	ExpectedLockVersion uint32 `json:"expected_lock_version"`
}

func (handler Handler) ArchiveDataset(ctx *gin.Context) {
	handler.setDatasetStatus(ctx, DatasetArchived, "dataset.archived")
}

func (handler Handler) RestoreDataset(ctx *gin.Context) {
	handler.setDatasetStatus(ctx, DatasetActive, "dataset.restored")
}

func (handler Handler) setDatasetStatus(ctx *gin.Context, status, action string) {
	principal, _ := access.From(ctx)
	datasetID, ok := pathID(ctx, "dataset_id")
	if !ok {
		return
	}
	var request lockRequest
	if !bindDatasetJSON(ctx, &request) {
		return
	}
	before, after, err := handler.service.SetDatasetStatus(
		ctx.Request.Context(), principal.ID(), datasetID, status, request.ExpectedLockVersion,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), action, "dataset", datasetID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) ListVersions(ctx *gin.Context) {
	dataset, ok := handler.visibleDataset(ctx)
	if !ok {
		return
	}
	items, err := handler.repository.ListVersions(ctx.Request.Context(), dataset.ID)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	principal, _ := access.From(ctx)
	publicItems := []VersionPublic{}
	for _, item := range items {
		if !principal.Admin() && item.Status == VersionDraft {
			continue
		}
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, map[string]any{"items": publicItems})
}

func (handler Handler) GetVersion(ctx *gin.Context) {
	version, _, ok := handler.visibleVersion(ctx)
	if !ok {
		return
	}
	response.JSON(ctx, http.StatusOK, version.Public())
}

type createDraftRequest struct {
	BaseVersionID              *string `json:"base_version_id"`
	ExpectedDatasetLockVersion uint32  `json:"expected_dataset_lock_version"`
}

func (handler Handler) CreateDraft(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	datasetID, ok := pathID(ctx, "dataset_id")
	if !ok {
		return
	}
	var request createDraftRequest
	if !bindDatasetJSON(ctx, &request) {
		return
	}
	var baseID *id.UUID
	if request.BaseVersionID != nil && *request.BaseVersionID != "" {
		parsed, ok := requestID(ctx, *request.BaseVersionID, "base_version_id")
		if !ok {
			return
		}
		baseID = &parsed
	}
	item, err := handler.service.CreateDraft(
		ctx.Request.Context(), principal.ID(), datasetID,
		CreateDraftInput{
			BaseVersionID:              baseID,
			ExpectedDatasetLockVersion: request.ExpectedDatasetLockVersion,
		},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "dataset_version.draft_created", "dataset_version", item.ID, nil, item.Public())
	response.JSON(ctx, http.StatusCreated, item.Public())
}

func (handler Handler) DeleteDraft(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	versionID, ok := pathID(ctx, "version_id")
	if !ok {
		return
	}
	expected, err := strconv.ParseUint(ctx.Query("expected_lock_version"), 10, 32)
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "expected_lock_version", Message: "缺少有效锁版本"},
		))
		return
	}
	if err := handler.service.DeleteDraft(
		ctx.Request.Context(), principal.ID(), versionID, uint32(expected),
	); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "dataset_version.draft_deleted", "dataset_version", versionID, nil, nil)
	ctx.Status(http.StatusNoContent)
}

type publishRequest struct {
	ReleaseNote         *string `json:"release_note"`
	ExpectedLockVersion uint32  `json:"expected_lock_version"`
}

func (handler Handler) PublishVersion(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	versionID, ok := pathID(ctx, "version_id")
	if !ok {
		return
	}
	var request publishRequest
	if !bindDatasetJSON(ctx, &request) {
		return
	}
	item, err := handler.service.Publish(
		ctx.Request.Context(), principal.ID(), versionID,
		PublishInput{ReleaseNote: request.ReleaseNote, ExpectedLockVersion: request.ExpectedLockVersion},
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "dataset_version.published", "dataset_version", versionID, nil, item.Public())
	response.JSON(ctx, http.StatusOK, item.Public())
}

func (handler Handler) ArchiveVersion(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	versionID, ok := pathID(ctx, "version_id")
	if !ok {
		return
	}
	var request lockRequest
	if !bindDatasetJSON(ctx, &request) {
		return
	}
	before, after, err := handler.service.ArchiveVersion(
		ctx.Request.Context(), principal.ID(), versionID, request.ExpectedLockVersion,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "dataset_version.archived", "dataset_version", versionID, before.Public(), after.Public())
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) ListCases(ctx *gin.Context) {
	version, _, ok := handler.visibleVersion(ctx)
	if !ok {
		return
	}
	page, ok := datasetPage(ctx)
	if !ok {
		return
	}
	items, total, err := handler.repository.ListCases(
		ctx.Request.Context(), version.ID, page.Page, page.PageSize,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	publicItems := make([]CasePublic, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, item.Public())
	}
	response.JSON(ctx, http.StatusOK, transport.PageData[CasePublic]{
		Items: publicItems, Page: page.Page, PageSize: page.PageSize, Total: total,
	})
}

func (handler Handler) GetCase(ctx *gin.Context) {
	caseID, ok := pathID(ctx, "case_id")
	if !ok {
		return
	}
	item, err := handler.repository.GetCase(ctx.Request.Context(), caseID)
	if err != nil {
		response.ApplicationError(ctx, mapNotFound(err))
		return
	}
	ctx.Params = append(ctx.Params, gin.Param{Key: "version_id", Value: item.DatasetVersionID.String()})
	if _, _, ok := handler.visibleVersion(ctx); !ok {
		return
	}
	response.JSON(ctx, http.StatusOK, item.Public())
}

type caseRequest struct {
	ScenarioID          *string  `json:"scenario_id"`
	Name                *string  `json:"name"`
	UserPrompt          string   `json:"user_prompt"`
	Precondition        *string  `json:"precondition"`
	ExpectedResult      *string  `json:"expected_result"`
	JudgingGuide        *string  `json:"judging_guide"`
	IsEnabled           *bool    `json:"is_enabled"`
	TagIDs              []string `json:"tag_ids"`
	ExpectedLockVersion uint32   `json:"expected_lock_version"`
}

func (handler Handler) CreateCase(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	versionID, ok := pathID(ctx, "version_id")
	if !ok {
		return
	}
	input, ok := bindCaseInput(ctx)
	if !ok {
		return
	}
	item, err := handler.service.CreateCase(ctx.Request.Context(), principal.ID(), versionID, input)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusCreated, item.Public())
}

func (handler Handler) UpdateCase(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	caseID, ok := pathID(ctx, "case_id")
	if !ok {
		return
	}
	input, ok := bindCaseInput(ctx)
	if !ok {
		return
	}
	_, after, err := handler.service.UpdateCase(ctx.Request.Context(), principal.ID(), caseID, input)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, after.Public())
}

func (handler Handler) DeleteCase(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	caseID, ok := pathID(ctx, "case_id")
	if !ok {
		return
	}
	expected, err := strconv.ParseUint(ctx.Query("expected_lock_version"), 10, 32)
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "expected_lock_version", Message: "缺少有效锁版本"},
		))
		return
	}
	if err := handler.service.DeleteCase(
		ctx.Request.Context(), principal.ID(), caseID, uint32(expected),
	); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

type reorderRequest struct {
	Items []struct {
		ID                  string `json:"id"`
		SortOrder           uint32 `json:"sort_order"`
		ExpectedLockVersion uint32 `json:"expected_lock_version"`
	} `json:"items"`
}

func (handler Handler) ReorderCases(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	versionID, ok := pathID(ctx, "version_id")
	if !ok {
		return
	}
	var request reorderRequest
	if !bindDatasetJSON(ctx, &request) {
		return
	}
	items := make([]ReorderItem, 0, len(request.Items))
	for index, item := range request.Items {
		itemID, err := id.Parse(item.ID)
		if err != nil {
			response.ApplicationError(ctx, apperror.Validation(
				apperror.FieldError{
					Field: "items", Message: fmt.Sprintf("第 %d 项用例 ID 格式错误", index+1),
				},
			))
			return
		}
		items = append(items, ReorderItem{
			ID: itemID, SortOrder: item.SortOrder, ExpectedLockVersion: item.ExpectedLockVersion,
		})
	}
	if err := handler.service.ReorderCases(
		ctx.Request.Context(), principal.ID(), versionID, items,
	); err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (handler Handler) ImportTemplate(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", `attachment; filename="quickeval-case-import-template.csv"`)
	if err := WriteCaseTemplate(ctx.Writer); err != nil {
		handler.logger.Error("write CSV template", "error", err)
	}
}

func (handler Handler) PreviewImport(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	versionID, ok := pathID(ctx, "version_id")
	if !ok {
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 6*1024*1024)
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "file", Message: "请选择 CSV 文件"},
		))
		return
	}
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".csv") {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "file", Message: "仅支持 .csv 文件"},
		))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	defer file.Close()
	result, err := handler.service.PreviewImport(
		ctx.Request.Context(), handler.imports, principal.ID(), versionID, file,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, result)
}

type commitImportRequest struct {
	ImportToken string `json:"import_token"`
}

func (handler Handler) CommitImport(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	versionID, ok := pathID(ctx, "version_id")
	if !ok {
		return
	}
	var request commitImportRequest
	if !bindDatasetJSON(ctx, &request) {
		return
	}
	items, err := handler.service.CommitImport(
		ctx.Request.Context(), handler.imports, principal.ID(), versionID, request.ImportToken,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	handler.record(ctx, principal.ID(), "version_cases.csv_imported", "dataset_version", versionID, nil, map[string]any{
		"case_count": len(items),
	})
	response.JSON(ctx, http.StatusCreated, map[string]any{"created_count": len(items)})
}

func (handler Handler) ExportCases(ctx *gin.Context) {
	version, _, ok := handler.visibleVersion(ctx)
	if !ok {
		return
	}
	items, err := handler.repository.AllCases(ctx.Request.Context(), version.ID)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf(
		`attachment; filename="quickeval-version-%s-cases.csv"`, version.ID.String(),
	))
	if err := WriteCaseCSV(ctx.Writer, items); err != nil {
		handler.logger.Error("write case CSV", "error", err)
	}
}

func (handler Handler) visibleDataset(ctx *gin.Context) (Dataset, bool) {
	datasetID, ok := pathID(ctx, "dataset_id")
	if !ok {
		return Dataset{}, false
	}
	item, err := handler.repository.GetDataset(ctx.Request.Context(), datasetID)
	if err != nil {
		response.ApplicationError(ctx, mapNotFound(err))
		return Dataset{}, false
	}
	principal, _ := access.From(ctx)
	if !principal.Admin() && item.Status != DatasetActive {
		response.ApplicationError(ctx, apperror.NotFound())
		return Dataset{}, false
	}
	return item, true
}

func (handler Handler) visibleVersion(ctx *gin.Context) (Version, Dataset, bool) {
	versionID, ok := pathID(ctx, "version_id")
	if !ok {
		return Version{}, Dataset{}, false
	}
	version, err := handler.repository.GetVersion(ctx.Request.Context(), versionID)
	if err != nil {
		response.ApplicationError(ctx, mapNotFound(err))
		return Version{}, Dataset{}, false
	}
	dataset, err := handler.repository.GetDataset(ctx.Request.Context(), version.DatasetID)
	if err != nil {
		response.ApplicationError(ctx, err)
		return Version{}, Dataset{}, false
	}
	principal, _ := access.From(ctx)
	if !principal.Admin() && (dataset.Status != DatasetActive || version.Status == VersionDraft) {
		response.ApplicationError(ctx, apperror.NotFound())
		return Version{}, Dataset{}, false
	}
	return version, dataset, true
}

func bindCaseInput(ctx *gin.Context) (CaseInput, bool) {
	var request caseRequest
	if !bindDatasetJSON(ctx, &request) {
		return CaseInput{}, false
	}
	tagIDs := make([]id.UUID, 0, len(request.TagIDs))
	for _, value := range request.TagIDs {
		parsed, err := id.Parse(value)
		if err != nil {
			response.ApplicationError(ctx, apperror.Validation(
				apperror.FieldError{Field: "tag_ids", Message: "用例标签 ID 格式错误"},
			))
			return CaseInput{}, false
		}
		tagIDs = append(tagIDs, parsed)
	}
	var scenarioID *id.UUID
	if request.ScenarioID != nil && strings.TrimSpace(*request.ScenarioID) != "" {
		parsed, ok := requestID(ctx, *request.ScenarioID, "scenario_id")
		if !ok {
			return CaseInput{}, false
		}
		scenarioID = &parsed
	}
	enabled := true
	if request.IsEnabled != nil {
		enabled = *request.IsEnabled
	}
	return CaseInput{
		ScenarioID: scenarioID, Name: request.Name, UserPrompt: request.UserPrompt, Precondition: request.Precondition,
		ExpectedResult: request.ExpectedResult, JudgingGuide: request.JudgingGuide,
		IsEnabled: enabled, TagIDs: tagIDs, ExpectedLockVersion: request.ExpectedLockVersion,
	}, true
}

func bindDatasetJSON(ctx *gin.Context, target any) bool {
	if err := ctx.ShouldBindJSON(target); err != nil {
		response.ApplicationError(ctx, apperror.Validation())
		return false
	}
	return true
}

func datasetPage(ctx *gin.Context) (transport.Page, bool) {
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

func optionalQueryID(ctx *gin.Context, key string) (*id.UUID, bool) {
	value := ctx.Query(key)
	if value == "" {
		return nil, true
	}
	parsed, ok := requestID(ctx, value, key)
	if !ok {
		return nil, false
	}
	return &parsed, true
}

func pathID(ctx *gin.Context, key string) (id.UUID, bool) {
	return requestID(ctx, ctx.Param(key), key)
}

func requestID(ctx *gin.Context, value, field string) (id.UUID, bool) {
	parsed, err := id.Parse(value)
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: field, Message: "ID 格式错误"},
		))
		return id.UUID{}, false
	}
	return parsed, true
}

func (handler Handler) record(
	ctx *gin.Context,
	actorID id.UUID,
	action, entityType string,
	entityID id.UUID,
	before, after any,
) {
	if err := handler.audit.Record(
		ctx.Request.Context(), &actorID, action, entityType, entityID,
		before, after, requestid.From(ctx), ctx.ClientIP(), ctx.Request.UserAgent(),
	); err != nil {
		handler.logger.Error("record audit log", "action", action, "error", err)
	}
}
