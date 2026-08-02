package evaluation

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

type RunInput struct {
	DatasetVersionID    id.UUID
	AgentVersion        string
	Environment         string
	PurposeNote         *string
	ConfigNote          *string
	ExpectedLockVersion uint32
}

func (service Service) CreateRun(
	ctx context.Context,
	actorID id.UUID,
	input RunInput,
) (Run, error) {
	normalized, err := validateRunInput(input)
	if err != nil {
		return Run{}, err
	}
	var run Run
	err = service.repository.Transaction(ctx, func(repository Repository) error {
		version, err := repository.GetVersionContext(ctx, normalized.DatasetVersionID)
		if err != nil {
			return mapNotFound(err)
		}
		if err := validateVersionAvailability(version); err != nil {
			return err
		}
		caseIDs, err := repository.EnabledCaseIDs(ctx, normalized.DatasetVersionID)
		if err != nil {
			return err
		}
		run = Run{
			ID: id.MustNew(), DatasetVersionID: normalized.DatasetVersionID,
			EvaluatorID: actorID, AgentVersion: normalized.AgentVersion,
			Environment: normalized.Environment, PurposeNote: normalized.PurposeNote,
			ConfigNote: normalized.ConfigNote, Status: RunInProgress, UpdatedBy: actorID,
		}
		if err := repository.CreateRun(ctx, &run); err != nil {
			return err
		}
		results := make([]Result, 0, len(caseIDs))
		for _, caseID := range caseIDs {
			results = append(results, Result{
				ID: id.MustNew(), EvaluationRunID: run.ID, VersionCaseID: caseID,
				Status: ResultPending, UpdatedBy: actorID,
			})
		}
		return repository.CreateResults(ctx, results)
	})
	if err != nil {
		return Run{}, err
	}
	return service.repository.GetRun(ctx, run.ID)
}

func validateVersionAvailability(version VersionContext) error {
	if version.TargetStatus != "active" {
		return apperror.Conflict(
			"VERSION_NOT_EVALUATABLE", "评测对象已停用，不能开始新的评测",
		)
	}
	if version.Status != "published" || version.DatasetStatus != "active" {
		return apperror.Conflict(
			"VERSION_NOT_EVALUATABLE", "只能基于活跃评测集的已发布版本开始评测",
		)
	}
	if version.EnabledCount == 0 {
		return apperror.Conflict("VERSION_HAS_NO_ENABLED_CASES", "版本没有可评测的启用用例")
	}
	return nil
}

func (service Service) UpdateRun(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	runID id.UUID,
	input RunInput,
) (Run, Run, error) {
	before, err := service.authorizedRun(ctx, actorID, admin, runID)
	if err != nil {
		return Run{}, Run{}, err
	}
	if before.Status != RunInProgress {
		return Run{}, Run{}, apperror.Conflict("RUN_NOT_EDITABLE", "只有进行中的评测可以修改")
	}
	normalized, err := validateRunInput(input)
	if err != nil {
		return Run{}, Run{}, err
	}
	if err := service.repository.UpdateRun(
		ctx, runID, actorID, normalized.AgentVersion, normalized.Environment,
		normalized.PurposeNote, normalized.ConfigNote, input.ExpectedLockVersion,
	); err != nil {
		return Run{}, Run{}, mapWriteError(err)
	}
	after, err := service.repository.GetRun(ctx, runID)
	return before, after, err
}

type ResultInput struct {
	Status              string
	AnswerText          *string
	Score               *uint8
	Comment             *string
	SkipReason          *string
	ExpectedLockVersion uint32
}

