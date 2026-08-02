package dataset

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/redis/go-redis/v9"
)

const (
	maxImportRows = 5000
	csvBOM        = "\xEF\xBB\xBF"
)

var csvHeaders = []string{
	"用例名称", "用户问题", "前置条件", "期望结果", "评判要点", "用例标签", "是否启用",
}

type CSVFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ImportRow struct {
	RowNumber      int             `json:"row_number"`
	Name           string          `json:"name"`
	UserPrompt     string          `json:"user_prompt"`
	Precondition   string          `json:"precondition"`
	ExpectedResult string          `json:"expected_result"`
	JudgingGuide   string          `json:"judging_guide"`
	TagNames       []string        `json:"tag_names"`
	IsEnabled      bool            `json:"is_enabled"`
	TagIDs         []string        `json:"-"`
	Errors         []CSVFieldError `json:"errors"`
}

type ImportPreview struct {
	ActorID            string      `json:"actor_id"`
	VersionID          string      `json:"version_id"`
	VersionLockVersion uint32      `json:"version_lock_version"`
	Rows               []ImportRow `json:"rows"`
	HasErrors          bool        `json:"has_errors"`
}

type PreviewResult struct {
	ImportToken        string      `json:"import_token"`
	VersionLockVersion uint32      `json:"version_lock_version"`
	Rows               []ImportRow `json:"rows"`
	HasErrors          bool        `json:"has_errors"`
	ValidRowCount      int         `json:"valid_row_count"`
	ErrorRowCount      int         `json:"error_row_count"`
}

func (service Service) PreviewImport(
	ctx context.Context,
	store ImportPreviewStore,
	actorID, versionID id.UUID,
	reader io.Reader,
) (PreviewResult, error) {
	version, err := service.repository.GetVersion(ctx, versionID)
	if err != nil {
		return PreviewResult{}, mapNotFound(err)
	}
	if version.Status != VersionDraft {
		return PreviewResult{}, apperror.Conflict("VERSION_IMMUTABLE", "只能向草稿版本导入用例")
	}
	rows, err := parseCaseCSV(reader)
	if err != nil {
		return PreviewResult{}, err
	}

	allNames := []string{}
	seenNames := map[string]struct{}{}
	for _, row := range rows {
		for _, name := range row.TagNames {
			if _, exists := seenNames[name]; !exists {
				allNames = append(allNames, name)
				seenNames[name] = struct{}{}
			}
		}
	}
	tagIDs, err := service.repository.FindActiveTagsByNames(ctx, nil, allNames)
	if err != nil {
		return PreviewResult{}, err
	}
	hasErrors := false
	validCount := 0
	for index := range rows {
		for _, name := range rows[index].TagNames {
			tagID, exists := tagIDs[name]
			if !exists {
				rows[index].Errors = append(rows[index].Errors, CSVFieldError{
					Field: "tags", Message: "全局标签“" + name + "”不存在或已停用",
				})
				continue
			}
			rows[index].TagIDs = append(rows[index].TagIDs, tagID.String())
		}
		if len(rows[index].Errors) > 0 {
			hasErrors = true
		} else {
			validCount++
		}
	}
	preview := ImportPreview{
		ActorID: actorID.String(), VersionID: versionID.String(),
		VersionLockVersion: version.LockVersion, Rows: rows, HasErrors: hasErrors,
	}
	token, err := store.Save(ctx, preview)
	if err != nil {
		return PreviewResult{}, err
	}
	return PreviewResult{
		ImportToken: token, VersionLockVersion: version.LockVersion, Rows: rows,
		HasErrors: hasErrors, ValidRowCount: validCount, ErrorRowCount: len(rows) - validCount,
	}, nil
}

func (service Service) CommitImport(
	ctx context.Context,
	store ImportPreviewStore,
	actorID, versionID id.UUID,
	token string,
) ([]VersionCase, error) {
	preview, err := store.Consume(ctx, strings.TrimSpace(token))
	if errors.Is(err, redis.Nil) {
		return nil, apperror.Conflict("IMPORT_PREVIEW_EXPIRED", "导入预览已过期或已使用，请重新预览")
	}
	if err != nil {
		return nil, err
	}
	if preview.ActorID != actorID.String() || preview.VersionID != versionID.String() {
		return nil, apperror.Forbidden()
	}
	if preview.HasErrors {
		return nil, apperror.Conflict("IMPORT_HAS_ERRORS", "CSV 存在错误行，不能提交")
	}
	inputs := make([]CaseInput, 0, len(preview.Rows))
	for _, row := range preview.Rows {
		tagIDs := make([]id.UUID, 0, len(row.TagIDs))
		for _, value := range row.TagIDs {
			tagID, err := id.Parse(value)
			if err != nil {
				return nil, err
			}
			tagIDs = append(tagIDs, tagID)
		}
		inputs = append(inputs, CaseInput{
			Name: stringPointer(row.Name), UserPrompt: row.UserPrompt,
			Precondition: stringPointer(row.Precondition), ExpectedResult: stringPointer(row.ExpectedResult),
			JudgingGuide: stringPointer(row.JudgingGuide), IsEnabled: row.IsEnabled, TagIDs: tagIDs,
		})
	}
	return service.AppendCases(ctx, actorID, versionID, preview.VersionLockVersion, inputs)
}

