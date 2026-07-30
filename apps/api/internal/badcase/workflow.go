package badcase

import (
	"context"
	"strings"
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusResolved   = "resolved"
	StatusDeferred   = "deferred"
)

type BusinessInput struct {
	ScenarioID          id.UUID
	Title               string
	Description         *string
	AgentResponseText   *string
	AgentVersion        *string
	Environment         string
	OccurredAt          time.Time
	BusinessReference   *string
	SessionID           *string
	IssueTagIDs         []id.UUID
	ExpectedLockVersion uint32
}

func (service Service) CreateBusiness(
	ctx context.Context,
	actorID id.UUID,
	input BusinessInput,
) (Badcase, error) {
	normalized, err := validateBusinessInput(input, true)
	if err != nil {
		return Badcase{}, err
	}
	active, err := service.repository.ScenarioActive(ctx, normalized.ScenarioID)
	if err != nil {
		return Badcase{}, err
	}
	if !active {
		return Badcase{}, apperror.Conflict(
			"RESOURCE_DISABLED", "只能在启用的对象和场景下登记 Badcase",
		)
	}
	var badcaseID id.UUID
	err = service.repository.Transaction(ctx, func(repository Repository) error {
		tags, err := validateActiveTags(ctx, repository, normalized.IssueTagIDs)
		if err != nil {
			return err
		}
		item := Badcase{
			ID: id.MustNew(), SourceType: "business", ScenarioID: normalized.ScenarioID,
			Title: normalized.Title, Description: normalized.Description,
			AgentResponseText: normalized.AgentResponseText, AgentVersion: normalized.AgentVersion,
			Environment: normalized.Environment, OccurredAt: normalized.OccurredAt,
			BusinessReference: normalized.BusinessReference, SessionID: normalized.SessionID,
			Status: StatusPending, CreatedBy: actorID, UpdatedBy: actorID,
		}
		if err := repository.Create(ctx, &item); err != nil {
			return err
		}
		badcaseID = item.ID
		if err := repository.ReplaceTags(ctx, item.ID, actorID, tags); err != nil {
			return err
		}
		return repository.CreateActivity(ctx, &Activity{
			ID: id.MustNew(), BadcaseID: item.ID, Type: "created", ActorID: actorID,
		})
	})
	if err != nil {
		return Badcase{}, err
	}
	return service.repository.Get(ctx, badcaseID)
}

func (service Service) UpdateBusiness(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	badcaseID id.UUID,
	input BusinessInput,
) (Badcase, error) {
	normalized, err := validateBusinessInput(input, false)
	if err != nil {
		return Badcase{}, err
	}
	err = service.repository.Transaction(ctx, func(repository Repository) error {
		item, err := repository.LockBadcase(ctx, badcaseID)
		if err != nil {
			return mapNotFound(err)
		}
		if item.SourceType != "business" {
			return apperror.Conflict(
				"BADCASE_SOURCE_IMMUTABLE", "评测来源 Badcase 的原始内容不能在此修改",
			)
		}
		if err := requireActive(item); err != nil {
			return err
		}
		if !admin && item.CreatedBy != actorID {
			return apperror.Forbidden()
		}
		if item.LockVersion != normalized.ExpectedLockVersion {
			return mapWriteError(ErrLockConflict)
		}
		return mapWriteError(repository.UpdateBusiness(ctx, item, actorID, normalized))
	})
	if err != nil {
		return Badcase{}, err
	}
	return service.repository.Get(ctx, badcaseID)
}

func (service Service) UpdateIssueTags(
	ctx context.Context,
	actorID id.UUID,
	badcaseID id.UUID,
	expectedVersion uint32,
	tagIDs []id.UUID,
) (Badcase, error) {
	tagIDs = deduplicateIDs(tagIDs)
	if len(tagIDs) == 0 {
		return Badcase{}, apperror.Validation(
			apperror.FieldError{Field: "issue_tag_ids", Message: "请至少选择一个问题标签"},
		)
	}
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		item, err := repository.LockBadcase(ctx, badcaseID)
		if err != nil {
			return mapNotFound(err)
		}
		if err := requireActive(item); err != nil {
			return err
		}
		if item.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		tags, err := validateActiveTags(ctx, repository, tagIDs)
		if err != nil {
			return err
		}
		if err := repository.ReplaceTags(ctx, badcaseID, actorID, tags); err != nil {
			return err
		}
		return mapWriteError(repository.BumpBadcase(ctx, badcaseID, actorID, expectedVersion))
	})
	if err != nil {
		return Badcase{}, err
	}
	return service.repository.Get(ctx, badcaseID)
}

