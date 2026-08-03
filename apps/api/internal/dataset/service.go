package dataset

import (
	"context"
	"errors"
	"strings"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) Service { return Service{repository: repository} }

type DatasetInput struct {
	TargetID            id.UUID
	Name                string
	Description         *string
	ExpectedLockVersion uint32
}

type CreateResult struct {
	Dataset Dataset
	Draft   Version
}

func (service Service) CreateDataset(
	ctx context.Context,
	actorID id.UUID,
	input DatasetInput,
) (CreateResult, error) {
	name, description, err := validateName(input.Name, input.Description)
	if err != nil {
		return CreateResult{}, err
	}
	var result CreateResult
	err = service.repository.Transaction(ctx, func(repository Repository) error {
		target, err := repository.GetTargetInfo(ctx, input.TargetID)
		if err != nil {
			return mapNotFound(err)
		}
		if err := validateTargetAvailability(target); err != nil {
			return err
		}
		result.Dataset = Dataset{
			ID: id.MustNew(), TargetID: input.TargetID, Name: name, Description: description,
			Status: DatasetActive, CreatedBy: actorID, UpdatedBy: actorID,
		}
		if err := repository.CreateDataset(ctx, &result.Dataset); err != nil {
			return mapWriteError(err)
		}
		result.Draft = newDraft(result.Dataset.ID, nil, actorID)
		if err := repository.CreateVersion(ctx, &result.Draft); err != nil {
			return mapWriteError(err)
		}
		return nil
	})
	if err != nil {
		return CreateResult{}, err
	}
	result.Dataset, err = service.repository.GetDataset(ctx, result.Dataset.ID)
	if err != nil {
		return CreateResult{}, err
	}
	result.Draft, err = service.repository.GetVersion(ctx, result.Draft.ID)
	return result, err
}

func (service Service) UpdateDataset(
	ctx context.Context,
	actorID, datasetID id.UUID,
	input DatasetInput,
) (Dataset, Dataset, error) {
	before, err := service.repository.GetDataset(ctx, datasetID)
	if err != nil {
		return Dataset{}, Dataset{}, mapNotFound(err)
	}
	name, description, err := validateName(input.Name, input.Description)
	if err != nil {
		return Dataset{}, Dataset{}, err
	}
	target, err := service.repository.GetTargetInfo(ctx, input.TargetID)
	if err != nil {
		return Dataset{}, Dataset{}, mapNotFound(err)
	}
	if err := validateTargetAvailability(target); err != nil {
		return Dataset{}, Dataset{}, err
	}
	if before.TargetID != input.TargetID && (before.PublishedVersionCount > 0 || before.DraftCaseCount > 0) {
		return Dataset{}, Dataset{}, apperror.Conflict(
			"DATASET_TARGET_LOCKED",
			"评测集已有用例或发布版本，不能更换评测对象",
		)
	}
	if err := service.repository.UpdateDataset(
		ctx, datasetID, input.TargetID, actorID, name, description, input.ExpectedLockVersion,
	); err != nil {
		return Dataset{}, Dataset{}, mapWriteError(err)
	}
	after, err := service.repository.GetDataset(ctx, datasetID)
	return before, after, err
}

func (service Service) SetDatasetStatus(
	ctx context.Context,
	actorID, datasetID id.UUID,
	status string,
	expectedVersion uint32,
) (Dataset, Dataset, error) {
	before, err := service.repository.GetDataset(ctx, datasetID)
	if err != nil {
		return Dataset{}, Dataset{}, mapNotFound(err)
	}
	if status != DatasetActive && status != DatasetArchived {
		return Dataset{}, Dataset{}, apperror.Validation()
	}
	if before.Status == status {
		return Dataset{}, Dataset{}, apperror.Conflict("INVALID_STATE_TRANSITION", "评测集已经处于目标状态")
	}
	if status == DatasetArchived {
		count, err := service.repository.CountDrafts(ctx, datasetID)
		if err != nil {
			return Dataset{}, Dataset{}, err
		}
		if count > 0 {
			return Dataset{}, Dataset{}, apperror.Conflict(
				"DRAFT_EXISTS", "请先发布或删除草稿后再归档评测集",
			)
		}
	}
	if err := service.repository.SetDatasetStatus(
		ctx, datasetID, actorID, status, expectedVersion,
	); err != nil {
		return Dataset{}, Dataset{}, mapWriteError(err)
	}
	after, err := service.repository.GetDataset(ctx, datasetID)
	return before, after, err
}