func WriteCaseCSV(writer io.Writer, cases []VersionCase) error {
	if _, err := io.WriteString(writer, csvBOM); err != nil {
		return err
	}
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write(csvHeaders); err != nil {
		return err
	}
	for _, item := range cases {
		tagNames := make([]string, 0, len(item.Tags))
		for _, tag := range item.Tags {
			tagNames = append(tagNames, tag.Name)
		}
		enabled := "是"
		if !item.IsEnabled {
			enabled = "否"
		}
		if err := csvWriter.Write([]string{
			stringValue(item.Name), item.UserPrompt, stringValue(item.Precondition),
			stringValue(item.ExpectedResult), stringValue(item.JudgingGuide),
			strings.Join(tagNames, "|"), enabled,
		}); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func WriteCaseTemplate(writer io.Writer) error {
	return WriteCaseCSV(writer, []VersionCase{
		{
			Name: stringPointer("示例：查询商品交付周期"), UserPrompt: "请问该商品多久可以交付？",
			Precondition: stringPointer("进入商品详情页"), ExpectedResult: nil,
			JudgingGuide: stringPointer("回答应基于商品信息，不应编造"), IsEnabled: true,
		},
	})
}

func parseCaseCSV(reader io.Reader) ([]ImportRow, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.ReuseRecord = false
	header, err := csvReader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, apperror.Validation(
				apperror.FieldError{Field: "file", Message: "CSV 文件为空"},
			)
		}
		return nil, apperror.Validation(
			apperror.FieldError{Field: "file", Message: "CSV 表头无法解析"},
		)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], csvBOM)
	}
	if !validHeaders(header) {
		return nil, apperror.Validation(
			apperror.FieldError{Field: "file", Message: "CSV 表头与标准模板不一致"},
		)
	}
	rows := []ImportRow{}
	for rowNumber := 2; ; rowNumber++ {
		record, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, apperror.Validation(
				apperror.FieldError{
					Field: "file", Message: fmt.Sprintf("第 %d 行 CSV 格式错误：%s", rowNumber, err.Error()),
				},
			)
		}
		if len(rows) >= maxImportRows {
			return nil, apperror.Validation(
				apperror.FieldError{Field: "file", Message: "单次导入不能超过 5000 行"},
			)
		}
		for len(record) < len(csvHeaders) {
			record = append(record, "")
		}
		if len(record) > len(csvHeaders) {
			rows = append(rows, ImportRow{
				RowNumber: rowNumber,
				Errors:    []CSVFieldError{{Field: "row", Message: "列数超过标准模板"}},
			})
			continue
		}
		row := ImportRow{
			RowNumber: rowNumber, Name: strings.TrimSpace(record[0]),
			UserPrompt: strings.TrimSpace(record[1]), Precondition: strings.TrimSpace(record[2]),
			ExpectedResult: strings.TrimSpace(record[3]), JudgingGuide: strings.TrimSpace(record[4]),
			TagNames: parseTagNames(record[5]),
		}
		row.IsEnabled, err = parseEnabled(record[6])
		if err != nil {
			row.Errors = append(row.Errors, CSVFieldError{Field: "is_enabled", Message: err.Error()})
		}
		if row.UserPrompt == "" {
			row.Errors = append(row.Errors, CSVFieldError{Field: "user_prompt", Message: "用户问题不能为空"})
		}
		if len([]rune(row.Name)) > 200 {
			row.Errors = append(row.Errors, CSVFieldError{Field: "name", Message: "用例名称不能超过 200 个字符"})
		}
		if !utf8.ValidString(strings.Join(record, "")) {
			row.Errors = append(row.Errors, CSVFieldError{Field: "row", Message: "内容不是有效 UTF-8"})
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, apperror.Validation(
			apperror.FieldError{Field: "file", Message: "CSV 没有可导入的数据行"},
		)
	}
	return rows, nil
}

func validHeaders(header []string) bool {
	if len(header) != len(csvHeaders) {
		return false
	}
	for index := range csvHeaders {
		if strings.TrimSpace(header[index]) != csvHeaders[index] {
			return false
		}
	}
	return true
}

func parseTagNames(value string) []string {
	parts := strings.FieldsFunc(value, func(character rune) bool {
		return character == '|' || character == '；' || character == ';'
	})
	result := []string{}
	seen := map[string]struct{}{}
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; !exists {
			result = append(result, name)
			seen[name] = struct{}{}
		}
	}
	return result
}

func parseEnabled(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "是", "true", "1", "启用", "yes":
		return true, nil
	case "否", "false", "0", "停用", "no":
		return false, nil
	default:
		return false, fmt.Errorf("是否启用只能填写“是”或“否”")
	}
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func ParseExpectedVersion(value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	return uint32(parsed), err
}
