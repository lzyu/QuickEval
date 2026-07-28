package attachment

import (
	"context"
	"errors"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrLockConflict = errors.New("attachment owner lock conflict")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository { return Repository{db: db} }

func (repository Repository) Transaction(ctx context.Context, fn func(Repository) error) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(Repository{db: tx})
	})
}

func (repository Repository) LockResultOwner(ctx context.Context, resultID id.UUID) (Owner, error) {
	var item struct {
		ID          id.UUID
		LockVersion uint32
		Status      string
		AnswerText  *string
		RunID       id.UUID
		EvaluatorID id.UUID
		RunStatus   string
	}
	err := repository.db.WithContext(ctx).Table("case_results").
		Select(`case_results.id, case_results.lock_version, case_results.status,
			case_results.answer_text, evaluation_runs.id AS run_id,
			evaluation_runs.evaluator_id, evaluation_runs.status AS run_status`).
		Joins("JOIN evaluation_runs ON evaluation_runs.id = case_results.evaluation_run_id").
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "case_results"}}).
		Where("case_results.id = ?", resultID).Take(&item).Error
	runID := item.RunID
	evaluatorID := item.EvaluatorID
	return Owner{
		Kind: "result", ID: item.ID, LockVersion: item.LockVersion,
		Status: item.Status, AnswerText: item.AnswerText,
		RunID: &runID, EvaluatorID: &evaluatorID, CreatedBy: evaluatorID,
		Invalidated: item.RunStatus != "in_progress",
	}, err
}

func (repository Repository) LockBadcaseOwner(ctx context.Context, badcaseID id.UUID) (Owner, error) {
	var item struct {
		ID            id.UUID
		LockVersion   uint32
		Status        string
		CreatedBy     id.UUID
		InvalidatedAt any
	}
	err := repository.db.WithContext(ctx).Table("badcases").
		Select("id, lock_version, status, created_by, invalidated_at").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", badcaseID).Take(&item).Error
	return Owner{
		Kind: "badcase", ID: item.ID, LockVersion: item.LockVersion,
		Status: item.Status, CreatedBy: item.CreatedBy,
		Invalidated: item.InvalidatedAt != nil,
	}, err
}

func (repository Repository) Count(ctx context.Context, owner Owner) (int64, error) {
	var count int64
	query := repository.db.WithContext(ctx).Model(&Attachment{})
	if owner.Kind == "result" {
		query = query.Where("case_result_id = ?", owner.ID)
	} else {
		query = query.Where("badcase_id = ?", owner.ID)
	}
	err := query.Count(&count).Error
	return count, err
}

func (repository Repository) NextSortOrder(ctx context.Context, owner Owner) (uint32, error) {
	var value struct{ Maximum *uint32 }
	query := repository.db.WithContext(ctx).Model(&Attachment{}).
		Select("MAX(sort_order) AS maximum")
	if owner.Kind == "result" {
		query = query.Where("case_result_id = ?", owner.ID)
	} else {
		query = query.Where("badcase_id = ?", owner.ID)
	}
	if err := query.Scan(&value).Error; err != nil || value.Maximum == nil {
		return 10, err
	}
	return *value.Maximum + 10, nil
}

func (repository Repository) Create(ctx context.Context, items []Attachment) error {
	return repository.db.WithContext(ctx).Create(&items).Error
}

func (repository Repository) BumpOwner(
	ctx context.Context,
	owner Owner,
	actorID id.UUID,
	expectedVersion uint32,
) error {
	table := "case_results"
	if owner.Kind == "badcase" {
		table = "badcases"
	}
	result := repository.db.WithContext(ctx).Table(table).
		Where("id = ? AND lock_version = ?", owner.ID, expectedVersion).
		Updates(map[string]any{
			"updated_by":   actorID,
			"lock_version": gorm.Expr("lock_version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLockConflict
	}
	return nil
}

func (repository Repository) List(ctx context.Context, owner Owner) ([]Attachment, error) {
	var items []Attachment
	query := repository.db.WithContext(ctx)
	if owner.Kind == "result" {
		query = query.Where("case_result_id = ?", owner.ID)
	} else {
		query = query.Where("badcase_id = ?", owner.ID)
	}
	err := query.Order("sort_order ASC, created_at ASC, id ASC").Find(&items).Error
	return items, err
}

func (repository Repository) Get(ctx context.Context, attachmentID id.UUID) (Attachment, error) {
	var item Attachment
	err := repository.db.WithContext(ctx).Where("id = ?", attachmentID).Take(&item).Error
	return item, err
}

func (repository Repository) Delete(ctx context.Context, attachmentID id.UUID) error {
	result := repository.db.WithContext(ctx).Where("id = ?", attachmentID).Delete(&Attachment{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

type ReorderItem struct {
	ID        id.UUID
	SortOrder uint32
}

func (repository Repository) Reorder(
	ctx context.Context,
	owner Owner,
	items []ReorderItem,
) error {
	current, err := repository.List(ctx, owner)
	if err != nil {
		return err
	}
	if len(current) != len(items) {
		return gorm.ErrRecordNotFound
	}
	owned := make(map[id.UUID]bool, len(current))
	for _, item := range current {
		owned[item.ID] = true
	}
	for _, item := range items {
		if !owned[item.ID] {
			return gorm.ErrRecordNotFound
		}
		if err := repository.db.WithContext(ctx).Model(&Attachment{}).
			Where("id = ?", item.ID).Update("sort_order", item.SortOrder).Error; err != nil {
			return err
		}
	}
	return nil
}

func (repository Repository) ResultContentVisible(
	ctx context.Context,
	resultID, actorID id.UUID,
	admin bool,
) (bool, error) {
	if admin {
		return true, nil
	}
	var count int64
	err := repository.db.WithContext(ctx).Table("case_results").
		Joins("JOIN evaluation_runs ON evaluation_runs.id = case_results.evaluation_run_id").
		Where(`case_results.id = ? AND (
			evaluation_runs.evaluator_id = ? OR EXISTS (
				SELECT 1 FROM badcases
				WHERE badcases.case_result_id = case_results.id
				AND badcases.invalidated_at IS NULL
			)
		)`, resultID, actorID).Count(&count).Error
	return count > 0, err
}

func (repository Repository) BadcaseExists(ctx context.Context, badcaseID id.UUID) (bool, error) {
	var count int64
	err := repository.db.WithContext(ctx).Table("badcases").
		Where("id = ?", badcaseID).Count(&count).Error
	return count > 0, err
}
