package evaluation

import (
	"context"
	"errors"

	"github.com/lzyu/QuickEval/apps/api/internal/attachment"
	"github.com/lzyu/QuickEval/apps/api/internal/dataset"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrLockConflict = errors.New("evaluation lock version conflict")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository { return Repository{db: db} }

func (repository Repository) Transaction(ctx context.Context, fn func(Repository) error) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(Repository{db: tx})
	})
}

type VersionContext struct {
	ID             id.UUID
	DatasetID      id.UUID
	Status         string
	DatasetStatus  string
	ScenarioStatus string
	TargetStatus   string
	EnabledCount   int64
}

func (repository Repository) GetVersionContext(
	ctx context.Context,
	versionID id.UUID,
) (VersionContext, error) {
	var item VersionContext
	err := repository.db.WithContext(ctx).Table("dataset_versions").
		Select(`dataset_versions.id, dataset_versions.dataset_id, dataset_versions.status,
			datasets.status AS dataset_status,
			scenarios.status AS scenario_status,
			evaluation_targets.status AS target_status,
			(SELECT COUNT(*) FROM version_cases
			 WHERE dataset_version_id = dataset_versions.id AND is_enabled = TRUE) AS enabled_count`).
		Joins("JOIN datasets ON datasets.id = dataset_versions.dataset_id").
		Joins("JOIN scenarios ON scenarios.id = datasets.scenario_id").
		Joins("JOIN evaluation_targets ON evaluation_targets.id = scenarios.evaluation_target_id").
		Where("dataset_versions.id = ?", versionID).Take(&item).Error
	return item, err
}

func (repository Repository) EnabledCaseIDs(
	ctx context.Context,
	versionID id.UUID,
) ([]id.UUID, error) {
	var items []id.UUID
	err := repository.db.WithContext(ctx).Table("version_cases").
		Where("dataset_version_id = ? AND is_enabled = TRUE", versionID).
		Order("sort_order ASC, created_at ASC, id ASC").Pluck("id", &items).Error
	return items, err
}

func (repository Repository) CreateRun(ctx context.Context, item *Run) error {
	return repository.db.WithContext(ctx).Create(item).Error
}

func (repository Repository) CreateResults(ctx context.Context, items []Result) error {
	if len(items) == 0 {
		return nil
	}
	return repository.db.WithContext(ctx).Create(&items).Error
}

type RunFilters struct {
	EvaluatorID *id.UUID
	Status      string
	Environment string
	DatasetID   *id.UUID
	ScenarioID  *id.UUID
	Keyword     string
}

