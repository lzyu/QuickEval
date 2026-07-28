package badcase

import (
	"context"
	"errors"

	"github.com/lzyu/QuickEval/apps/api/internal/attachment"
	"github.com/lzyu/QuickEval/apps/api/internal/dataset"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrLockConflict = errors.New("badcase lock version conflict")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository { return Repository{db: db} }

func (repository Repository) Transaction(ctx context.Context, fn func(Repository) error) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(Repository{db: tx})
	})
}

type ResultContext struct {
	ResultID     id.UUID
	ResultStatus string
	AnswerText   *string
	Score        *uint8
	Comment      *string
	ResultLock   uint32
	RunID        id.UUID
	RunStatus    string
	EvaluatorID  id.UUID
	AgentVersion string
	Environment  string
	ScenarioID   id.UUID
}

func (repository Repository) LockResultContext(
	ctx context.Context,
	resultID id.UUID,
) (ResultContext, error) {
	var item ResultContext
	err := repository.db.WithContext(ctx).Table("case_results").
		Select(`case_results.id AS result_id, case_results.status AS result_status,
			case_results.answer_text, case_results.score, case_results.comment,
			case_results.lock_version AS result_lock,
			evaluation_runs.id AS run_id, evaluation_runs.status AS run_status,
			evaluation_runs.evaluator_id, evaluation_runs.agent_version,
			evaluation_runs.environment, datasets.scenario_id`).
		Joins("JOIN evaluation_runs ON evaluation_runs.id = case_results.evaluation_run_id").
		Joins("JOIN dataset_versions ON dataset_versions.id = evaluation_runs.dataset_version_id").
		Joins("JOIN datasets ON datasets.id = dataset_versions.dataset_id").
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "case_results"}}).
		Where("case_results.id = ?", resultID).Take(&item).Error
	return item, err
}

func (repository Repository) CountAttachments(ctx context.Context, resultID id.UUID) (int64, error) {
	var count int64
	err := repository.db.WithContext(ctx).Table("attachments").
		Where("case_result_id = ?", resultID).Count(&count).Error
	return count, err
}

func (repository Repository) UpdateResult(
	ctx context.Context,
	item ResultContext,
	actorID id.UUID,
	status string,
	answer *string,
	score *uint8,
	comment *string,
	expectedVersion uint32,
) error {
	result := repository.db.WithContext(ctx).Table("case_results").
		Where("id = ? AND lock_version = ?", item.ResultID, expectedVersion).
		Updates(map[string]any{
			"status": status, "answer_text": answer, "score": score,
			"comment": comment, "skip_reason": nil, "updated_by": actorID,
			"lock_version": gorm.Expr("lock_version + 1"),
		})
	return affected(result)
}

func (repository Repository) BumpResult(
	ctx context.Context,
	resultID, actorID id.UUID,
	expectedVersion uint32,
) error {
	result := repository.db.WithContext(ctx).Table("case_results").
		Where("id = ? AND lock_version = ?", resultID, expectedVersion).
		Updates(map[string]any{
			"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
		})
	return affected(result)
}

func (repository Repository) BumpRun(ctx context.Context, runID, actorID id.UUID) error {
	return repository.db.WithContext(ctx).Table("evaluation_runs").Where("id = ?", runID).
		Updates(map[string]any{
			"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
		}).Error
}

func (repository Repository) FindByResult(
	ctx context.Context,
	resultID id.UUID,
	lock bool,
) (Badcase, error) {
	var item Badcase
	query := repository.db.WithContext(ctx)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.Where("case_result_id = ?", resultID).Take(&item).Error
	return item, err
}

func (repository Repository) Create(ctx context.Context, item *Badcase) error {
	return repository.db.WithContext(ctx).Create(item).Error
}