type CreateDraftInput struct {
	BaseVersionID              *id.UUID
	ExpectedDatasetLockVersion uint32
}

func (service Service) CreateDraft(
	ctx context.Context,
	actorID, datasetID id.UUID,
	input CreateDraftInput,
) (Version, error) {
	var draft Version
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		item, err := repository.LockDataset(ctx, datasetID)
		if err != nil {
			return mapNotFound(err)
		}
		if item.LockVersion != input.ExpectedDatasetLockVersion {
			return mapWriteError(ErrLockConflict)
		}
		if err := validateDatasetAvailability(item); err != nil {
			return err
		}
		count, err := repository.CountDrafts(ctx, datasetID)
		if err != nil {
			return err
		}
		if count > 0 {
			return apperror.Conflict("DRAFT_EXISTS", "评测集已经存在草稿")
		}
		var baseCases []VersionCase
		if input.BaseVersionID != nil {
			base, err := repository.GetVersion(ctx, *input.BaseVersionID)
			if err != nil {
				return mapNotFound(err)
			}
			if base.DatasetID != datasetID || base.Status == VersionDraft {
				return apperror.Conflict("INVALID_BASE_VERSION", "只能从当前评测集的已发布版本复制草稿")
			}
			baseCases, err = repository.AllCases(ctx, base.ID)
			if err != nil {
				return err
			}
		}
		draft = newDraft(datasetID, input.BaseVersionID, actorID)
		if err := repository.CreateVersion(ctx, &draft); err != nil {
			return mapWriteError(err)
		}
		for _, source := range baseCases {
			copied := source
			copied.ID = id.MustNew()
			copied.DatasetVersionID = draft.ID
			copied.LockVersion = 0
			copied.CreatedBy = actorID
			copied.UpdatedBy = actorID
			copied.CreatedAt = draft.CreatedAt
			copied.UpdatedAt = draft.UpdatedAt
			tags, err := copiedTagRecords(copied.ID, source.Tags, actorID)
			if err != nil {
				return err
			}
			if err := repository.CreateCase(ctx, &copied, tags); err != nil {
				return err
			}
		}
		return repository.BumpDatasetLock(ctx, datasetID, actorID, input.ExpectedDatasetLockVersion)
	})
	if err != nil {
		return Version{}, err
	}
	return service.repository.GetVersion(ctx, draft.ID)
}

func (service Service) DeleteDraft(
	ctx context.Context,
	actorID, versionID id.UUID,
	expectedVersion uint32,
) error {
	return service.repository.Transaction(ctx, func(repository Repository) error {
		version, err := repository.LockVersion(ctx, versionID)
		if err != nil {
			return mapNotFound(err)
		}
		if version.Status != VersionDraft {
			return apperror.Conflict("VERSION_IMMUTABLE", "已发布或归档版本不能删除")
		}
		if version.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		dataset, err := repository.LockDataset(ctx, version.DatasetID)
		if err != nil {
			return err
		}
		if err := repository.DeleteDraft(ctx, versionID); err != nil {
			return mapWriteError(err)
		}
		return repository.BumpDatasetLock(ctx, dataset.ID, actorID, dataset.LockVersion)
	})
}

type PublishInput struct {
	ReleaseNote         *string
	ExpectedLockVersion uint32
}