func (repository Repository) ListRuns(
	ctx context.Context,
	page, pageSize int,
	filters RunFilters,
) ([]Run, int64, error) {
	query := repository.runQuery(repository.db.WithContext(ctx))
	if filters.EvaluatorID != nil {
		query = query.Where("evaluation_runs.evaluator_id = ?", *filters.EvaluatorID)
	}
	if filters.Status != "" {
		query = query.Where("evaluation_runs.status = ?", filters.Status)
	}
	if filters.Environment != "" {
		query = query.Where("evaluation_runs.environment = ?", filters.Environment)
	}
	if filters.DatasetID != nil {
		query = query.Where("datasets.id = ?", *filters.DatasetID)
	}
	if filters.ScenarioID != nil {
		query = query.Where("scenarios.id = ?", *filters.ScenarioID)
	}
	if filters.Keyword != "" {
		value := "%" + filters.Keyword + "%"
		query = query.Where(
			"(datasets.name LIKE ? OR evaluation_runs.agent_version LIKE ?)",
			value, value,
		)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Run
	err := query.Select(repository.runSelect()).
		Order("evaluation_runs.updated_at DESC, evaluation_runs.id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error
	return items, total, err
}

func (repository Repository) GetRun(ctx context.Context, runID id.UUID) (Run, error) {
	var item Run
	err := repository.runQuery(repository.db.WithContext(ctx)).
		Select(repository.runSelect()).Where("evaluation_runs.id = ?", runID).
		Take(&item).Error
	return item, err
}

func (repository Repository) LockRun(ctx context.Context, runID id.UUID) (Run, error) {
	var item Run
	err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", runID).Take(&item).Error
	return item, err
}

func (repository Repository) UpdateRun(
	ctx context.Context,
	runID, actorID id.UUID,
	agentVersion, environment string,
	purposeNote, configNote *string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "evaluation_runs", runID, expectedVersion, map[string]any{
		"agent_version": agentVersion, "environment": environment,
		"purpose_note": purposeNote, "config_note": configNote,
		"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) BumpRun(ctx context.Context, runID, actorID id.UUID) error {
	return repository.db.WithContext(ctx).Table("evaluation_runs").Where("id = ?", runID).
		Updates(map[string]any{
			"updated_by":   actorID,
			"lock_version": gorm.Expr("lock_version + 1"),
		}).Error
}

func (repository Repository) CompleteRun(
	ctx context.Context,
	runID, actorID id.UUID,
	expectedVersion uint32,
) error {
	result := repository.db.WithContext(ctx).Table("evaluation_runs").
		Where("id = ? AND status = ? AND lock_version = ?", runID, RunInProgress, expectedVersion).
		Updates(map[string]any{
			"status":             RunCompleted,
			"first_completed_at": gorm.Expr("COALESCE(first_completed_at, UTC_TIMESTAMP(3))"),
			"completed_at":       gorm.Expr("UTC_TIMESTAMP(3)"),
			"updated_by":         actorID, "lock_version": gorm.Expr("lock_version + 1"),
		})
	return affected(result)
}

func (repository Repository) ReopenRun(
	ctx context.Context,
	runID, actorID id.UUID,
	expectedVersion uint32,
) error {
	result := repository.db.WithContext(ctx).Table("evaluation_runs").
		Where("id = ? AND status = ? AND lock_version = ?", runID, RunCompleted, expectedVersion).
		Updates(map[string]any{
			"status": RunInProgress, "completed_at": nil,
			"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
		})
	return affected(result)
}

func (repository Repository) VoidRun(
	ctx context.Context,
	runID, actorID id.UUID,
	reason string,
	expectedVersion uint32,
) error {
	result := repository.db.WithContext(ctx).Table("evaluation_runs").
		Where("id = ? AND status IN ? AND lock_version = ?",
			runID, []string{RunInProgress, RunCompleted}, expectedVersion).
		Updates(map[string]any{
			"status": RunVoided, "voided_at": gorm.Expr("UTC_TIMESTAMP(3)"),
			"voided_by": actorID, "void_reason": reason,
			"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
		})
	return affected(result)
}

func (repository Repository) DeleteRun(ctx context.Context, runID id.UUID) error {
	result := repository.db.WithContext(ctx).Where("id = ?", runID).Delete(&Run{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (repository Repository) CountPending(ctx context.Context, runID id.UUID) (int64, error) {
	var count int64
	err := repository.db.WithContext(ctx).Model(&Result{}).
		Where("evaluation_run_id = ? AND status = ?", runID, ResultPending).Count(&count).Error
	return count, err
}

func (repository Repository) CountBadcases(ctx context.Context, runID id.UUID) (int64, error) {
	var count int64
	err := repository.db.WithContext(ctx).Table("badcases").
		Joins("JOIN case_results ON case_results.id = badcases.case_result_id").
		Where("case_results.evaluation_run_id = ?", runID).Count(&count).Error
	return count, err
}

func (repository Repository) ListResults(
	ctx context.Context,
	runID id.UUID,
	page, pageSize int,
	status string,
) ([]Result, int64, error) {
	query := repository.resultQuery(repository.db.WithContext(ctx)).
		Where("case_results.evaluation_run_id = ?", runID)
	if status != "" {
		query = query.Where("case_results.status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Result
	err := query.Select(repository.resultSelect()).
		Order("version_cases.sort_order ASC, version_cases.created_at ASC, version_cases.id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	if err := repository.loadResultTags(ctx, items); err != nil {
		return nil, 0, err
	}
	if err := repository.loadResultEvidence(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (repository Repository) GetResult(ctx context.Context, resultID id.UUID) (Result, error) {
	var item Result
	err := repository.resultQuery(repository.db.WithContext(ctx)).
		Select(repository.resultSelect()).Where("case_results.id = ?", resultID).Take(&item).Error
	if err != nil {
		return Result{}, err
	}
	items := []Result{item}
	if err := repository.loadResultTags(ctx, items); err != nil {
		return Result{}, err
	}
	if err := repository.loadResultEvidence(ctx, items); err != nil {
		return Result{}, err
	}
	return items[0], nil
}

func (repository Repository) CountResultAttachments(
	ctx context.Context,
	resultID id.UUID,
) (int64, error) {
	var count int64
	err := repository.db.WithContext(ctx).Table("attachments").
		Where("case_result_id = ?", resultID).Count(&count).Error
	return count, err
}

func (repository Repository) UpdateResult(
	ctx context.Context,
	resultID, actorID id.UUID,
	status string,
	answerText *string,
	score *uint8,
	comment, skipReason *string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "case_results", resultID, expectedVersion, map[string]any{
		"status": status, "answer_text": answerText, "score": score,
		"comment": comment, "skip_reason": skipReason,
		"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) runQuery(db *gorm.DB) *gorm.DB {
	return db.Table("evaluation_runs").
		Joins("JOIN dataset_versions ON dataset_versions.id = evaluation_runs.dataset_version_id").
		Joins("JOIN datasets ON datasets.id = dataset_versions.dataset_id").
		Joins("JOIN scenarios ON scenarios.id = datasets.scenario_id").
		Joins("JOIN evaluation_targets ON evaluation_targets.id = scenarios.evaluation_target_id").
		Joins("JOIN users ON users.id = evaluation_runs.evaluator_id")
}

func (repository Repository) runSelect() string {
	return `evaluation_runs.*,
		datasets.id AS dataset_id, datasets.name AS dataset_name,
		dataset_versions.version_no AS version_no,
		scenarios.id AS scenario_id, scenarios.name AS scenario_name,
		evaluation_targets.id AS evaluation_target_id,
		evaluation_targets.name AS evaluation_target_name,
		users.display_name AS evaluator_name,
		(SELECT COUNT(*) FROM case_results WHERE evaluation_run_id = evaluation_runs.id) AS total_count,
		(SELECT COUNT(*) FROM case_results WHERE evaluation_run_id = evaluation_runs.id AND status = 'pending') AS pending_count,
		(SELECT COUNT(*) FROM case_results WHERE evaluation_run_id = evaluation_runs.id AND status = 'evaluated') AS evaluated_count,
		(SELECT COUNT(*) FROM case_results WHERE evaluation_run_id = evaluation_runs.id AND status = 'skipped') AS skipped_count,
		(SELECT COUNT(*) FROM case_results WHERE evaluation_run_id = evaluation_runs.id AND score IS NOT NULL) AS scored_count,
		(SELECT COUNT(*) FROM badcases JOIN case_results cr ON cr.id = badcases.case_result_id
		 WHERE cr.evaluation_run_id = evaluation_runs.id AND badcases.invalidated_at IS NULL) AS badcase_count,
		(SELECT AVG(score) FROM case_results
		 WHERE evaluation_run_id = evaluation_runs.id AND status = 'evaluated' AND score IS NOT NULL) AS average_score`
}

func (repository Repository) resultQuery(db *gorm.DB) *gorm.DB {
	return db.Table("case_results").
		Joins("JOIN version_cases ON version_cases.id = case_results.version_case_id")
}

func (repository Repository) resultSelect() string {
	return `case_results.*, version_cases.case_key, version_cases.name AS case_name,
		version_cases.user_prompt, version_cases.precondition, version_cases.expected_result,
		version_cases.judging_guide, version_cases.sort_order,
		EXISTS(SELECT 1 FROM badcases
		 WHERE badcases.case_result_id = case_results.id AND badcases.invalidated_at IS NULL) AS has_badcase`
}

func (repository Repository) loadResultTags(ctx context.Context, items []Result) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]id.UUID, 0, len(items))
	index := make(map[id.UUID]int, len(items))
	for position := range items {
		ids = append(ids, items[position].VersionCaseID)
		index[items[position].VersionCaseID] = position
		items[position].Tags = []dataset.CaseTag{}
	}
	var rows []struct {
		VersionCaseID   id.UUID
		CaseTagID       id.UUID
		TagNameSnapshot string
	}
	if err := repository.db.WithContext(ctx).Table("version_case_tags").
		Where("version_case_id IN ?", ids).Order("created_at ASC").Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		position := index[row.VersionCaseID]
		items[position].Tags = append(items[position].Tags, dataset.CaseTag{
			ID: row.CaseTagID.String(), Name: row.TagNameSnapshot,
		})
	}
	return nil
}

func (repository Repository) loadResultEvidence(ctx context.Context, items []Result) error {
	if len(items) == 0 {
		return nil
	}
	resultIDs := make([]id.UUID, 0, len(items))
	index := make(map[id.UUID]int, len(items))
	for position := range items {
		resultIDs = append(resultIDs, items[position].ID)
		index[items[position].ID] = position
		items[position].Attachments = []attachment.Public{}
	}
	var attachmentItems []attachment.Attachment
	if err := repository.db.WithContext(ctx).
		Where("case_result_id IN ?", resultIDs).
		Order("sort_order ASC, created_at ASC, id ASC").
		Find(&attachmentItems).Error; err != nil {
		return err
	}
	for _, item := range attachmentItems {
		position := index[*item.CaseResultID]
		items[position].Attachments = append(items[position].Attachments, item.Public())
	}
	var badcases []struct {
		ID           id.UUID
		CaseResultID id.UUID
		Title        string
		Description  *string
		Status       string
	}
	if err := repository.db.WithContext(ctx).Table("badcases").
		Select("id, case_result_id, title, description, status").
		Where("case_result_id IN ? AND invalidated_at IS NULL", resultIDs).
		Scan(&badcases).Error; err != nil {
		return err
	}
	if len(badcases) == 0 {
		return nil
	}
	badcaseIDs := make([]id.UUID, 0, len(badcases))
	badcaseIndex := make(map[id.UUID]*BadcaseSummary, len(badcases))
	for _, item := range badcases {
		summary := &BadcaseSummary{
			ID: item.ID.String(), Title: item.Title, Description: item.Description,
			Status: item.Status, IssueTags: []dataset.CaseTag{},
		}
		items[index[item.CaseResultID]].Badcase = summary
		badcaseIDs = append(badcaseIDs, item.ID)
		badcaseIndex[item.ID] = summary
	}
	var tags []struct {
		BadcaseID id.UUID
		TagID     id.UUID
		Name      string
	}
	if err := repository.db.WithContext(ctx).Table("badcase_issue_tags").
		Select("badcase_issue_tags.badcase_id, issue_tags.id AS tag_id, issue_tags.name").
		Joins("JOIN issue_tags ON issue_tags.id = badcase_issue_tags.issue_tag_id").
		Where("badcase_issue_tags.badcase_id IN ?", badcaseIDs).
		Order("issue_tags.sort_order ASC, issue_tags.name ASC").Scan(&tags).Error; err != nil {
		return err
	}
	for _, tag := range tags {
		badcaseIndex[tag.BadcaseID].IssueTags = append(
			badcaseIndex[tag.BadcaseID].IssueTags,
			dataset.CaseTag{ID: tag.TagID.String(), Name: tag.Name},
		)
	}
	return nil
}

func updateWithLock(
	db *gorm.DB,
	table string,
	itemID id.UUID,
	expectedVersion uint32,
	values map[string]any,
) error {
	result := db.Table(table).Where("id = ? AND lock_version = ?", itemID, expectedVersion).Updates(values)
	return affected(result)
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