func (repository Repository) Restore(
	ctx context.Context,
	item Badcase,
	actorID id.UUID,
	title string,
	description, answer *string,
	agentVersion, environment string,
) error {
	result := repository.db.WithContext(ctx).Table("badcases").
		Where("id = ? AND lock_version = ?", item.ID, item.LockVersion).
		Updates(map[string]any{
			"title": title, "description": description, "agent_response_text": answer,
			"agent_version": agentVersion, "environment": environment,
			"occurred_at": gorm.Expr("UTC_TIMESTAMP(3)"), "status": "pending",
			"invalidated_at": nil, "invalidated_by": nil, "invalid_reason": nil,
			"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
		})
	return affected(result)
}

func (repository Repository) ReplaceTags(
	ctx context.Context,
	badcaseID, actorID id.UUID,
	tagIDs []id.UUID,
) error {
	if err := repository.db.WithContext(ctx).
		Where("badcase_id = ?", badcaseID).Delete(&BadcaseIssueTag{}).Error; err != nil {
		return err
	}
	items := make([]BadcaseIssueTag, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		items = append(items, BadcaseIssueTag{
			ID: id.MustNew(), BadcaseID: badcaseID, IssueTagID: tagID, CreatedBy: actorID,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return repository.db.WithContext(ctx).Create(&items).Error
}

func (repository Repository) ActiveIssueTags(
	ctx context.Context,
	tagIDs []id.UUID,
) ([]id.UUID, error) {
	var items []id.UUID
	err := repository.db.WithContext(ctx).Table("issue_tags").
		Where("id IN ? AND status = 'active'", tagIDs).Pluck("id", &items).Error
	return items, err
}

func (repository Repository) CreateActivity(ctx context.Context, item *Activity) error {
	return repository.db.WithContext(ctx).Create(item).Error
}

type Filters struct {
	Status     string
	ScenarioID *id.UUID
	IssueTagID *id.UUID
	Keyword    string
}

func (repository Repository) List(
	ctx context.Context,
	page, pageSize int,
	filters Filters,
) ([]Badcase, int64, error) {
	query := repository.baseQuery(repository.db.WithContext(ctx)).
		Where("badcases.source_type = 'evaluation' AND badcases.invalidated_at IS NULL")
	if filters.Status != "" {
		query = query.Where("badcases.status = ?", filters.Status)
	}
	if filters.ScenarioID != nil {
		query = query.Where("badcases.scenario_id = ?", *filters.ScenarioID)
	}
	if filters.IssueTagID != nil {
		query = query.Where(`EXISTS (
			SELECT 1 FROM badcase_issue_tags
			WHERE badcase_issue_tags.badcase_id = badcases.id
			AND badcase_issue_tags.issue_tag_id = ?
		)`, *filters.IssueTagID)
	}
	if filters.Keyword != "" {
		value := "%" + filters.Keyword + "%"
		query = query.Where(
			"(badcases.title LIKE ? OR badcases.description LIKE ? OR badcases.agent_response_text LIKE ?)",
			value, value, value,
		)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Badcase
	err := query.Select(repository.selectFields()).
		Order("badcases.occurred_at DESC, badcases.id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	if err := repository.loadDetails(ctx, items, false); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (repository Repository) Get(ctx context.Context, badcaseID id.UUID) (Badcase, error) {
	var item Badcase
	err := repository.baseQuery(repository.db.WithContext(ctx)).
		Select(repository.selectFields()).Where("badcases.id = ?", badcaseID).Take(&item).Error
	if err != nil {
		return Badcase{}, err
	}
	items := []Badcase{item}
	if err := repository.loadDetails(ctx, items, true); err != nil {
		return Badcase{}, err
	}
	return items[0], nil
}

func (repository Repository) baseQuery(db *gorm.DB) *gorm.DB {
	return db.Table("badcases").
		Joins("JOIN scenarios ON scenarios.id = badcases.scenario_id").
		Joins("JOIN evaluation_targets ON evaluation_targets.id = scenarios.evaluation_target_id").
		Joins("JOIN users creator ON creator.id = badcases.created_by").
		Joins("LEFT JOIN case_results ON case_results.id = badcases.case_result_id").
		Joins("LEFT JOIN evaluation_runs ON evaluation_runs.id = case_results.evaluation_run_id").
		Joins("LEFT JOIN users evaluator ON evaluator.id = evaluation_runs.evaluator_id").
		Joins("LEFT JOIN dataset_versions ON dataset_versions.id = evaluation_runs.dataset_version_id").
		Joins("LEFT JOIN datasets ON datasets.id = dataset_versions.dataset_id").
		Joins("LEFT JOIN version_cases ON version_cases.id = case_results.version_case_id")
}

func (repository Repository) selectFields() string {
	return `badcases.*, scenarios.name AS scenario_name,
		evaluation_targets.id AS evaluation_target_id,
		evaluation_targets.name AS evaluation_target_name,
		creator.display_name AS creator_name,
		evaluation_runs.id AS evaluation_run_id,
		evaluation_runs.evaluator_id, evaluator.display_name AS evaluator_name,
		datasets.id AS dataset_id, datasets.name AS dataset_name,
		dataset_versions.id AS dataset_version_id, dataset_versions.version_no,
		version_cases.id AS version_case_id, version_cases.name AS case_name,
		version_cases.user_prompt, case_results.score, case_results.comment`
}

func (repository Repository) loadDetails(
	ctx context.Context,
	items []Badcase,
	includeActivities bool,
) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]id.UUID, 0, len(items))
	resultIndex := make(map[id.UUID]int)
	badcaseIndex := make(map[id.UUID]int)
	for position := range items {
		ids = append(ids, items[position].ID)
		badcaseIndex[items[position].ID] = position
		items[position].IssueTags = []dataset.CaseTag{}
		items[position].Attachments = []attachment.Public{}
		items[position].Activities = []ActivityPublic{}
		if items[position].CaseResultID != nil {
			resultIndex[*items[position].CaseResultID] = position
		}
	}
	var tags []struct {
		BadcaseID id.UUID
		TagID     id.UUID
		Name      string
	}
	if err := repository.db.WithContext(ctx).Table("badcase_issue_tags").
		Select("badcase_issue_tags.badcase_id, issue_tags.id AS tag_id, issue_tags.name").
		Joins("JOIN issue_tags ON issue_tags.id = badcase_issue_tags.issue_tag_id").
		Where("badcase_issue_tags.badcase_id IN ?", ids).
		Order("issue_tags.sort_order ASC, issue_tags.name ASC").Scan(&tags).Error; err != nil {
		return err
	}
	for _, tag := range tags {
		position := badcaseIndex[tag.BadcaseID]
		items[position].IssueTags = append(items[position].IssueTags, dataset.CaseTag{
			ID: tag.TagID.String(), Name: tag.Name,
		})
	}
	if len(resultIndex) > 0 {
		resultIDs := make([]id.UUID, 0, len(resultIndex))
		for resultID := range resultIndex {
			resultIDs = append(resultIDs, resultID)
		}
		var attachments []attachment.Attachment
		if err := repository.db.WithContext(ctx).Where("case_result_id IN ?", resultIDs).
			Order("sort_order ASC, created_at ASC, id ASC").Find(&attachments).Error; err != nil {
			return err
		}
		for _, item := range attachments {
			position := resultIndex[*item.CaseResultID]
			items[position].Attachments = append(items[position].Attachments, item.Public())
		}
	}
	if !includeActivities {
		return nil
	}
	var activities []struct {
		Activity
		ActorName string
	}
	if err := repository.db.WithContext(ctx).Table("badcase_activities").
		Select("badcase_activities.*, users.display_name AS actor_name").
		Joins("JOIN users ON users.id = badcase_activities.actor_id").
		Where("badcase_id IN ?", ids).
		Order("created_at ASC, id ASC").Scan(&activities).Error; err != nil {
		return err
	}
	for _, item := range activities {
		position := badcaseIndex[item.BadcaseID]
		items[position].Activities = append(items[position].Activities, ActivityPublic{
			ID: item.ID.String(), Type: item.Type, Note: item.Note,
			ActorID: item.ActorID.String(), ActorName: item.ActorName, CreatedAt: item.CreatedAt,
		})
	}
	return nil
}

func affected(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLockConflict
	}
	return nil
}