func (service Service) Assign(
	ctx context.Context,
	actorID id.UUID,
	badcaseID id.UUID,
	expectedVersion uint32,
	assigneeID *id.UUID,
) (Badcase, error) {
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		item, err := repository.LockBadcase(ctx, badcaseID)
		if err != nil {
			return mapNotFound(err)
		}
		if err := requireActive(item); err != nil {
			return err
		}
		if item.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		if sameOptionalID(item.AssigneeID, assigneeID) {
			return apperror.Conflict("ASSIGNEE_UNCHANGED", "负责人没有变化")
		}
		if assigneeID != nil {
			active, err := repository.UserActive(ctx, *assigneeID)
			if err != nil {
				return err
			}
			if !active {
				return apperror.Conflict("RESOURCE_DISABLED", "负责人不存在或已停用")
			}
		}
		if err := repository.UpdateAssignee(
			ctx, item, actorID, assigneeID, expectedVersion,
		); err != nil {
			return mapWriteError(err)
		}
		return repository.CreateActivity(ctx, &Activity{
			ID: id.MustNew(), BadcaseID: badcaseID, Type: "assignee_changed",
			FromAssigneeID: item.AssigneeID, ToAssigneeID: assigneeID, ActorID: actorID,
		})
	})
	if err != nil {
		return Badcase{}, err
	}
	return service.repository.Get(ctx, badcaseID)
}

func (service Service) Transition(
	ctx context.Context,
	actorID id.UUID,
	badcaseID id.UUID,
	expectedVersion uint32,
	targetStatus, reason string,
) (Badcase, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Badcase{}, apperror.Validation(
			apperror.FieldError{Field: "reason", Message: "请填写状态变化说明"},
		)
	}
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		item, err := repository.LockBadcase(ctx, badcaseID)
		if err != nil {
			return mapNotFound(err)
		}
		if err := requireActive(item); err != nil {
			return err
		}
		if item.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		if !validTransition(item.Status, targetStatus) {
			return apperror.Conflict(
				"INVALID_STATE_TRANSITION", "当前状态不允许执行该操作",
			)
		}
		if err := repository.UpdateStatus(
			ctx, item, actorID, targetStatus, expectedVersion,
		); err != nil {
			return mapWriteError(err)
		}
		from, to := item.Status, targetStatus
		return repository.CreateActivity(ctx, &Activity{
			ID: id.MustNew(), BadcaseID: badcaseID, Type: "status_changed",
			FromStatus: &from, ToStatus: &to, Note: &reason, ActorID: actorID,
		})
	})
	if err != nil {
		return Badcase{}, err
	}
	return service.repository.Get(ctx, badcaseID)
}

func (service Service) AddNote(
	ctx context.Context,
	actorID id.UUID,
	badcaseID id.UUID,
	expectedVersion uint32,
	note string,
) (Badcase, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return Badcase{}, apperror.Validation(
			apperror.FieldError{Field: "note", Message: "处理备注不能为空"},
		)
	}
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		item, err := repository.LockBadcase(ctx, badcaseID)
		if err != nil {
			return mapNotFound(err)
		}
		if err := requireActive(item); err != nil {
			return err
		}
		if item.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		if err := repository.BumpBadcase(ctx, badcaseID, actorID, expectedVersion); err != nil {
			return mapWriteError(err)
		}
		return repository.CreateActivity(ctx, &Activity{
			ID: id.MustNew(), BadcaseID: badcaseID, Type: "note_added",
			Note: &note, ActorID: actorID,
		})
	})
	if err != nil {
		return Badcase{}, err
	}
	return service.repository.Get(ctx, badcaseID)
}

func (service Service) SetValidity(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	badcaseID id.UUID,
	expectedVersion uint32,
	reactivate bool,
	reason string,
) (Badcase, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Badcase{}, apperror.Validation(
			apperror.FieldError{Field: "reason", Message: "请填写操作说明"},
		)
	}
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		item, err := repository.LockBadcase(ctx, badcaseID)
		if err != nil {
			return mapNotFound(err)
		}
		if !admin && item.CreatedBy != actorID {
			return apperror.Forbidden()
		}
		if item.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		if reactivate {
			if item.InvalidatedAt == nil {
				return apperror.Conflict("INVALID_STATE_TRANSITION", "Badcase 当前有效，无需重新激活")
			}
			if err := repository.Reactivate(ctx, item, actorID, expectedVersion); err != nil {
				return mapWriteError(err)
			}
			return repository.CreateActivity(ctx, &Activity{
				ID: id.MustNew(), BadcaseID: badcaseID, Type: "reactivated",
				Note: &reason, ActorID: actorID,
			})
		}
		if item.InvalidatedAt != nil {
			return apperror.Conflict("BADCASE_INVALIDATED", "Badcase 已经无效")
		}
		if err := repository.Invalidate(ctx, item, actorID, reason, expectedVersion); err != nil {
			return mapWriteError(err)
		}
		return repository.CreateActivity(ctx, &Activity{
			ID: id.MustNew(), BadcaseID: badcaseID, Type: "invalidated",
			Note: &reason, ActorID: actorID,
		})
	})
	if err != nil {
		return Badcase{}, err
	}
	return service.repository.Get(ctx, badcaseID)
}

