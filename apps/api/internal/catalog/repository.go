package catalog

import (
	"context"
	"errors"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
)

var ErrLockConflict = errors.New("catalog lock version conflict")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return Repository{db: db}
}

func (repository Repository) ListTargets(
	ctx context.Context,
	page, pageSize int,
	status, keyword string,
) ([]EvaluationTarget, int64, error) {
	query := repository.db.WithContext(ctx).Model(&EvaluationTarget{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []EvaluationTarget
	err := query.Order("updated_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

func (repository Repository) GetTarget(ctx context.Context, targetID id.UUID) (EvaluationTarget, error) {
	var item EvaluationTarget
	err := repository.db.WithContext(ctx).Take(&item, "id = ?", targetID).Error
	return item, err
}

func (repository Repository) CreateTarget(ctx context.Context, item *EvaluationTarget) error {
	return repository.db.WithContext(ctx).Create(item).Error
}

func (repository Repository) UpdateTarget(
	ctx context.Context,
	targetID, actorID id.UUID,
	name string,
	description *string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "evaluation_targets", targetID, expectedVersion, map[string]any{
		"name":         name,
		"description":  description,
		"updated_by":   actorID,
		"lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) SetTargetStatus(
	ctx context.Context,
	targetID, actorID id.UUID,
	status string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "evaluation_targets", targetID, expectedVersion, map[string]any{
		"status":       status,
		"updated_by":   actorID,
		"lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) ListScenarios(
	ctx context.Context,
	page, pageSize int,
	targetID *id.UUID,
	status, keyword string,
) ([]Scenario, int64, error) {
	base := repository.db.WithContext(ctx).
		Table("scenarios").
		Joins("JOIN evaluation_targets ON evaluation_targets.id = scenarios.evaluation_target_id")
	query := base.Select("scenarios.*, evaluation_targets.name AS target_name")
	countQuery := base
	if targetID != nil {
		query = query.Where("scenarios.evaluation_target_id = ?", *targetID)
		countQuery = countQuery.Where("scenarios.evaluation_target_id = ?", *targetID)
	}
	if status != "" {
		query = query.Where("scenarios.status = ?", status)
		countQuery = countQuery.Where("scenarios.status = ?", status)
	}
	if keyword != "" {
		query = query.Where("scenarios.name LIKE ?", "%"+keyword+"%")
		countQuery = countQuery.Where("scenarios.name LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Scenario
	err := query.Order("scenarios.updated_at DESC, scenarios.id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error
	return items, total, err
}

func (repository Repository) GetScenario(ctx context.Context, scenarioID id.UUID) (Scenario, error) {
	var item Scenario
	err := repository.db.WithContext(ctx).
		Table("scenarios").
		Select("scenarios.*, evaluation_targets.name AS target_name").
		Joins("JOIN evaluation_targets ON evaluation_targets.id = scenarios.evaluation_target_id").
		Where("scenarios.id = ?", scenarioID).
		Take(&item).Error
	return item, err
}

func (repository Repository) CreateScenario(ctx context.Context, item *Scenario) error {
	return repository.db.WithContext(ctx).Create(item).Error
}

func (repository Repository) UpdateScenario(
	ctx context.Context,
	scenarioID, actorID, targetID id.UUID,
	name string,
	description *string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "scenarios", scenarioID, expectedVersion, map[string]any{
		"evaluation_target_id": targetID,
		"name":                 name,
		"description":          description,
		"updated_by":           actorID,
		"lock_version":         gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) SetScenarioStatus(
	ctx context.Context,
	scenarioID, actorID id.UUID,
	status string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "scenarios", scenarioID, expectedVersion, map[string]any{
		"status":       status,
		"updated_by":   actorID,
		"lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) ScenarioHasHistory(ctx context.Context, scenarioID id.UUID) (bool, error) {
	var count int64
	err := repository.db.WithContext(ctx).Raw(
		"SELECT (SELECT COUNT(*) FROM datasets WHERE scenario_id = ?) + "+
			"(SELECT COUNT(*) FROM badcases WHERE scenario_id = ?)",
		scenarioID,
		scenarioID,
	).Scan(&count).Error
	return count > 0, err
}

func (repository Repository) ListCaseTags(
	ctx context.Context,
	scenarioID id.UUID,
	status string,
) ([]CaseTag, error) {
	query := repository.db.WithContext(ctx).Where("scenario_id = ?", scenarioID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var items []CaseTag
	err := query.Order("sort_order ASC, created_at ASC, id ASC").Find(&items).Error
	return items, err
}

func (repository Repository) GetCaseTag(ctx context.Context, tagID id.UUID) (CaseTag, error) {
	var item CaseTag
	err := repository.db.WithContext(ctx).Take(&item, "id = ?", tagID).Error
	return item, err
}

func (repository Repository) CreateCaseTag(ctx context.Context, item *CaseTag) error {
	return repository.db.WithContext(ctx).Create(item).Error
}

func (repository Repository) UpdateCaseTag(
	ctx context.Context,
	tagID, actorID id.UUID,
	name string,
	description *string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "case_tags", tagID, expectedVersion, map[string]any{
		"name":         name,
		"description":  description,
		"updated_by":   actorID,
		"lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) SetCaseTagStatus(
	ctx context.Context,
	tagID, actorID id.UUID,
	status string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "case_tags", tagID, expectedVersion, map[string]any{
		"status":       status,
		"updated_by":   actorID,
		"lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) ReorderCaseTags(
	ctx context.Context,
	scenarioID, actorID id.UUID,
	items []ReorderItem,
) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			result := tx.Table("case_tags").
				Where(
					"id = ? AND scenario_id = ? AND lock_version = ?",
					item.ID,
					scenarioID,
					item.ExpectedLockVersion,
				).
				Updates(map[string]any{
					"sort_order":   item.SortOrder,
					"updated_by":   actorID,
					"lock_version": gorm.Expr("lock_version + 1"),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrLockConflict
			}
		}
		return nil
	})
}

func (repository Repository) ListIssueTags(
	ctx context.Context,
	status, keyword string,
) ([]IssueTag, error) {
	query := repository.db.WithContext(ctx).Model(&IssueTag{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	var items []IssueTag
	err := query.Order("sort_order ASC, created_at ASC, id ASC").Find(&items).Error
	return items, err
}

func (repository Repository) GetIssueTag(ctx context.Context, tagID id.UUID) (IssueTag, error) {
	var item IssueTag
	err := repository.db.WithContext(ctx).Take(&item, "id = ?", tagID).Error
	return item, err
}

func (repository Repository) CreateIssueTag(ctx context.Context, item *IssueTag) error {
	return repository.db.WithContext(ctx).Create(item).Error
}

func (repository Repository) UpdateIssueTag(
	ctx context.Context,
	tagID, actorID id.UUID,
	name string,
	description *string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "issue_tags", tagID, expectedVersion, map[string]any{
		"name":         name,
		"description":  description,
		"updated_by":   actorID,
		"lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) SetIssueTagStatus(
	ctx context.Context,
	tagID, actorID id.UUID,
	status string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "issue_tags", tagID, expectedVersion, map[string]any{
		"status":       status,
		"updated_by":   actorID,
		"lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) ReorderIssueTags(
	ctx context.Context,
	actorID id.UUID,
	items []ReorderItem,
) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			result := tx.Table("issue_tags").
				Where("id = ? AND lock_version = ?", item.ID, item.ExpectedLockVersion).
				Updates(map[string]any{
					"sort_order":   item.SortOrder,
					"updated_by":   actorID,
					"lock_version": gorm.Expr("lock_version + 1"),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrLockConflict
			}
		}
		return nil
	})
}

func updateWithLock(
	db *gorm.DB,
	table string,
	entityID id.UUID,
	expectedVersion uint32,
	updates map[string]any,
) error {
	result := db.Table(table).
		Where("id = ? AND lock_version = ?", entityID, expectedVersion).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLockConflict
	}
	return nil
}

func IsDuplicate(err error) bool {
	var mysqlError *mysqlDriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
