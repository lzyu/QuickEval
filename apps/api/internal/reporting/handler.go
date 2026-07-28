package reporting

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/access"
	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/response"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/lzyu/QuickEval/apps/api/internal/transport"
)

type Handler struct {
	repository Repository
}

func NewHandler(repository Repository) Handler {
	return Handler{repository: repository}
}

func (handler Handler) Home(ctx *gin.Context) {
	principal, _ := access.From(ctx)
	result, err := handler.repository.Home(ctx.Request.Context(), principal.ID())
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, result)
}

func (handler Handler) Dashboard(ctx *gin.Context) {
	filters, ok := parseFilters(ctx)
	if !ok {
		return
	}
	result, err := handler.repository.Dashboard(ctx.Request.Context(), filters)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, result)
}

func (handler Handler) VersionComparison(ctx *gin.Context) {
	datasetID, err := id.Parse(ctx.Param("dataset_id"))
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "dataset_id", Message: "ID 格式错误"},
		))
		return
	}
	filters, ok := parseFilters(ctx)
	if !ok {
		return
	}
	filters.DatasetID = &datasetID
	items, err := handler.repository.versionComparison(ctx.Request.Context(), filters)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, map[string]any{"items": items})
}

func (handler Handler) EvaluationResults(ctx *gin.Context) {
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
	filters, ok := parseFilters(ctx)
	if !ok {
		return
	}
	resultID, ok := optionalID(ctx, "result_id")
	if !ok {
		return
	}
	detailFilters := EvaluationResultFilters{
		Filters:      filters,
		ResultID:     resultID,
		ResultStatus: strings.TrimSpace(ctx.Query("result_status")),
		SkipReason:   strings.TrimSpace(ctx.Query("skip_reason")),
		Keyword:      strings.TrimSpace(ctx.Query("keyword")),
	}
	if detailFilters.ResultStatus != "" &&
		!contains([]string{"evaluated", "skipped"}, detailFilters.ResultStatus) {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "result_status", Message: "筛选值不合法"},
		))
		return
	}
	if len([]rune(detailFilters.SkipReason)) > 500 || len([]rune(detailFilters.Keyword)) > 100 {
		response.ApplicationError(ctx, apperror.Validation())
		return
	}
	if raw := strings.TrimSpace(ctx.Query("score")); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 8)
		if err != nil || value < 1 || value > 5 {
			response.ApplicationError(ctx, apperror.Validation(
				apperror.FieldError{Field: "score", Message: "评分必须为 1 到 5"},
			))
			return
		}
		score := uint8(value)
		detailFilters.Score = &score
	}
	detailFilters.HasBadcase, ok = optionalBool(ctx, "has_badcase")
	if !ok {
		return
	}
	detailFilters.Scored, ok = optionalBool(ctx, "scored")
	if !ok {
		return
	}
	items, total, runCount, err := handler.repository.EvaluationResults(
		ctx.Request.Context(), page.Page, page.PageSize, detailFilters,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, struct {
		transport.PageData[EvaluationResultDetail]
		CompletedRunCount int64 `json:"completed_run_count"`
	}{
		PageData: transport.PageData[EvaluationResultDetail]{
			Items: items, Page: page.Page, PageSize: page.PageSize, Total: total,
		},
		CompletedRunCount: runCount,
	})
}

func (handler Handler) Search(ctx *gin.Context) {
	keyword := strings.TrimSpace(ctx.Query("q"))
	if keyword == "" || len([]rune(keyword)) > 100 {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "q", Message: "关键词不能为空且最多 100 个字符"},
		))
		return
	}
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
	types, ok := searchTypes(ctx.Query("types"))
	if !ok {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "types", Message: "搜索类型不合法"},
		))
		return
	}
	result, err := handler.repository.Search(
		ctx.Request.Context(), keyword, types, page.Page, page.PageSize,
	)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	response.JSON(ctx, http.StatusOK, result)
}