func (service Service) Page(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	badcaseID id.UUID,
) (PagePublic, error) {
	item, err := service.repository.Get(ctx, badcaseID)
	if err != nil {
		return PagePublic{}, mapNotFound(err)
	}
	users, err := service.repository.ActiveUsers(ctx)
	if err != nil {
		return PagePublic{}, err
	}
	tags, err := service.repository.ActiveIssueTagOptions(ctx)
	if err != nil {
		return PagePublic{}, err
	}
	return PagePublic{
		Public: item.Public(), CandidateAssignees: users,
		CandidateIssueTags: tags, AllowedActions: allowedActions(item, actorID, admin),
	}, nil
}

func validateBusinessInput(input BusinessInput, creating bool) (BusinessInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = clean(input.Description)
	input.AgentResponseText = clean(input.AgentResponseText)
	input.AgentVersion = clean(input.AgentVersion)
	input.BusinessReference = clean(input.BusinessReference)
	input.SessionID = clean(input.SessionID)
	input.Environment = strings.TrimSpace(input.Environment)
	input.IssueTagIDs = deduplicateIDs(input.IssueTagIDs)
	fields := []apperror.FieldError{}
	if input.Title == "" || len([]rune(input.Title)) > 200 {
		fields = append(fields, apperror.FieldError{
			Field: "title", Message: "标题不能为空且最多 200 个字符",
		})
	}
	if input.AgentVersion != nil && len([]rune(*input.AgentVersion)) > 100 {
		fields = append(fields, apperror.FieldError{
			Field: "agent_version", Message: "Agent 版本最多 100 个字符",
		})
	}
	if input.BusinessReference != nil && len([]rune(*input.BusinessReference)) > 200 {
		fields = append(fields, apperror.FieldError{
			Field: "business_reference", Message: "业务单号最多 200 个字符",
		})
	}
	if input.SessionID != nil && len([]rune(*input.SessionID)) > 200 {
		fields = append(fields, apperror.FieldError{
			Field: "session_id", Message: "会话 ID 最多 200 个字符",
		})
	}
	if !validEnvironment(input.Environment) {
		fields = append(fields, apperror.FieldError{
			Field: "environment", Message: "运行环境不合法",
		})
	}
	if input.OccurredAt.IsZero() {
		fields = append(fields, apperror.FieldError{
			Field: "occurred_at", Message: "请选择问题发生时间",
		})
	}
	if creating && len(input.IssueTagIDs) == 0 {
		fields = append(fields, apperror.FieldError{
			Field: "issue_tag_ids", Message: "请至少选择一个问题标签",
		})
	}
	if len(fields) > 0 {
		return BusinessInput{}, apperror.Validation(fields...)
	}
	return input, nil
}

func validateActiveTags(
	ctx context.Context,
	repository Repository,
	tagIDs []id.UUID,
) ([]id.UUID, error) {
	tags, err := repository.ActiveIssueTags(ctx, tagIDs)
	if err != nil {
		return nil, err
	}
	if len(tags) != len(tagIDs) {
		return nil, apperror.Validation(
			apperror.FieldError{Field: "issue_tag_ids", Message: "问题标签不存在或已停用"},
		)
	}
	return tags, nil
}

func validEnvironment(value string) bool {
	return value == "test" || value == "staging" || value == "production" || value == "other"
}

func requireActive(item Badcase) error {
	if item.InvalidatedAt != nil {
		return apperror.Conflict("BADCASE_INVALIDATED", "无效 Badcase 不能继续处理")
	}
	return nil
}

func validTransition(from, to string) bool {
	switch from {
	case StatusPending:
		return to == StatusProcessing || to == StatusResolved || to == StatusDeferred
	case StatusProcessing:
		return to == StatusResolved || to == StatusDeferred
	case StatusResolved, StatusDeferred:
		return to == StatusPending
	default:
		return false
	}
}

func deduplicateIDs(values []id.UUID) []id.UUID {
	seen := make(map[id.UUID]bool, len(values))
	result := make([]id.UUID, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func sameOptionalID(left, right *id.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func allowedActions(item Badcase, actorID id.UUID, admin bool) []string {
	owner := admin || item.CreatedBy == actorID
	if item.InvalidatedAt != nil {
		if owner {
			return []string{"reactivate"}
		}
		return []string{}
	}
	actions := []string{"assign", "unassign", "update_tags", "add_note", "add_attachment"}
	switch item.Status {
	case StatusPending:
		actions = append(actions, "start_processing", "resolve", "defer")
	case StatusProcessing:
		actions = append(actions, "resolve", "defer")
	case StatusResolved, StatusDeferred:
		actions = append(actions, "reopen")
	}
	if owner {
		actions = append(actions, "invalidate")
		if item.SourceType == "business" {
			actions = append(actions, "edit")
		}
	}
	return actions
}
