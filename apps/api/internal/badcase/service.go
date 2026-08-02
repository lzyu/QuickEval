package badcase

import (
	"context"
	"errors"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/evaluation"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
)

type Service struct {
	repository  Repository
	evaluations evaluation.Repository
}

func NewService(repository Repository, evaluations evaluation.Repository) Service {
	return Service{repository: repository, evaluations: evaluations}
}

type ResultPatch struct {
	Status     string
	AnswerText *string
	Score      *uint8
	Comment    *string
}

type MarkInput struct {
	ExpectedResultLockVersion uint32
	ResultPatch               *ResultPatch
	Description               *string
	IssueTagIDs               []id.UUID
}

type MarkResult struct {
	Badcase Badcase
	Result  evaluation.Result
	Run     evaluation.Run
}

func (service Service) MarkEvaluation(
	ctx context.Context,
	actorID id.UUID,
	admin bool,
	resultID id.UUID,
	input MarkInput,
) (MarkResult, error) {
	normalized, err := validateMarkInput(input)
	if err != nil {
		return MarkResult{}, err
	}
	title := problemTitle(*normalized.Description)
	var badcaseID id.UUID
	err = service.repository.Transaction(ctx, func(repository Repository) error {
		result, err := repository.LockResultContext(ctx, resultID)
		if err != nil {
			return mapNotFound(err)
		}
		if !admin && result.EvaluatorID != actorID {
			return apperror.Forbidden()
		}
		if result.RunStatus != evaluation.RunInProgress {
			return apperror.Conflict("RUN_NOT_EDITABLE", "只有进行中的评测可以标记 Badcase")
		}
		if result.ResultLock != normalized.ExpectedResultLockVersion {
			return mapWriteError(ErrLockConflict)
		}
		status, answer, score, comment := result.ResultStatus, result.AnswerText, result.Score, result.Comment
		if normalized.ResultPatch != nil {
			status = normalized.ResultPatch.Status
			answer = normalized.ResultPatch.AnswerText
			score = normalized.ResultPatch.Score
			comment = normalized.ResultPatch.Comment
		}
		if status != evaluation.ResultEvaluated {
			return apperror.Conflict("BADCASE_RESULT_INVALID", "跳过或待评用例不能标记 Badcase")
		}
		if score == nil {
			return apperror.Validation(
				apperror.FieldError{Field: "score", Message: "标记 Badcase 时必须选择评分"},
			)
		}
		if clean(comment) == nil {
			return apperror.Validation(
				apperror.FieldError{Field: "comment", Message: "标记 Badcase 时评语不能为空"},
			)
		}
		activeTags, err := validateActiveTags(ctx, repository, normalized.IssueTagIDs)
		if err != nil {
			return err
		}
		existing, findErr := repository.FindByResult(ctx, resultID, true)
		restored := false
		if findErr == nil {
			if existing.InvalidatedAt == nil {
				return apperror.Conflict("BADCASE_ALREADY_EXISTS", "该用例结果已经标记为 Badcase")
			}
			if err := repository.Restore(
				ctx, existing, actorID, title, normalized.Description,
				answer, result.AgentVersion, result.Environment,
			); err != nil {
				return err
			}
			badcaseID = existing.ID
			restored = true
		} else if errors.Is(findErr, gorm.ErrRecordNotFound) {
			agentVersion := result.AgentVersion
			item := Badcase{
				ID: id.MustNew(), SourceType: "evaluation", TargetID: result.TargetID,
				ScenarioID: result.ScenarioID, AssignmentStatus: result.AssignmentStatus,
				CaseResultID: &resultID, Title: title,
				Description: normalized.Description, AgentResponseText: answer,
				AgentVersion: &agentVersion, Environment: result.Environment,
				OccurredAt: time.Now().UTC(), Status: "pending",
				CreatedBy: actorID, UpdatedBy: actorID,
			}
			if err := repository.Create(ctx, &item); err != nil {
				return mapDuplicate(err)
			}
			badcaseID = item.ID
		} else {
			return findErr
		}
		if err := repository.ReplaceTags(ctx, badcaseID, actorID, activeTags); err != nil {
			return err
		}
		activityType := "created"
		var note *string
		if restored {
			activityType = "reactivated"
			value := "重新标记为评测 Badcase"
			note = &value
		}
		if err := repository.CreateActivity(ctx, &Activity{
			ID: id.MustNew(), BadcaseID: badcaseID, Type: activityType,
			Note: note, ActorID: actorID,
		}); err != nil {
			return err
		}
		if normalized.ResultPatch != nil {
			if err := repository.UpdateResult(
				ctx, result, actorID, status, clean(answer), score, clean(comment),
				normalized.ExpectedResultLockVersion,
			); err != nil {
				return mapWriteError(err)
			}
		} else if err := repository.BumpResult(
			ctx, resultID, actorID, normalized.ExpectedResultLockVersion,
		); err != nil {
			return mapWriteError(err)
		}
		return repository.BumpRun(ctx, result.RunID, actorID)
	})
	if err != nil {
		return MarkResult{}, err
	}
	return service.LoadMarkResult(ctx, badcaseID)
}