func (handler Handler) ExportEvaluationResults(ctx *gin.Context) {
	filters, ok := parseFilters(ctx)
	if !ok {
		return
	}
	count, err := handler.repository.EvaluationExportCount(ctx.Request.Context(), filters)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	if !exportSizeValid(ctx, count) {
		return
	}
	writeCSV(ctx, "quickeval-evaluation-results.csv", []string{
		"评测ID", "评测集", "版本", "评测对象", "场景", "评测人", "Agent版本",
		"环境", "完成时间", "评测用例总数", "评测已评数", "评测已评分数", "评测跳过数",
		"评测平均分", "评测Badcase数", "结果ID", "用例名称", "用户问题", "结果状态",
		"Agent回答", "评分", "评语", "跳过原因", "是否Badcase", "Badcase ID",
		"Badcase标题", "用例标签", "截图鉴权地址",
	}, func(writer *csv.Writer) error {
		return handler.repository.StreamEvaluationExportRows(
			ctx.Request.Context(), filters, func(row EvaluationExportRow) error {
				score := ""
				if row.Score != nil {
					score = strconv.Itoa(int(*row.Score))
				}
				completedAt := ""
				if row.CompletedAt != nil {
					completedAt = row.CompletedAt.UTC().Format(time.RFC3339)
				}
				runAverage := ""
				if row.RunAverage != nil {
					runAverage = strconv.FormatFloat(*row.RunAverage, 'f', 4, 64)
				}
				return writer.Write(csvSafeRow([]string{
					row.RunID, row.DatasetName, strconv.Itoa(int(row.VersionNo)),
					row.TargetName, row.ScenarioName, row.EvaluatorName, row.AgentVersion,
					row.Environment, completedAt, strconv.FormatInt(row.RunTotalCount, 10),
					strconv.FormatInt(row.RunEvaluated, 10), strconv.FormatInt(row.RunScored, 10),
					strconv.FormatInt(row.RunSkipped, 10), runAverage,
					strconv.FormatInt(row.RunBadcases, 10), row.ResultID, optionalString(row.CaseName),
					row.UserPrompt, row.ResultStatus, optionalString(row.AnswerText), score,
					optionalString(row.Comment), optionalString(row.SkipReason),
					strconv.FormatBool(row.HasBadcase), optionalString(row.BadcaseID),
					optionalString(row.BadcaseTitle), row.CaseTags, row.EvidenceURLs,
				}))
			},
		)
	})
}

func (handler Handler) ExportBadcases(ctx *gin.Context) {
	filters, ok := parseFilters(ctx)
	if !ok {
		return
	}
	count, err := handler.repository.BadcaseExportCount(ctx.Request.Context(), filters)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	if !exportSizeValid(ctx, count) {
		return
	}
	writeCSV(ctx, "quickeval-badcases.csv", []string{
		"Badcase ID", "来源", "评测对象", "场景", "标题", "问题描述", "Agent回答",
		"Agent版本", "环境", "发生时间", "状态", "负责人", "创建人", "业务单号",
		"会话ID", "问题标签", "原始评测截图鉴权地址", "补充截图鉴权地址", "创建时间", "更新时间",
	}, func(writer *csv.Writer) error {
		return handler.repository.StreamBadcaseExportRows(
			ctx.Request.Context(), filters, func(row BadcaseExportRow) error {
				return writer.Write(csvSafeRow([]string{
					row.ID, row.SourceType, row.TargetName, row.ScenarioName, row.Title,
					optionalString(row.Description), optionalString(row.AgentResponseText),
					optionalString(row.AgentVersion), row.Environment, row.OccurredAt.UTC().Format(time.RFC3339),
					row.Status, optionalString(row.AssigneeName), row.CreatorName,
					optionalString(row.BusinessReference), optionalString(row.SessionID),
					row.IssueTags, row.OriginalEvidenceURLs, row.SupplementalEvidenceURLs,
					row.CreatedAt.UTC().Format(time.RFC3339), row.UpdatedAt.UTC().Format(time.RFC3339),
				}))
			},
		)
	})
}

func (handler Handler) ExportBadcaseDistribution(ctx *gin.Context) {
	filters, ok := parseFilters(ctx)
	if !ok {
		return
	}
	rows, err := handler.repository.DistributionExportRows(ctx.Request.Context(), filters)
	if err != nil {
		response.ApplicationError(ctx, err)
		return
	}
	if !exportSizeValid(ctx, int64(len(rows))) {
		return
	}
	writeCSV(ctx, "quickeval-badcase-distribution.csv",
		[]string{"维度", "键", "名称", "数量"},
		func(writer *csv.Writer) error {
			for _, row := range rows {
				parts := strings.SplitN(row.Key, ":", 2)
				if err := writer.Write(csvSafeRow([]string{
					parts[0], parts[1], row.Label, strconv.FormatInt(row.Count, 10),
				})); err != nil {
					return err
				}
			}
			return nil
		},
	)
}

