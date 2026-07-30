package catalog

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

func NewService(repository Repository) Service {
	return Service{repository: repository}
}

type NamedInput struct {
	Name                string
	Description         *string
	ExpectedLockVersion uint32
}

type CaseTagInput struct {
	Scope               string
	ScenarioID          *id.UUID
	Name                string
	Description         *string
	ExpectedLockVersion uint32
}

func (service Service) CreateTarget(
	ctx context.Context,
	actorID id.UUID,
	input NamedInput,
) (EvaluationTarget, error) {
	name, description, err := validateNamed(input.Name, input.Description, 200)
	if err != nil {
		return EvaluationTarget{}, err
	}
	item := EvaluationTarget{
		ID:          id.MustNew(),
		Name:        name,
		Description: description,
		Status:      StatusActive,
		CreatedBy:   actorID,
		UpdatedBy:   actorID,
	}
	if err := service.repository.CreateTarget(ctx, &item); err != nil {
		return EvaluationTarget{}, mapWriteError(err)
	}
	return service.repository.GetTarget(ctx, item.ID)
}

func (service Service) UpdateTarget(
	ctx context.Context,
	actorID, targetID id.UUID,
	input NamedInput,
) (EvaluationTarget, EvaluationTarget, error) {
	before, err := service.repository.GetTarget(ctx, targetID)
	if err != nil {
		return EvaluationTarget{}, EvaluationTarget{}, mapNotFound(err)
	}
	name, description, err := validateNamed(input.Name, input.Description, 200)
	if err != nil {
		return EvaluationTarget{}, EvaluationTarget{}, err
	}
	if err := service.repository.UpdateTarget(
		ctx,
		targetID,
		actorID,
		name,
		description,
		input.ExpectedLockVersion,
	); err != nil {
		return EvaluationTarget{}, EvaluationTarget{}, mapWriteError(err)
	}
	after, err := service.repository.GetTarget(ctx, targetID)
	return before, after, err
}

func (service Service) SetTargetStatus(
	ctx context.Context,
	actorID, targetID id.UUID,
	status string,
	expectedVersion uint32,
) (EvaluationTarget, EvaluationTarget, error) {
	before, err := service.repository.GetTarget(ctx, targetID)
	if err != nil {
		return EvaluationTarget{}, EvaluationTarget{}, mapNotFound(err)
	}
	if err := validateTransition(before.Status, status); err != nil {
		return EvaluationTarget{}, EvaluationTarget{}, err
	}
	if err := service.repository.SetTargetStatus(
		ctx,
		targetID,
		actorID,
		status,
		expectedVersion,
	); err != nil {
		return EvaluationTarget{}, EvaluationTarget{}, mapWriteError(err)
	}
	after, err := service.repository.GetTarget(ctx, targetID)
	return before, after, err
}

type ScenarioInput struct {
	EvaluationTargetID  id.UUID
	Name                string
	Description         *string
	ExpectedLockVersion uint32
}

func (service Service) CreateScenario(
	ctx context.Context,
	actorID id.UUID,
	input ScenarioInput,
) (Scenario, error) {
	target, err := service.repository.GetTarget(ctx, input.EvaluationTargetID)
	if err != nil {
		return Scenario{}, mapNotFound(err)
	}
	if target.Status != StatusActive {
		return Scenario{}, apperror.Conflict("RESOURCE_DISABLED", "停用的评测对象不能新建场景")
	}
	name, description, err := validateNamed(input.Name, input.Description, 200)
	if err != nil {
		return Scenario{}, err
	}
	item := Scenario{
		ID:                 id.MustNew(),
		EvaluationTargetID: input.EvaluationTargetID,
		Name:               name,
		Description:        description,
		Status:             StatusActive,
		CreatedBy:          actorID,
		UpdatedBy:          actorID,
	}
	if err := service.repository.CreateScenario(ctx, &item); err != nil {
		return Scenario{}, mapWriteError(err)
	}
	return service.repository.GetScenario(ctx, item.ID)
}