func (service Service) Publish(
	ctx context.Context,
	actorID, versionID id.UUID,
	input PublishInput,
) (Version, error) {
	var published Version
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		version, err := repository.LockVersion(ctx, versionID)
		if err != nil {
			return mapNotFound(err)
		}
		if version.Status != VersionDraft {
			return apperror.Conflict("VERSION_IMMUTABLE", "只有草稿可以发布")
		}
		if version.LockVersion != input.ExpectedLockVersion {
			return mapWriteError(ErrLockConflict)
		}
		dataset, err := repository.LockDataset(ctx, version.DatasetID)
		if err != nil {
			return err
		}
		if err := validateDatasetAvailability(dataset); err != nil {
			return err
		}
		enabled, err := repository.CountEnabledCases(ctx, versionID)
		if err != nil {
			return err
		}
		if enabled == 0 {
			return apperror.Conflict("NO_ENABLED_CASES", "至少需要一条启用用例才能发布")
		}
		maxVersion, err := repository.MaxVersionNo(ctx, version.DatasetID)
		if err != nil {
			return err
		}
		note := normalizeOptional(input.ReleaseNote)
		if err := repository.PublishVersion(
			ctx, versionID, actorID, maxVersion+1, note, input.ExpectedLockVersion,
		); err != nil {
			return mapWriteError(err)
		}
		published, err = repository.GetVersion(ctx, versionID)
		return err
	})
	return published, err
}

func (service Service) ArchiveVersion(
	ctx context.Context,
	actorID, versionID id.UUID,
	expectedVersion uint32,
) (Version, Version, error) {
	before, err := service.repository.GetVersion(ctx, versionID)
	if err != nil {
		return Version{}, Version{}, mapNotFound(err)
	}
	if before.Status != VersionPublished {
		return Version{}, Version{}, apperror.Conflict("INVALID_STATE_TRANSITION", "只有已发布版本可以归档")
	}
	if err := service.repository.ArchiveVersion(ctx, versionID, actorID, expectedVersion); err != nil {
		return Version{}, Version{}, mapWriteError(err)
	}
	after, err := service.repository.GetVersion(ctx, versionID)
	return before, after, err
}

type CaseInput struct {
	ScenarioID          *id.UUID
	Name                *string
	UserPrompt          string
	Precondition        *string
	ExpectedResult      *string
	JudgingGuide        *string
	IsEnabled           bool
	TagIDs              []id.UUID
	ExpectedLockVersion uint32
}

type ReorderItem struct {
	ID                  id.UUID
	SortOrder           uint32
	ExpectedLockVersion uint32
}

func (service Service) CreateCase(
	ctx context.Context,
	actorID, versionID id.UUID,
	input CaseInput,
) (VersionCase, error) {
	normalized, err := validateCaseInput(input)
	if err != nil {
		return VersionCase{}, err
	}
	var item VersionCase
	err = service.repository.Transaction(ctx, func(repository Repository) error {
		version, dataset, err := requireDraft(ctx, repository, versionID)
		if err != nil {
			return err
		}
		if err := service.validateCaseScenario(ctx, repository, dataset.TargetID, normalized.ScenarioID); err != nil {
			return err
		}
		tags, err := service.caseTagRecords(ctx, repository, dataset.TargetID, id.UUID{}, actorID, normalized.TagIDs)
		if err != nil {
			return err
		}
		sortOrder, err := repository.NextSortOrder(ctx, versionID)
		if err != nil {
			return err
		}
		item = VersionCase{
			ID: id.MustNew(), DatasetVersionID: versionID, CaseKey: id.MustNew(),
			ScenarioID: normalized.ScenarioID, AssignmentStatus: assignmentStatus(normalized.ScenarioID),
			Name: normalized.Name, UserPrompt: normalized.UserPrompt, Precondition: normalized.Precondition,
			ExpectedResult: normalized.ExpectedResult, JudgingGuide: normalized.JudgingGuide,
			SortOrder: sortOrder, IsEnabled: normalized.IsEnabled, CreatedBy: actorID, UpdatedBy: actorID,
		}
		for index := range tags {
			tags[index].VersionCaseID = item.ID
		}
		if err := repository.CreateCase(ctx, &item, tags); err != nil {
			return mapWriteError(err)
		}
		return repository.BumpVersionLock(ctx, version.ID, actorID)
	})
	if err != nil {
		return VersionCase{}, err
	}
	return service.repository.GetCase(ctx, item.ID)
}

