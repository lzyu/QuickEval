package dataset

import (
	"context"
	"errors"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrLockConflict = errors.New("dataset lock version conflict")

type Repository struct {
	db *gorm.DB
}

type TargetInfo struct {
	ID     id.UUID
	Status string
}

type ScenarioInfo struct {
	ID       id.UUID
	TargetID id.UUID
	Status   string
}

func (repository Repository) GetTargetInfo(
	ctx context.Context,
	targetID id.UUID,
) (TargetInfo, error) {
	var item TargetInfo
	err := repository.db.WithContext(ctx).Table("evaluation_targets").
		Select("id, status").Where("id = ?", targetID).Take(&item).Error
	return item, err
}

func (repository Repository) GetScenarioInfo(ctx context.Context, scenarioID id.UUID) (ScenarioInfo, error) {
	var item ScenarioInfo
	err := repository.db.WithContext(ctx).Table("scenarios").
		Select("id, evaluation_target_id AS target_id, status").
		Where("id = ?", scenarioID).Take(&item).Error
	return item, err
}

func NewRepository(db *gorm.DB) Repository { return Repository{db: db} }

func (repository Repository) Transaction(ctx context.Context, fn func(Repository) error) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(Repository{db: tx})
	})
}

func (repository Repository) ListDatasets(
	ctx context.Context,
	page, pageSize int,
	targetID, scenarioID *id.UUID,
	status, keyword string,
) ([]Dataset, int64, error) {
	base := repository.datasetQuery(repository.db.WithContext(ctx))
	if targetID != nil {
		base = base.Where("datasets.evaluation_target_id = ?", *targetID)
	}
	if scenarioID != nil {
		base = base.Where(`EXISTS (
			SELECT 1 FROM dataset_versions dv
			JOIN version_cases vc ON vc.dataset_version_id = dv.id
			WHERE dv.dataset_id = datasets.id AND vc.scenario_id = ?
		)`, *scenarioID)
	}
	if status != "" {
		base = base.Where("datasets.status = ?", status)
	}
	if keyword != "" {
		base = base.Where("datasets.name LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Dataset
	err := base.Select(repository.datasetSelect()).
		Order("datasets.updated_at DESC, datasets.id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error
	return items, total, err
}

func (repository Repository) GetDataset(ctx context.Context, datasetID id.UUID) (Dataset, error) {
	var item Dataset
	err := repository.datasetQuery(repository.db.WithContext(ctx)).
		Select(repository.datasetSelect()).
		Where("datasets.id = ?", datasetID).Take(&item).Error
	return item, err
}

func (repository Repository) LockDataset(ctx context.Context, datasetID id.UUID) (Dataset, error) {
	var item Dataset
	err := repository.datasetQuery(repository.db.WithContext(ctx)).
		Select(repository.datasetSelect()).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("datasets.id = ?", datasetID).Take(&item).Error
	return item, err
}

func (repository Repository) CreateDataset(ctx context.Context, item *Dataset) error {
	return repository.db.WithContext(ctx).Create(item).Error
}

func (repository Repository) UpdateDataset(
	ctx context.Context,
	datasetID, targetID, actorID id.UUID,
	name string,
	description *string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "datasets", datasetID, expectedVersion, map[string]any{
		"evaluation_target_id": targetID, "name": name, "description": description,
		"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) SetDatasetStatus(
	ctx context.Context,
	datasetID, actorID id.UUID,
	status string,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "datasets", datasetID, expectedVersion, map[string]any{
		"status": status, "updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) BumpDatasetLock(
	ctx context.Context,
	datasetID, actorID id.UUID,
	expectedVersion uint32,
) error {
	return updateWithLock(repository.db.WithContext(ctx), "datasets", datasetID, expectedVersion, map[string]any{
		"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
	})
}

func (repository Repository) CountDrafts(ctx context.Context, datasetID id.UUID) (int64, error) {
	var count int64
	err := repository.db.WithContext(ctx).Model(&Version{}).
		Where("dataset_id = ? AND status = ?", datasetID, VersionDraft).Count(&count).Error
	return count, err
}

func (repository Repository) MaxVersionNo(ctx context.Context, datasetID id.UUID) (uint32, error) {
	var value struct{ Maximum *uint32 }
	err := repository.db.WithContext(ctx).Model(&Version{}).
		Select("MAX(version_no) AS maximum").Where("dataset_id = ?", datasetID).Scan(&value).Error
	if value.Maximum == nil {
		return 0, err
	}
	return *value.Maximum, err
}

func (repository Repository) CountEnabledCases(ctx context.Context, versionID id.UUID) (int64, error) {
	var count int64
	err := repository.db.WithContext(ctx).Model(&VersionCase{}).
		Where("dataset_version_id = ? AND is_enabled = TRUE", versionID).Count(&count).Error
	return count, err
}

func (repository Repository) ListVersions(
	ctx context.Context,
	datasetID id.UUID,
) ([]Version, error) {
	var items []Version
	err := repository.versionQuery(repository.db.WithContext(ctx)).
		Where("dataset_versions.dataset_id = ?", datasetID).
		Order("dataset_versions.status = 'draft' DESC, dataset_versions.version_no DESC, dataset_versions.created_at DESC").
		Scan(&items).Error
	return items, err
}

func (repository Repository) GetVersion(ctx context.Context, versionID id.UUID) (Version, error) {
	var item Version
	err := repository.versionQuery(repository.db.WithContext(ctx)).
		Where("dataset_versions.id = ?", versionID).Take(&item).Error
	return item, err
}

func (repository Repository) LockVersion(ctx context.Context, versionID id.UUID) (Version, error) {
	var item Version
	err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", versionID).Take(&item).Error
	return item, err
}

func (repository Repository) CreateVersion(ctx context.Context, item *Version) error {
	return repository.db.WithContext(ctx).Create(item).Error
}

func (repository Repository) DeleteDraft(ctx context.Context, versionID id.UUID) error {
	result := repository.db.WithContext(ctx).
		Where("id = ? AND status = ?", versionID, VersionDraft).Delete(&Version{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLockConflict
	}
	return nil
}

func (repository Repository) PublishVersion(
	ctx context.Context,
	versionID, actorID id.UUID,
	versionNo uint32,
	releaseNote *string,
	expectedVersion uint32,
) error {
	result := repository.db.WithContext(ctx).Table("dataset_versions").
		Where("id = ? AND status = ? AND lock_version = ?", versionID, VersionDraft, expectedVersion).
		Updates(map[string]any{
			"version_no": versionNo, "status": VersionPublished, "release_note": releaseNote,
			"published_at": gorm.Expr("UTC_TIMESTAMP(3)"), "published_by": actorID,
			"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLockConflict
	}
	return nil
}

func (repository Repository) ArchiveVersion(
	ctx context.Context,
	versionID, actorID id.UUID,
	expectedVersion uint32,
) error {
	result := repository.db.WithContext(ctx).Table("dataset_versions").
		Where("id = ? AND status = ? AND lock_version = ?", versionID, VersionPublished, expectedVersion).
		Updates(map[string]any{
			"status": VersionArchived, "archived_at": gorm.Expr("UTC_TIMESTAMP(3)"),
			"archived_by": actorID, "updated_by": actorID,
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

func (repository Repository) ListCases(
	ctx context.Context,
	versionID id.UUID,
	page, pageSize int,
) ([]VersionCase, int64, error) {
	query := repository.db.WithContext(ctx).Model(&VersionCase{}).
		Where("dataset_version_id = ?", versionID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []VersionCase
	if err := repository.caseQuery(repository.db.WithContext(ctx)).
		Where("version_cases.dataset_version_id = ?", versionID).
		Order("version_cases.sort_order ASC, version_cases.created_at ASC, version_cases.id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	if err := repository.loadTags(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (repository Repository) AllCases(ctx context.Context, versionID id.UUID) ([]VersionCase, error) {
	var items []VersionCase
	if err := repository.caseQuery(repository.db.WithContext(ctx)).
		Where("version_cases.dataset_version_id = ?", versionID).
		Order("version_cases.sort_order ASC, version_cases.created_at ASC, version_cases.id ASC").
		Scan(&items).Error; err != nil {
		return nil, err
	}
	if err := repository.loadTags(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (repository Repository) GetCase(ctx context.Context, caseID id.UUID) (VersionCase, error) {
	var item VersionCase
	if err := repository.caseQuery(repository.db.WithContext(ctx)).
		Where("version_cases.id = ?", caseID).Take(&item).Error; err != nil {
		return VersionCase{}, err
	}
	items := []VersionCase{item}
	if err := repository.loadTags(ctx, items); err != nil {
		return VersionCase{}, err
	}
	return items[0], nil
}

func (repository Repository) NextSortOrder(ctx context.Context, versionID id.UUID) (uint32, error) {
	var value struct{ Maximum *uint32 }
	err := repository.db.WithContext(ctx).Model(&VersionCase{}).
		Select("MAX(sort_order) AS maximum").Where("dataset_version_id = ?", versionID).Scan(&value).Error
	if value.Maximum == nil {
		return 10, err
	}
	return *value.Maximum + 10, err
}

func (repository Repository) CreateCase(ctx context.Context, item *VersionCase, tags []VersionCaseTag) error {
	if err := repository.db.WithContext(ctx).Create(item).Error; err != nil {
		return err
	}
	if len(tags) > 0 {
		return repository.db.WithContext(ctx).Create(&tags).Error
	}
	return nil
}

func (repository Repository) UpdateCase(
	ctx context.Context,
	item VersionCase,
	actorID id.UUID,
	expectedVersion uint32,
	tags []VersionCaseTag,
) error {
	result := repository.db.WithContext(ctx).Table("version_cases").
		Where("id = ? AND lock_version = ?", item.ID, expectedVersion).
		Updates(map[string]any{
			"name": item.Name, "user_prompt": item.UserPrompt, "precondition": item.Precondition,
			"expected_result": item.ExpectedResult, "judging_guide": item.JudgingGuide,
			"scenario_id": item.ScenarioID, "scenario_assignment_status": item.AssignmentStatus,
			"is_enabled": item.IsEnabled, "updated_by": actorID,
			"lock_version": gorm.Expr("lock_version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLockConflict
	}
	if err := repository.db.WithContext(ctx).Where("version_case_id = ?", item.ID).
		Delete(&VersionCaseTag{}).Error; err != nil {
		return err
	}
	if len(tags) > 0 {
		return repository.db.WithContext(ctx).Create(&tags).Error
	}
	return nil
}

func (repository Repository) DeleteCase(ctx context.Context, caseID id.UUID, expectedVersion uint32) error {
	result := repository.db.WithContext(ctx).
		Where("id = ? AND lock_version = ?", caseID, expectedVersion).Delete(&VersionCase{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLockConflict
	}
	return nil
}

func (repository Repository) ReorderCases(
	ctx context.Context,
	versionID, actorID id.UUID,
	items []ReorderItem,
) error {
	for _, item := range items {
		result := repository.db.WithContext(ctx).Table("version_cases").
			Where("id = ? AND dataset_version_id = ? AND lock_version = ?",
				item.ID, versionID, item.ExpectedLockVersion).
			Updates(map[string]any{
				"sort_order": item.SortOrder, "updated_by": actorID,
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
}

func (repository Repository) BumpVersionLock(ctx context.Context, versionID, actorID id.UUID) error {
	result := repository.db.WithContext(ctx).Table("dataset_versions").Where("id = ?", versionID).
		Updates(map[string]any{
			"updated_by": actorID, "lock_version": gorm.Expr("lock_version + 1"),
		})
	return result.Error
}

func (repository Repository) FindActiveTags(
	ctx context.Context,
	targetID id.UUID,
	tagIDs []id.UUID,
) (map[id.UUID]string, error) {
	if len(tagIDs) == 0 {
		return map[id.UUID]string{}, nil
	}
	var rows []struct {
		ID   id.UUID
		Name string
	}
	query := repository.db.WithContext(ctx).Table("case_tags").Select("id, name").
		Where("status = 'active' AND id IN ?", tagIDs)
	query = query.Where("scope = 'global' OR evaluation_target_id = ?", targetID)
	err := query.Scan(&rows).Error
	result := make(map[id.UUID]string, len(rows))
	for _, row := range rows {
		result[row.ID] = row.Name
	}
	return result, err
}

func (repository Repository) FindActiveTagsByNames(
	ctx context.Context,
	targetID id.UUID,
	names []string,
) (map[string]id.UUID, error) {
	if len(names) == 0 {
		return map[string]id.UUID{}, nil
	}
	var rows []struct {
		ID   id.UUID
		Name string
	}
	query := repository.db.WithContext(ctx).Table("case_tags").Select("id, name").
		Where("status = 'active' AND name IN ?", names)
	query = query.Where("scope = 'global' OR evaluation_target_id = ?", targetID)
	err := query.Scan(&rows).Error
	result := make(map[string]id.UUID, len(rows))
	for _, row := range rows {
		result[row.Name] = row.ID
	}
	return result, err
}

func (repository Repository) datasetQuery(db *gorm.DB) *gorm.DB {
	return db.Table("datasets").
		Joins("JOIN evaluation_targets ON evaluation_targets.id = datasets.evaluation_target_id")
}

func (repository Repository) datasetSelect() string {
	return "datasets.*, evaluation_targets.name AS evaluation_target_name, " +
		"evaluation_targets.status AS evaluation_target_status, " +
		"(SELECT MAX(version_no) FROM dataset_versions WHERE dataset_id = datasets.id) AS latest_version_no, " +
		"(SELECT COUNT(*) FROM dataset_versions WHERE dataset_id = datasets.id AND status IN ('published','archived')) AS published_version_count, " +
		"(SELECT id FROM dataset_versions WHERE dataset_id = datasets.id AND status = 'draft' ORDER BY created_at DESC LIMIT 1) AS draft_version_id, " +
		"(SELECT COUNT(*) FROM version_cases WHERE dataset_version_id = " +
		"(SELECT id FROM dataset_versions WHERE dataset_id = datasets.id AND status = 'draft' ORDER BY created_at DESC LIMIT 1)) AS draft_case_count"
}

func (repository Repository) caseQuery(db *gorm.DB) *gorm.DB {
	return db.Table("version_cases").
		Select("version_cases.*, scenarios.name AS scenario_name, scenarios.status AS scenario_status").
		Joins("LEFT JOIN scenarios ON scenarios.id = version_cases.scenario_id")
}

func (repository Repository) versionQuery(db *gorm.DB) *gorm.DB {
	return db.Table("dataset_versions").Select(
		"dataset_versions.*, " +
			"(SELECT COUNT(*) FROM version_cases WHERE dataset_version_id = dataset_versions.id) AS case_count, " +
			"(SELECT COUNT(*) FROM version_cases WHERE dataset_version_id = dataset_versions.id AND is_enabled = TRUE) AS enabled_count",
	)
}

func (repository Repository) loadTags(ctx context.Context, items []VersionCase) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]id.UUID, 0, len(items))
	index := make(map[id.UUID]int, len(items))
	for position := range items {
		ids = append(ids, items[position].ID)
		index[items[position].ID] = position
		items[position].Tags = []CaseTag{}
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
		items[position].Tags = append(items[position].Tags, CaseTag{
			ID: row.CaseTagID.String(), Name: row.TagNameSnapshot,
		})
	}
	return nil
}

func updateWithLock(
	db *gorm.DB,
	table string,
	entityID id.UUID,
	expectedVersion uint32,
	updates map[string]any,
) error {
	result := db.Table(table).Where("id = ? AND lock_version = ?", entityID, expectedVersion).Updates(updates)
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