func (service Service) LoadMarkResult(ctx context.Context, badcaseID id.UUID) (MarkResult, error) {
	item, err := service.repository.Get(ctx, badcaseID)
	if err != nil {
		return MarkResult{}, mapNotFound(err)
	}
	if item.CaseResultID == nil {
		return MarkResult{}, apperror.NotFound()
	}
	result, err := service.evaluations.GetResult(ctx, *item.CaseResultID)
	if err != nil {
		return MarkResult{}, err
	}
	run, err := service.evaluations.GetRun(ctx, result.EvaluationRunID)
	return MarkResult{Badcase: item, Result: result, Run: run}, err
}

func validateMarkInput(input MarkInput) (MarkInput, error) {
	input.Description = clean(input.Description)
	fields := []apperror.FieldError{}
	if input.Description == nil {
		fields = append(fields, apperror.FieldError{
			Field: "description", Message: "请填写问题描述",
		})
	}
	unique := make(map[id.UUID]bool, len(input.IssueTagIDs))
	deduplicated := make([]id.UUID, 0, len(input.IssueTagIDs))
	for _, tagID := range input.IssueTagIDs {
		if !unique[tagID] {
			unique[tagID] = true
			deduplicated = append(deduplicated, tagID)
		}
	}
	input.IssueTagIDs = deduplicated
	if input.ResultPatch != nil {
		input.ResultPatch.Status = strings.TrimSpace(input.ResultPatch.Status)
		input.ResultPatch.AnswerText = clean(input.ResultPatch.AnswerText)
		input.ResultPatch.Comment = clean(input.ResultPatch.Comment)
		if input.ResultPatch.Status != evaluation.ResultEvaluated {
			fields = append(fields, apperror.FieldError{
				Field: "result_patch.status", Message: "标记 Badcase 时结果必须为已评",
			})
		}
		if input.ResultPatch.Score == nil {
			fields = append(fields, apperror.FieldError{
				Field: "result_patch.score", Message: "标记 Badcase 时必须选择评分",
			})
		} else if *input.ResultPatch.Score < 1 || *input.ResultPatch.Score > 5 {
			fields = append(fields, apperror.FieldError{
				Field: "result_patch.score", Message: "评分只能是 1～5 分",
			})
		}
	}
	if len(fields) > 0 {
		return MarkInput{}, apperror.Validation(fields...)
	}
	return input, nil
}

func problemTitle(description string) string {
	normalized := strings.Join(strings.Fields(description), " ")
	runes := []rune(normalized)
	if len(runes) <= 200 {
		return normalized
	}
	return string(runes[:197]) + "..."
}

func clean(value *string) *string {
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
		return apperror.Conflict("LOCK_VERSION_CONFLICT", "数据已变化，请刷新后重试")
	}
	return err
}

func mapDuplicate(err error) error {
	var mysqlError *mysqlDriver.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return apperror.Conflict("BADCASE_ALREADY_EXISTS", "该用例结果已经标记为 Badcase")
	}
	return err
}