func (service Service) UpdateCase(
	ctx context.Context,
	actorID, caseID id.UUID,
	input CaseInput,
) (VersionCase, VersionCase, error) {
	normalized, err := validateCaseInput(input)
	if err != nil {
		return VersionCase{}, VersionCase{}, err
	}
	var before VersionCase
	err = service.repository.Transaction(ctx, func(repository Repository) error {
		before, err = repository.GetCase(ctx, caseID)
		if err != nil {
			return mapNotFound(err)
		}
		version, dataset, err := requireDraft(ctx, repository, before.DatasetVersionID)
		if err != nil {
			return err
		}
		if err := service.validateCaseScenario(ctx, repository, dataset.TargetID, normalized.ScenarioID); err != nil {
			return err
		}
		tags, err := service.caseTagRecords(ctx, repository, dataset.TargetID, caseID, actorID, normalized.TagIDs)
		if err != nil {
			return err
		}
		updated := before
		updated.ScenarioID = normalized.ScenarioID
		updated.AssignmentStatus = assignmentStatus(normalized.ScenarioID)
		updated.Name = normalized.Name
		updated.UserPrompt = normalized.UserPrompt
		updated.Precondition = normalized.Precondition
		updated.ExpectedResult = normalized.ExpectedResult
		updated.JudgingGuide = normalized.JudgingGuide
		updated.IsEnabled = normalized.IsEnabled
		if err := repository.UpdateCase(ctx, updated, actorID, input.ExpectedLockVersion, tags); err != nil {
			return mapWriteError(err)
		}
		return repository.BumpVersionLock(ctx, version.ID, actorID)
	})
	if err != nil {
		return VersionCase{}, VersionCase{}, err
	}
	after, err := service.repository.GetCase(ctx, caseID)
	return before, after, err
}

func (service Service) DeleteCase(
	ctx context.Context,
	actorID, caseID id.UUID,
	expectedVersion uint32,
) error {
	return service.repository.Transaction(ctx, func(repository Repository) error {
		item, err := repository.GetCase(ctx, caseID)
		if err != nil {
			return mapNotFound(err)
		}
		version, _, err := requireDraft(ctx, repository, item.DatasetVersionID)
		if err != nil {
			return err
		}
		if err := repository.DeleteCase(ctx, caseID, expectedVersion); err != nil {
			return mapWriteError(err)
		}
		return repository.BumpVersionLock(ctx, version.ID, actorID)
	})
}

func (service Service) ReorderCases(
	ctx context.Context,
	actorID, versionID id.UUID,
	items []ReorderItem,
) error {
	if err := validateReorder(items); err != nil {
		return err
	}
	return service.repository.Transaction(ctx, func(repository Repository) error {
		version, _, err := requireDraft(ctx, repository, versionID)
		if err != nil {
			return err
		}
		if err := repository.ReorderCases(ctx, versionID, actorID, items); err != nil {
			return mapWriteError(err)
		}
		return repository.BumpVersionLock(ctx, version.ID, actorID)
	})
}