func (service Service) UpdateResult(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	resultID id.UUID,
	input ResultInput,
) (Result, Run, error) {
	normalized, err := validateResultInput(input)
	if err != nil {
		return Result{}, Run{}, err
	}
	var updated Result
	err = service.repository.Transaction(ctx, func(repository Repository) error {
		before, err := repository.GetResult(ctx, resultID)
		if err != nil {
			return mapNotFound(err)
		}
		run, err := repository.LockRun(ctx, before.EvaluationRunID)
		if err != nil {
			return mapNotFound(err)
		}
		if !admin && run.EvaluatorID != actorID {
			return apperror.Forbidden()
		}
		if run.Status != RunInProgress {
			return apperror.Conflict("RUN_NOT_EDITABLE", "评测已完成或作废，结果不能修改")
		}
		if normalized.Status == ResultEvaluated && normalized.Score != nil &&
			*normalized.Score <= 2 && !before.HasBadcase {
			return apperror.Validation(
				apperror.FieldError{
					Field:   "score",
					Message: "1～2 分必须同时标记为 Badcase",
				},
			)
		}
		if err := repository.UpdateResult(
			ctx, resultID, actorID, normalized.Status, normalized.AnswerText,
			normalized.Score, normalized.Comment, normalized.SkipReason,
			input.ExpectedLockVersion,
		); err != nil {
			return mapWriteError(err)
		}
		if err := repository.BumpRun(ctx, run.ID, actorID); err != nil {
			return err
		}
		updated, err = repository.GetResult(ctx, resultID)
		return err
	})
	if err != nil {
		return Result{}, Run{}, err
	}
	run, err := service.repository.GetRun(ctx, updated.EvaluationRunID)
	return updated, run, err
}

func (service Service) Complete(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	runID id.UUID,
	expectedVersion uint32,
) (Run, Run, error) {
	var before Run
	err := service.repository.Transaction(ctx, func(repository Repository) error {
		item, err := repository.LockRun(ctx, runID)
		if err != nil {
			return mapNotFound(err)
		}
		before = item
		if !admin && item.EvaluatorID != actorID {
			return apperror.Forbidden()
		}
		if item.Status != RunInProgress {
			return apperror.Conflict("INVALID_STATE_TRANSITION", "只有进行中的评测可以完成")
		}
		pending, err := repository.CountPending(ctx, runID)
		if err != nil {
			return err
		}
		if pending > 0 {
			conflict := apperror.Conflict("PENDING_RESULTS_EXIST", "仍有待评用例，不能完成评测")
			conflict.Details = map[string]any{"pending_count": pending}
			return conflict
		}
		return repository.CompleteRun(ctx, runID, actorID, expectedVersion)
	})
	if err != nil {
		return Run{}, Run{}, mapWriteError(err)
	}
	after, err := service.repository.GetRun(ctx, runID)
	return before, after, err
}

func (service Service) Reopen(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	runID id.UUID,
	reason string,
	expectedVersion uint32,
) (Run, Run, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Run{}, Run{}, apperror.Validation(
			apperror.FieldError{Field: "reason", Message: "请填写重开原因"},
		)
	}
	before, err := service.authorizedRun(ctx, actorID, admin, runID)
	if err != nil {
		return Run{}, Run{}, err
	}
	if before.Status != RunCompleted {
		return Run{}, Run{}, apperror.Conflict("INVALID_STATE_TRANSITION", "只有已完成评测可以重开")
	}
	if err := service.repository.ReopenRun(ctx, runID, actorID, expectedVersion); err != nil {
		return Run{}, Run{}, mapWriteError(err)
	}
	after, err := service.repository.GetRun(ctx, runID)
	return before, after, err
}

func (service Service) Void(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	runID id.UUID,
	reason string,
	expectedVersion uint32,
) (Run, Run, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Run{}, Run{}, apperror.Validation(
			apperror.FieldError{Field: "reason", Message: "请填写作废原因"},
		)
	}
	before, err := service.authorizedRun(ctx, actorID, admin, runID)
	if err != nil {
		return Run{}, Run{}, err
	}
	if before.Status == RunVoided {
		return Run{}, Run{}, apperror.Conflict("INVALID_STATE_TRANSITION", "已作废评测不能再次作废")
	}
	if err := service.repository.VoidRun(ctx, runID, actorID, reason, expectedVersion); err != nil {
		return Run{}, Run{}, mapWriteError(err)
	}
	after, err := service.repository.GetRun(ctx, runID)
	return before, after, err
}