func (service Service) UpdateScenario(
	ctx context.Context,
	actorID, scenarioID id.UUID,
	input ScenarioInput,
) (Scenario, Scenario, error) {
	before, err := service.repository.GetScenario(ctx, scenarioID)
	if err != nil {
		return Scenario{}, Scenario{}, mapNotFound(err)
	}
	target, err := service.repository.GetTarget(ctx, input.EvaluationTargetID)
	if err != nil {
		return Scenario{}, Scenario{}, mapNotFound(err)
	}
	if target.Status != StatusActive && target.ID != before.EvaluationTargetID {
		return Scenario{}, Scenario{}, apperror.Conflict("RESOURCE_DISABLED", "不能移动到停用的评测对象")
	}
	if before.EvaluationTargetID != input.EvaluationTargetID {
		hasHistory, err := service.repository.ScenarioHasHistory(ctx, scenarioID)
		if err != nil {
			return Scenario{}, Scenario{}, err
		}
		if hasHistory {
			return Scenario{}, Scenario{}, apperror.Conflict(
				"RELATIONSHIP_LOCKED",
				"场景已有历史数据，不能修改所属评测对象",
			)
		}
	}
	name, description, err := validateNamed(input.Name, input.Description, 200)
	if err != nil {
		return Scenario{}, Scenario{}, err
	}
	if err := service.repository.UpdateScenario(
		ctx,
		scenarioID,
		actorID,
		input.EvaluationTargetID,
		name,
		description,
		input.ExpectedLockVersion,
	); err != nil {
		return Scenario{}, Scenario{}, mapWriteError(err)
	}
	after, err := service.repository.GetScenario(ctx, scenarioID)
	return before, after, err
}

func (service Service) SetScenarioStatus(
	ctx context.Context,
	actorID, scenarioID id.UUID,
	status string,
	expectedVersion uint32,
) (Scenario, Scenario, error) {
	before, err := service.repository.GetScenario(ctx, scenarioID)
	if err != nil {
		return Scenario{}, Scenario{}, mapNotFound(err)
	}
	if err := validateTransition(before.Status, status); err != nil {
		return Scenario{}, Scenario{}, err
	}
	if err := service.repository.SetScenarioStatus(
		ctx,
		scenarioID,
		actorID,
		status,
		expectedVersion,
	); err != nil {
		return Scenario{}, Scenario{}, mapWriteError(err)
	}
	after, err := service.repository.GetScenario(ctx, scenarioID)
	return before, after, err
}

func (service Service) CreateCaseTag(
	ctx context.Context,
	actorID id.UUID,
	input CaseTagInput,
) (CaseTag, error) {
	if err := validateCaseTagScope(input.Scope, input.ScenarioID); err != nil {
		return CaseTag{}, err
	}
	if input.Scope == CaseTagScopeScenario {
		scenario, err := service.repository.GetScenario(ctx, *input.ScenarioID)
		if err != nil {
			return CaseTag{}, mapNotFound(err)
		}
		if scenario.Status != StatusActive {
			return CaseTag{}, apperror.Conflict("RESOURCE_DISABLED", "停用的场景不能新建用例标签")
		}
	}
	name, description, err := validateNamed(input.Name, input.Description, 100)
	if err != nil {
		return CaseTag{}, err
	}
	conflicts, err := service.repository.CaseTagNameConflicts(
		ctx,
		input.Scope,
		input.ScenarioID,
		name,
		nil,
	)
	if err != nil {
		return CaseTag{}, err
	}
	if conflicts {
		return CaseTag{}, apperror.Conflict(
			"NAME_CONFLICT",
			"全局标签与同一场景内的标签不能重名",
		)
	}
	items, err := service.repository.ListCaseTags(ctx, input.Scope, input.ScenarioID, "")
	if err != nil {
		return CaseTag{}, err
	}
	item := CaseTag{
		ID:          id.MustNew(),
		Scope:       input.Scope,
		ScenarioID:  input.ScenarioID,
		Name:        name,
		Description: description,
		Status:      StatusActive,
		SortOrder:   uint32(len(items)+1) * 10,
		CreatedBy:   actorID,
		UpdatedBy:   actorID,
	}
	if err := service.repository.CreateCaseTag(ctx, &item); err != nil {
		return CaseTag{}, mapWriteError(err)
	}
	return service.repository.GetCaseTag(ctx, item.ID)
}