func (service Service) AppendCases(
	ctx context.Context,
	actorID, versionID id.UUID,
	expectedVersion uint32,
	inputs []CaseInput,
) ([]VersionCase, error) {
	var created []VersionCase
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		version, dataset, err := requireDraft(ctx, repository, versionID)
		if err != nil {
			return err
		}
		if version.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		sortOrder, err := repository.NextSortOrder(ctx, versionID)
		if err != nil {
			return err
		}
		for _, input := range inputs {
			normalized, err := validateCaseInput(input)
			if err != nil {
				return err
			}
			item := VersionCase{
				ID: id.MustNew(), DatasetVersionID: versionID, CaseKey: id.MustNew(),
				ScenarioID: normalized.ScenarioID, AssignmentStatus: assignmentStatus(normalized.ScenarioID),
				Name: normalized.Name, UserPrompt: normalized.UserPrompt, Precondition: normalized.Precondition,
				ExpectedResult: normalized.ExpectedResult, JudgingGuide: normalized.JudgingGuide,
				SortOrder: sortOrder, IsEnabled: normalized.IsEnabled, CreatedBy: actorID, UpdatedBy: actorID,
			}
			if err := service.validateCaseScenario(ctx, repository, dataset.TargetID, normalized.ScenarioID); err != nil {
				return err
			}
			tags, err := service.caseTagRecords(ctx, repository, dataset.TargetID, item.ID, actorID, normalized.TagIDs)
			if err != nil {
				return err
			}
			if err := repository.CreateCase(ctx, &item, tags); err != nil {
				return mapWriteError(err)
			}
			created = append(created, item)
			sortOrder += 10
		}
		return repository.BumpVersionLock(ctx, version.ID, actorID)
	})
	return created, err
}

func (service Service) caseTagRecords(
	ctx context.Context,
	repository Repository,
	targetID id.UUID,
	caseID, actorID id.UUID,
	tagIDs []id.UUID,
) ([]VersionCaseTag, error) {
	names, err := repository.FindActiveTags(ctx, targetID, tagIDs)
	if err != nil {
		return nil, err
	}
	if len(names) != len(tagIDs) {
		return nil, apperror.Validation(
			apperror.FieldError{Field: "tag_ids", Message: "包含不存在、停用或不适用于当前评测对象的标签"},
		)
	}
	tags := make([]VersionCaseTag, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		tags = append(tags, VersionCaseTag{
			ID: id.MustNew(), VersionCaseID: caseID, CaseTagID: tagID,
			TagNameSnapshot: names[tagID], CreatedBy: actorID,
		})
	}
	return tags, nil
}

func (service Service) validateCaseScenario(
	ctx context.Context,
	repository Repository,
	targetID id.UUID,
	scenarioID *id.UUID,
) error {
	if scenarioID == nil {
		return nil
	}
	scenario, err := repository.GetScenarioInfo(ctx, *scenarioID)
	if err != nil {
		return mapNotFound(err)
	}
	if scenario.TargetID != targetID {
		return apperror.Validation(
			apperror.FieldError{Field: "scenario_id", Message: "场景不属于当前评测对象"},
		)
	}
	if scenario.Status != "active" {
		return apperror.Validation(
			apperror.FieldError{Field: "scenario_id", Message: "停用场景不能用于新的用例归类"},
		)
	}
	return nil
}

func assignmentStatus(scenarioID *id.UUID) string {
	if scenarioID == nil {
		return "unclassified"
	}
	return "confirmed"
}

func requireDraft(
	ctx context.Context,
	repository Repository,
	versionID id.UUID,
) (Version, Dataset, error) {
	version, err := repository.LockVersion(ctx, versionID)
	if err != nil {
		return Version{}, Dataset{}, mapNotFound(err)
	}
	if version.Status != VersionDraft {
		return Version{}, Dataset{}, apperror.Conflict("VERSION_IMMUTABLE", "已发布或归档版本内容不可修改")
	}
	dataset, err := repository.GetDataset(ctx, version.DatasetID)
	if err != nil {
		return Version{}, Dataset{}, err
	}
	if err := validateDatasetAvailability(dataset); err != nil {
		return Version{}, Dataset{}, err
	}
	return version, dataset, nil
}

func validateTargetAvailability(item TargetInfo) error {
	if item.Status != "active" {
		return apperror.Conflict(
			"EVALUATION_TARGET_DISABLED", "评测对象已停用，不能继续创建或调整评测集",
		)
	}
	return nil
}