func parseFilters(ctx *gin.Context) (Filters, bool) {
	var filters Filters
	for _, item := range []struct {
		key    string
		target **id.UUID
	}{
		{"evaluation_target_id", &filters.TargetID},
		{"scenario_id", &filters.ScenarioID},
		{"dataset_id", &filters.DatasetID},
		{"dataset_version_id", &filters.DatasetVersionID},
		{"evaluator_id", &filters.EvaluatorID},
		{"issue_tag_id", &filters.IssueTagID},
	} {
		raw := strings.TrimSpace(ctx.Query(item.key))
		if raw == "" {
			continue
		}
		value, err := id.Parse(raw)
		if err != nil {
			response.ApplicationError(ctx, apperror.Validation(
				apperror.FieldError{Field: item.key, Message: "ID 格式错误"},
			))
			return Filters{}, false
		}
		*item.target = &value
	}
	filters.AgentVersion = strings.TrimSpace(ctx.Query("agent_version"))
	filters.Environment = strings.TrimSpace(ctx.Query("environment"))
	filters.SourceType = strings.TrimSpace(ctx.Query("source_type"))
	filters.BadcaseStatus = strings.TrimSpace(ctx.Query("badcase_status"))
	if filters.Environment != "" &&
		!contains([]string{"test", "staging", "production", "other"}, filters.Environment) {
		return invalidFilter(ctx, "environment")
	}
	if filters.SourceType != "" &&
		!contains([]string{"evaluation", "business"}, filters.SourceType) {
		return invalidFilter(ctx, "source_type")
	}
	if filters.BadcaseStatus != "" &&
		!contains([]string{"pending", "processing", "resolved", "deferred"}, filters.BadcaseStatus) {
		return invalidFilter(ctx, "badcase_status")
	}
	var ok bool
	filters.From, ok = optionalTime(ctx, "from")
	if !ok {
		return Filters{}, false
	}
	filters.To, ok = optionalTime(ctx, "to")
	if !ok {
		return Filters{}, false
	}
	if filters.From != nil && filters.To != nil && filters.From.After(*filters.To) {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: "to", Message: "结束时间不能早于开始时间"},
		))
		return Filters{}, false
	}
	return filters, true
}

func invalidFilter(ctx *gin.Context, field string) (Filters, bool) {
	response.ApplicationError(ctx, apperror.Validation(
		apperror.FieldError{Field: field, Message: "筛选值不合法"},
	))
	return Filters{}, false
}

func optionalTime(ctx *gin.Context, key string) (*time.Time, bool) {
	raw := strings.TrimSpace(ctx.Query(key))
	if raw == "" {
		return nil, true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: key, Message: "时间必须为 RFC3339 格式"},
		))
		return nil, false
	}
	value = value.UTC()
	return &value, true
}

func optionalID(ctx *gin.Context, key string) (*id.UUID, bool) {
	raw := strings.TrimSpace(ctx.Query(key))
	if raw == "" {
		return nil, true
	}
	value, err := id.Parse(raw)
	if err != nil {
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: key, Message: "ID 格式错误"},
		))
		return nil, false
	}
	return &value, true
}

func optionalBool(ctx *gin.Context, key string) (*bool, bool) {
	raw := strings.TrimSpace(ctx.Query(key))
	if raw == "" {
		return nil, true
	}
	switch raw {
	case "1", "true":
		value := true
		return &value, true
	case "0", "false":
		value := false
		return &value, true
	default:
		response.ApplicationError(ctx, apperror.Validation(
			apperror.FieldError{Field: key, Message: "布尔筛选必须为 0 或 1"},
		))
		return nil, false
	}
}

func searchTypes(raw string) ([]string, bool) {
	if strings.TrimSpace(raw) == "" {
		return []string{
			"target", "scenario", "dataset", "case", "evaluation_result", "badcase",
		}, true
	}
	values := strings.Split(raw, ",")
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !contains([]string{
			"target", "scenario", "dataset", "case", "evaluation_result", "badcase",
		}, value) {
			return nil, false
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result, true
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func exportSizeValid(ctx *gin.Context, size int64) bool {
	if size <= int64(ExportLimit) {
		return true
	}
	response.ApplicationError(ctx, apperror.New(
		http.StatusUnprocessableEntity,
		"EXPORT_TOO_LARGE",
		fmt.Sprintf("导出结果超过 %d 行，请缩小筛选范围", ExportLimit),
	))
	return false
}

func writeCSV(
	ctx *gin.Context,
	filename string,
	header []string,
	writeRows func(*csv.Writer) error,
) {
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	ctx.Header("Cache-Control", "private, no-store")
	ctx.Status(http.StatusOK)
	_, _ = ctx.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(ctx.Writer)
	if err := writer.Write(header); err == nil {
		err = writeRows(writer)
		if err != nil {
			ctx.Error(err)
		}
	} else {
		ctx.Error(err)
	}
	writer.Flush()
}

func csvSafeRow(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		trimmed := strings.TrimLeft(value, " \t\r\n")
		if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
			value = "'" + value
		}
		result[index] = value
	}
	return result
}