func (service Service) Delete(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	runID id.UUID,
	expectedVersion uint32,
) error {
	return service.repository.Transaction(ctx, func(repository Repository) error {
		run, err := repository.LockRun(ctx, runID)
		if err != nil {
			return mapNotFound(err)
		}
		if !admin && run.EvaluatorID != actorID {
			return apperror.Forbidden()
		}
		if run.LockVersion != expectedVersion {
			return mapWriteError(ErrLockConflict)
		}
		if run.Status != RunInProgress || run.FirstCompletedAt != nil {
			return apperror.Conflict(
				"RUN_DELETE_FORBIDDEN",
				"只有从未完成的进行中评测可以删除",
			)
		}
		count, err := repository.CountBadcases(ctx, runID)
		if err != nil {
			return err
		}
		if count > 0 {
			return apperror.Conflict("RUN_HAS_BADCASES", "评测已产生 Badcase，只能作废")
		}
		return repository.DeleteRun(ctx, runID)
	})
}

func (service Service) AuthorizedRun(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	runID id.UUID,
) (Run, error) {
	return service.authorizedRun(ctx, actorID, admin, runID)
}

func (service Service) authorizedRun(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	runID id.UUID,
) (Run, error) {
	run, err := service.repository.GetRun(ctx, runID)
	if err != nil {
		return Run{}, mapNotFound(err)
	}
	if !admin && run.EvaluatorID != actorID {
		return Run{}, apperror.Forbidden()
	}
	return run, nil
}

func validateRunInput(input RunInput) (RunInput, error) {
	input.AgentVersion = strings.TrimSpace(input.AgentVersion)
	input.Environment = strings.TrimSpace(input.Environment)
	input.PurposeNote = cleanOptional(input.PurposeNote)
	input.ConfigNote = cleanOptional(input.ConfigNote)
	fields := []apperror.FieldError{}
	if input.AgentVersion == "" || len([]rune(input.AgentVersion)) > 100 {
		fields = append(fields, apperror.FieldError{
			Field: "agent_version", Message: "Agent 版本不能为空且最多 100 个字符",
		})
	}
	switch input.Environment {
	case "test", "staging", "production", "other":
	default:
		fields = append(fields, apperror.FieldError{
			Field: "environment", Message: "运行环境不在允许范围内",
		})
	}
	if len(fields) > 0 {
		return RunInput{}, apperror.Validation(fields...)
	}
	return input, nil
}

func validateResultInput(input ResultInput) (ResultInput, error) {
	input.AnswerText = cleanOptional(input.AnswerText)
	input.Comment = cleanOptional(input.Comment)
	input.SkipReason = cleanOptional(input.SkipReason)
	switch input.Status {
	case ResultEvaluated:
		input.SkipReason = nil
		if input.Score == nil {
			return ResultInput{}, apperror.Validation(
				apperror.FieldError{Field: "score", Message: "请为已评用例选择评分"},
			)
		}
	case ResultSkipped:
		if input.SkipReason == nil {
			return ResultInput{}, apperror.Validation(
				apperror.FieldError{Field: "skip_reason", Message: "跳过用例必须填写原因"},
			)
		}
		input.Score = nil
	case ResultPending:
		input.AnswerText = nil
		input.Score = nil
		input.Comment = nil
		input.SkipReason = nil
	default:
		return ResultInput{}, apperror.Validation(
			apperror.FieldError{Field: "status", Message: "结果状态不在允许范围内"},
		)
	}
	if input.Score != nil && (*input.Score < 1 || *input.Score > 5) {
		return ResultInput{}, apperror.Validation(
			apperror.FieldError{Field: "score", Message: "评分只能是 1～5 分"},
		)
	}
	return input, nil
}

func cleanOptional(value *string) *string {
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
		return apperror.Conflict("LOCK_VERSION_CONFLICT", "数据已被其他操作更新，请刷新后重试")
	}
	return err
}