func validateDatasetAvailability(item Dataset) error {
	if item.TargetStatus != "active" {
		return apperror.Conflict(
			"EVALUATION_TARGET_DISABLED", "评测对象已停用，评测集只保留历史查看",
		)
	}
	if item.Status != DatasetActive {
		return apperror.Conflict("DATASET_ARCHIVED", "已归档评测集内容不可修改")
	}
	return nil
}

func newDraft(datasetID id.UUID, baseVersionID *id.UUID, actorID id.UUID) Version {
	return Version{
		ID: id.MustNew(), DatasetID: datasetID, BaseVersionID: baseVersionID,
		Status: VersionDraft, CreatedBy: actorID, UpdatedBy: actorID,
	}
}

func copiedTagRecords(caseID id.UUID, tags []CaseTag, actorID id.UUID) ([]VersionCaseTag, error) {
	result := make([]VersionCaseTag, 0, len(tags))
	for _, tag := range tags {
		tagID, err := id.Parse(tag.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, VersionCaseTag{
			ID: id.MustNew(), VersionCaseID: caseID, CaseTagID: tagID,
			TagNameSnapshot: tag.Name, CreatedBy: actorID,
		})
	}
	return result, nil
}

func validateName(name string, description *string) (string, *string, error) {
	name = strings.TrimSpace(name)
	description = normalizeOptional(description)
	if name == "" || len([]rune(name)) > 200 {
		return "", nil, apperror.Validation(
			apperror.FieldError{Field: "name", Message: "名称不能为空且不能超过 200 个字符"},
		)
	}
	if description != nil && len([]rune(*description)) > 10000 {
		return "", nil, apperror.Validation(
			apperror.FieldError{Field: "description", Message: "说明不能超过 10000 个字符"},
		)
	}
	return name, description, nil
}

func validateCaseInput(input CaseInput) (CaseInput, error) {
	input.Name = normalizeOptional(input.Name)
	input.Precondition = normalizeOptional(input.Precondition)
	input.ExpectedResult = normalizeOptional(input.ExpectedResult)
	input.JudgingGuide = normalizeOptional(input.JudgingGuide)
	input.UserPrompt = strings.TrimSpace(input.UserPrompt)
	var fields []apperror.FieldError
	if input.UserPrompt == "" {
		fields = append(fields, apperror.FieldError{Field: "user_prompt", Message: "用户问题不能为空"})
	}
	if len([]rune(input.UserPrompt)) > 1000000 {
		fields = append(fields, apperror.FieldError{Field: "user_prompt", Message: "用户问题内容过长"})
	}
	if input.Name != nil && len([]rune(*input.Name)) > 200 {
		fields = append(fields, apperror.FieldError{Field: "name", Message: "用例名称不能超过 200 个字符"})
	}
	if len(fields) > 0 {
		return CaseInput{}, apperror.Validation(fields...)
	}
	seen := map[id.UUID]struct{}{}
	for _, tagID := range input.TagIDs {
		if _, exists := seen[tagID]; exists {
			return CaseInput{}, apperror.Validation(
				apperror.FieldError{Field: "tag_ids", Message: "用例标签不能重复"},
			)
		}
		seen[tagID] = struct{}{}
	}
	return input, nil
}

func validateReorder(items []ReorderItem) error {
	if len(items) == 0 {
		return apperror.Validation(apperror.FieldError{Field: "items", Message: "排序列表不能为空"})
	}
	seen := map[id.UUID]struct{}{}
	for _, item := range items {
		if _, exists := seen[item.ID]; exists {
			return apperror.Validation(apperror.FieldError{Field: "items", Message: "排序列表包含重复用例"})
		}
		seen[item.ID] = struct{}{}
	}
	return nil
}

func normalizeOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound()
	}
	return err
}

func mapWriteError(err error) error {
	if errors.Is(err, ErrLockConflict) {
		return apperror.Conflict("LOCK_VERSION_CONFLICT", "数据已被其他用户更新，请刷新后重试")
	}
	if IsDuplicate(err) {
		return apperror.Conflict("NAME_CONFLICT", "当前范围内已存在同名数据")
	}
	return err
}