func (service Service) UpdateCaseTag(
	ctx context.Context,
	actorID, tagID id.UUID,
	input NamedInput,
) (CaseTag, CaseTag, error) {
	before, err := service.repository.GetCaseTag(ctx, tagID)
	if err != nil {
		return CaseTag{}, CaseTag{}, mapNotFound(err)
	}
	name, description, err := validateNamed(input.Name, input.Description, 100)
	if err != nil {
		return CaseTag{}, CaseTag{}, err
	}
	conflicts, err := service.repository.CaseTagNameConflicts(
		ctx,
		before.Scope,
		before.ScenarioID,
		name,
		&tagID,
	)
	if err != nil {
		return CaseTag{}, CaseTag{}, err
	}
	if conflicts {
		return CaseTag{}, CaseTag{}, apperror.Conflict(
			"NAME_CONFLICT",
			"全局标签与同一场景内的标签不能重名",
		)
	}
	if err := service.repository.UpdateCaseTag(
		ctx,
		tagID,
		actorID,
		name,
		description,
		input.ExpectedLockVersion,
	); err != nil {
		return CaseTag{}, CaseTag{}, mapWriteError(err)
	}
	after, err := service.repository.GetCaseTag(ctx, tagID)
	return before, after, err
}

func (service Service) SetCaseTagStatus(
	ctx context.Context,
	actorID, tagID id.UUID,
	status string,
	expectedVersion uint32,
) (CaseTag, CaseTag, error) {
	before, err := service.repository.GetCaseTag(ctx, tagID)
	if err != nil {
		return CaseTag{}, CaseTag{}, mapNotFound(err)
	}
	if err := validateTransition(before.Status, status); err != nil {
		return CaseTag{}, CaseTag{}, err
	}
	if err := service.repository.SetCaseTagStatus(
		ctx,
		tagID,
		actorID,
		status,
		expectedVersion,
	); err != nil {
		return CaseTag{}, CaseTag{}, mapWriteError(err)
	}
	after, err := service.repository.GetCaseTag(ctx, tagID)
	return before, after, err
}

func (service Service) ReorderCaseTags(
	ctx context.Context,
	actorID, scenarioID id.UUID,
	items []ReorderItem,
) error {
	if err := validateReorder(items); err != nil {
		return err
	}
	return mapWriteError(service.repository.ReorderCaseTags(ctx, scenarioID, actorID, items))
}

func validateCaseTagScope(scope string, scenarioID *id.UUID) error {
	switch scope {
	case CaseTagScopeGlobal:
		if scenarioID != nil {
			return apperror.Validation(
				apperror.FieldError{Field: "scenario_id", Message: "全局标签不能绑定场景"},
			)
		}
	case CaseTagScopeScenario:
		if scenarioID == nil {
			return apperror.Validation(
				apperror.FieldError{Field: "scenario_id", Message: "场景标签必须选择所属场景"},
			)
		}
	default:
		return apperror.Validation(
			apperror.FieldError{Field: "scope", Message: "标签作用域必须是 global 或 scenario"},
		)
	}
	return nil
}

func (service Service) CreateIssueTag(
	ctx context.Context,
	actorID id.UUID,
	input NamedInput,
) (IssueTag, error) {
	name, description, err := validateNamed(input.Name, input.Description, 100)
	if err != nil {
		return IssueTag{}, err
	}
	items, err := service.repository.ListIssueTags(ctx, "", "")
	if err != nil {
		return IssueTag{}, err
	}
	item := IssueTag{
		ID:          id.MustNew(),
		Name:        name,
		Description: description,
		Status:      StatusActive,
		SortOrder:   uint32(len(items)+1) * 10,
		CreatedBy:   actorID,
		UpdatedBy:   actorID,
	}
	if err := service.repository.CreateIssueTag(ctx, &item); err != nil {
		return IssueTag{}, mapWriteError(err)
	}
	return service.repository.GetIssueTag(ctx, item.ID)
}

func (service Service) UpdateIssueTag(
	ctx context.Context,
	actorID, tagID id.UUID,
	input NamedInput,
) (IssueTag, IssueTag, error) {
	before, err := service.repository.GetIssueTag(ctx, tagID)
	if err != nil {
		return IssueTag{}, IssueTag{}, mapNotFound(err)
	}
	name, description, err := validateNamed(input.Name, input.Description, 100)
	if err != nil {
		return IssueTag{}, IssueTag{}, err
	}
	if err := service.repository.UpdateIssueTag(
		ctx,
		tagID,
		actorID,
		name,
		description,
		input.ExpectedLockVersion,
	); err != nil {
		return IssueTag{}, IssueTag{}, mapWriteError(err)
	}
	after, err := service.repository.GetIssueTag(ctx, tagID)
	return before, after, err
}

func (service Service) SetIssueTagStatus(
	ctx context.Context,
	actorID, tagID id.UUID,
	status string,
	expectedVersion uint32,
) (IssueTag, IssueTag, error) {
	before, err := service.repository.GetIssueTag(ctx, tagID)
	if err != nil {
		return IssueTag{}, IssueTag{}, mapNotFound(err)
	}
	if err := validateTransition(before.Status, status); err != nil {
		return IssueTag{}, IssueTag{}, err
	}
	if err := service.repository.SetIssueTagStatus(
		ctx,
		tagID,
		actorID,
		status,
		expectedVersion,
	); err != nil {
		return IssueTag{}, IssueTag{}, mapWriteError(err)
	}
	after, err := service.repository.GetIssueTag(ctx, tagID)
	return before, after, err
}

func (service Service) ReorderIssueTags(
	ctx context.Context,
	actorID id.UUID,
	items []ReorderItem,
) error {
	if err := validateReorder(items); err != nil {
		return err
	}
	return mapWriteError(service.repository.ReorderIssueTags(ctx, actorID, items))
}

func validateNamed(name string, description *string, maxNameLength int) (string, *string, error) {
	name = strings.TrimSpace(name)
	description = normalizeDescription(description)
	if name == "" || len([]rune(name)) > maxNameLength {
		return "", nil, apperror.Validation(
			apperror.FieldError{Field: "name", Message: "名称不能为空或超过长度限制"},
		)
	}
	return name, description, nil
}

func normalizeDescription(description *string) *string {
	if description == nil {
		return nil
	}
	value := strings.TrimSpace(*description)
	if value == "" {
		return nil
	}
	return &value
}

func validateTransition(from, to string) error {
	if to != StatusActive && to != StatusDisabled {
		return apperror.Validation()
	}
	if from == to {
		return apperror.Conflict("INVALID_STATE_TRANSITION", "资源已经处于目标状态")
	}
	return nil
}

func validateReorder(items []ReorderItem) error {
	if len(items) == 0 {
		return apperror.Validation(
			apperror.FieldError{Field: "items", Message: "排序列表不能为空"},
		)
	}
	seen := make(map[id.UUID]struct{}, len(items))
	for _, item := range items {
		if _, exists := seen[item.ID]; exists {
			return apperror.Validation(
				apperror.FieldError{Field: "items", Message: "排序列表包含重复标签"},
			)
		}
		seen[item.ID] = struct{}{}
	}
	return nil
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound()
	}
	return err
}

func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrLockConflict) {
		return apperror.Conflict("LOCK_VERSION_CONFLICT", "数据已被其他用户更新，请刷新后重试")
	}
	if IsDuplicate(err) {
		return apperror.Conflict("NAME_CONFLICT", "同一范围内已存在同名记录")
	}
	return err
}
