package reporting

import (
	"context"
	"fmt"
	"strings"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return Repository{db: db}
}

func (repository Repository) Home(ctx context.Context, actorID id.UUID) (Home, error) {
	var inProgress, completed, assigned int64
	if err := repository.db.WithContext(ctx).Table("evaluation_runs").
		Where("evaluator_id = ? AND status = 'in_progress'", actorID).
		Count(&inProgress).Error; err != nil {
		return Home{}, err
	}
	if err := repository.db.WithContext(ctx).Table("evaluation_runs").
		Where("evaluator_id = ? AND status = 'completed'", actorID).
		Count(&completed).Error; err != nil {
		return Home{}, err
	}
	if err := repository.db.WithContext(ctx).Table("badcases").
		Where("assignee_id = ? AND invalidated_at IS NULL AND status IN ('pending','processing')", actorID).
		Count(&assigned).Error; err != nil {
		return Home{}, err
	}

	var runs []HomeRun
	err := repository.db.WithContext(ctx).Table("evaluation_runs er").
		Select(`er.id, d.name AS dataset_name, dv.version_no,
			t.name AS evaluation_target_name,
			er.agent_version, er.environment, er.updated_at,
			COUNT(cr.id) AS total_count,
			COALESCE(SUM(cr.status = 'evaluated'), 0) AS evaluated_count,
			COALESCE(SUM(cr.status = 'skipped'), 0) AS skipped_count`).
		Joins("JOIN dataset_versions dv ON dv.id = er.dataset_version_id").
		Joins("JOIN datasets d ON d.id = dv.dataset_id").
		Joins("JOIN evaluation_targets t ON t.id = d.evaluation_target_id").
		Joins("LEFT JOIN case_results cr ON cr.evaluation_run_id = er.id").
		Where("er.evaluator_id = ? AND er.status = 'in_progress'", actorID).
		Group("er.id, d.name, dv.version_no, t.name, er.agent_version, er.environment, er.updated_at").
		Order("er.updated_at DESC").Limit(5).Scan(&runs).Error
	if err != nil {
		return Home{}, err
	}

	var badcases []HomeBadcase
	err = repository.db.WithContext(ctx).Table("badcases b").
		Select("b.id, b.title, COALESCE(s.name, '待归类') AS scenario_name, b.status, b.source_type, b.updated_at").
		Joins("LEFT JOIN scenarios s ON s.id = b.scenario_id").
		Where("b.assignee_id = ? AND b.invalidated_at IS NULL AND b.status IN ('pending','processing')", actorID).
		Order("FIELD(b.status, 'processing', 'pending'), b.updated_at DESC").Limit(5).
		Scan(&badcases).Error
	if err != nil {
		return Home{}, err
	}

	var datasets []RecentDataset
	err = repository.db.WithContext(ctx).Table("dataset_versions dv").
		Select(`d.id AS dataset_id, dv.id AS version_id, d.name AS dataset_name,
			t.name AS target_name, dv.version_no, dv.published_at,
			COUNT(vc.id) AS case_count`).
		Joins("JOIN datasets d ON d.id = dv.dataset_id AND d.status = 'active'").
		Joins("JOIN evaluation_targets t ON t.id = d.evaluation_target_id AND t.status = 'active'").
		Joins("LEFT JOIN version_cases vc ON vc.dataset_version_id = dv.id AND vc.is_enabled = 1").
		Where("dv.status = 'published'").
		Group("d.id, dv.id, d.name, t.name, dv.version_no, dv.published_at").
		Order("dv.published_at DESC").Limit(5).Scan(&datasets).Error
	if err != nil {
		return Home{}, err
	}

	var activities []Activity
	err = repository.db.WithContext(ctx).Table("badcase_activities ba").
		Select(`ba.id, ba.badcase_id, b.title AS badcase_title, ba.activity_type,
			ba.note, u.display_name AS actor_name, ba.created_at`).
		Joins("JOIN badcases b ON b.id = ba.badcase_id").
		Joins("JOIN users u ON u.id = ba.actor_id").
		Where(`ba.actor_id = ? OR b.assignee_id = ? OR b.created_by = ?
			OR ba.from_assignee_id = ? OR ba.to_assignee_id = ?`,
			actorID, actorID, actorID, actorID, actorID).
		Order("ba.created_at DESC, ba.id DESC").Limit(10).Scan(&activities).Error
	if err != nil {
		return Home{}, err
	}

	result := Home{
		Metrics: []HomeMetric{
			{Key: "in_progress", Label: "我的进行中评测", Value: inProgress, URL: "/evaluations?status=in_progress"},
			{Key: "completed", Label: "我的已完成评测", Value: completed, URL: "/evaluations?status=completed"},
			{Key: "assigned_badcases", Label: "分配给我的未关闭 Badcase", Value: assigned, URL: "/badcases?assigned_to_me=1&open=1"},
		},
		ContinueRuns:     make([]HomeRunPublic, 0, len(runs)),
		AssignedBadcases: make([]HomeBadcasePublic, 0, len(badcases)),
		RecentDatasets:   make([]RecentDatasetPublic, 0, len(datasets)),
		RecentActivities: make([]ActivityPublic, 0, len(activities)),
	}
	for _, item := range runs {
		result.ContinueRuns = append(result.ContinueRuns, item.Public())
	}
	for _, item := range badcases {
		result.AssignedBadcases = append(result.AssignedBadcases, item.Public())
	}
	for _, item := range datasets {
		result.RecentDatasets = append(result.RecentDatasets, item.Public())
	}
	for _, item := range activities {
		result.RecentActivities = append(result.RecentActivities, item.Public())
	}
	return result, nil
}

func (repository Repository) Dashboard(ctx context.Context, filters Filters) (Dashboard, error) {
	var result Dashboard
	var err error
	result.Metrics, err = repository.metrics(ctx, filters)
	if err != nil {
		return Dashboard{}, err
	}
	result.ScoreDistribution, err = repository.scoreDistribution(ctx, filters)
	if err != nil {
		return Dashboard{}, err
	}
	result.SkipReasonDistribution, err = repository.skipDistribution(ctx, filters)
	if err != nil {
		return Dashboard{}, err
	}
	result.IssueTagDistribution, err = repository.badcaseDistribution(ctx, filters, "tag")
	if err != nil {
		return Dashboard{}, err
	}
	result.StatusDistribution, err = repository.badcaseDistribution(ctx, filters, "status")
	if err != nil {
		return Dashboard{}, err
	}
	result.SourceDistribution, err = repository.badcaseDistribution(ctx, filters, "source")
	if err != nil {
		return Dashboard{}, err
	}
	result.VersionComparison, err = repository.versionComparison(ctx, filters)
	if err != nil {
		return Dashboard{}, err
	}
	result.Options, err = repository.options(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	return result, nil
}

func (repository Repository) EvaluationResults(
	ctx context.Context,
	page, pageSize int,
	filters EvaluationResultFilters,
) ([]EvaluationResultDetail, int64, int64, error) {
	buildQuery := func() *gorm.DB {
		query := repository.applyEvaluationFilters(repository.evaluationBase(ctx), filters.Filters).
			Joins("JOIN case_results cr ON cr.evaluation_run_id = er.id").
			Joins("JOIN version_cases vc ON vc.id = cr.version_case_id").
			Joins("LEFT JOIN scenarios s ON s.id = vc.scenario_id").
			Joins("LEFT JOIN badcases b_detail ON b_detail.case_result_id = cr.id AND b_detail.invalidated_at IS NULL")
		if filters.ResultID != nil {
			query = query.Where("cr.id = ?", *filters.ResultID)
		}
		if filters.ResultStatus != "" {
			query = query.Where("cr.status = ?", filters.ResultStatus)
		}
		if filters.Score != nil {
			query = query.Where("cr.status = 'evaluated' AND cr.score = ?", *filters.Score)
		}
		if filters.SkipReason != "" {
			query = query.Where("cr.status = 'skipped' AND cr.skip_reason = ?", filters.SkipReason)
		}
		if filters.HasBadcase != nil {
			if *filters.HasBadcase {
				query = query.Where("b_detail.id IS NOT NULL")
			} else {
				query = query.Where("b_detail.id IS NULL")
			}
		}
		if filters.Scored != nil {
			if *filters.Scored {
				query = query.Where("cr.status = 'evaluated' AND cr.score IS NOT NULL")
			} else {
				query = query.Where("cr.status = 'evaluated' AND cr.score IS NULL")
			}
		}
		if filters.Keyword != "" {
			value := "%" + escapeLike(filters.Keyword) + "%"
			query = query.Where(`(vc.name LIKE ? OR vc.user_prompt LIKE ? OR cr.answer_text LIKE ?
				OR cr.comment LIKE ? OR evaluator.display_name LIKE ?)`, value, value, value, value, value)
		}
		if filters.ScenarioID != nil {
			query = query.Where("vc.scenario_id = ?", *filters.ScenarioID)
		}
		return query
	}

	var total int64
	if err := buildQuery().Distinct("cr.id").Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}
	var runCount int64
	if err := buildQuery().Distinct("er.id").Count(&runCount).Error; err != nil {
		return nil, 0, 0, err
	}
	var items []EvaluationResultDetail
	err := buildQuery().Select(`BIN_TO_UUID(er.id) AS evaluation_run_id, BIN_TO_UUID(cr.id) AS id,
			t.name AS evaluation_target_name, COALESCE(s.name, '待归类') AS scenario_name,
			d.name AS dataset_name,
			dv.version_no, evaluator.display_name AS evaluator_name, er.agent_version,
			er.environment, er.completed_at, vc.name AS case_name, vc.user_prompt,
			cr.status AS result_status, cr.answer_text, cr.score, cr.comment, cr.skip_reason,
			b_detail.id IS NOT NULL AS has_badcase, BIN_TO_UUID(b_detail.id) AS badcase_id,
			b_detail.title AS badcase_title,
			COALESCE((SELECT GROUP_CONCAT(vct.tag_name_snapshot ORDER BY vct.tag_name_snapshot SEPARATOR '；')
				FROM version_case_tags vct WHERE vct.version_case_id = vc.id), '') AS case_tags,
			(SELECT COUNT(*) FROM attachments a WHERE a.case_result_id = cr.id) AS evidence_count`).
		Order("er.completed_at DESC, er.id, cr.id").
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error
	if err != nil {
		return nil, 0, 0, err
	}
	if items == nil {
		items = []EvaluationResultDetail{}
	}
	for index := range items {
		items[index].ResultDetailURL = "/evaluation-runs/" + items[index].RunID +
			"/result?result_id=" + items[index].ResultID
	}
	return items, total, runCount, nil
}

func (repository Repository) metrics(ctx context.Context, filters Filters) (Metrics, error) {
	var result Metrics
	runs := repository.applyEvaluationFilters(repository.evaluationBase(ctx), filters)
	if err := runs.Distinct("er.id").Count(&result.CompletedRunCount).Error; err != nil {
		return Metrics{}, err
	}

	results := repository.applyEvaluationFilters(repository.evaluationBase(ctx), filters).
		Joins("JOIN case_results cr ON cr.evaluation_run_id = er.id").
		Joins("JOIN version_cases vc_metric ON vc_metric.id = cr.version_case_id")
	results = repository.applyResultScenarioFilter(results, filters, "vc_metric")
	row := struct {
		Evaluated int64
		Scored    int64
		Skipped   int64
		Average   *float64
	}{}
	if err := results.Select(`COALESCE(SUM(cr.status = 'evaluated'), 0) AS evaluated,
		COALESCE(SUM(cr.status = 'evaluated' AND cr.score IS NOT NULL), 0) AS scored,
		COALESCE(SUM(cr.status = 'skipped'), 0) AS skipped,
		AVG(CASE WHEN cr.status = 'evaluated' AND cr.score IS NOT NULL THEN cr.score END) AS average`).
		Scan(&row).Error; err != nil {
		return Metrics{}, err
	}
	result.EvaluatedCaseCount = row.Evaluated
	result.ScoredCaseCount = row.Scored
	result.SkippedCaseCount = row.Skipped
	result.AverageScore = row.Average

	evaluationBadcases := repository.applyBadcaseFilters(repository.badcaseBase(ctx), filters).
		Where("b.source_type = 'evaluation' AND er.status = 'completed'")
	if err := evaluationBadcases.Distinct("b.id").Count(&result.EvaluationBadcaseCount).Error; err != nil {
		return Metrics{}, err
	}
	if result.EvaluatedCaseCount > 0 {
		result.EvaluationBadcaseRate = ratio(
			result.EvaluationBadcaseCount, result.EvaluatedCaseCount,
		)
	}
	allBadcases := repository.applyBadcaseFilters(repository.badcaseBase(ctx), filters)
	if err := allBadcases.Distinct("b.id").Count(&result.ValidBadcaseCount).Error; err != nil {
		return Metrics{}, err
	}
	return result, nil
}

func (repository Repository) scoreDistribution(ctx context.Context, filters Filters) ([]DistributionItem, error) {
	type scoreRow struct {
		Score uint8 `gorm:"column:score"`
		Count int64 `gorm:"column:item_count"`
	}
	var rows []scoreRow
	query := repository.applyEvaluationFilters(repository.evaluationBase(ctx), filters).
		Joins("JOIN case_results cr ON cr.evaluation_run_id = er.id").
		Joins("JOIN version_cases vc_score ON vc_score.id = cr.version_case_id").
		Where("cr.status = 'evaluated' AND cr.score IS NOT NULL").
		Select("cr.score, COUNT(*) AS item_count").Group("cr.score").Order("cr.score")
	query = repository.applyResultScenarioFilter(query, filters, "vc_score")
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := map[uint8]int64{}
	for _, row := range rows {
		counts[row.Score] = row.Count
	}
	items := make([]DistributionItem, 0, 5)
	for score := uint8(1); score <= 5; score++ {
		items = append(items, DistributionItem{
			Key: fmt.Sprintf("%d", score), Label: fmt.Sprintf("%d 分", score), Count: counts[score],
		})
	}
	return items, nil
}

func (repository Repository) skipDistribution(ctx context.Context, filters Filters) ([]DistributionItem, error) {
	var items []DistributionItem
	query := repository.applyEvaluationFilters(repository.evaluationBase(ctx), filters).
		Joins("JOIN case_results cr ON cr.evaluation_run_id = er.id").
		Joins("JOIN version_cases vc_skip ON vc_skip.id = cr.version_case_id").
		Where("cr.status = 'skipped'").
		Select("cr.skip_reason AS item_key, cr.skip_reason AS item_label, COUNT(*) AS item_count").
		Group("cr.skip_reason").Order("item_count DESC, item_label")
	query = repository.applyResultScenarioFilter(query, filters, "vc_skip")
	if err := query.Scan(&items).Error; err != nil {
		return nil, err
	}
	return nonNilDistribution(items), nil
}

func (repository Repository) badcaseDistribution(
	ctx context.Context,
	filters Filters,
	dimension string,
) ([]DistributionItem, error) {
	query := repository.applyBadcaseFilters(repository.badcaseBase(ctx), filters)
	var items []DistributionItem
	switch dimension {
	case "tag":
		query = query.Joins("JOIN badcase_issue_tags bit_dist ON bit_dist.badcase_id = b.id").
			Joins("JOIN issue_tags it_dist ON it_dist.id = bit_dist.issue_tag_id").
			Select("BIN_TO_UUID(it_dist.id) AS item_key, it_dist.name AS item_label, COUNT(DISTINCT b.id) AS item_count").
			Group("it_dist.id, it_dist.name").Order("item_count DESC, item_label")
	case "status":
		query = query.Select(`b.status AS item_key,
			CASE b.status WHEN 'pending' THEN '待处理' WHEN 'processing' THEN '处理中'
			WHEN 'resolved' THEN '已解决' ELSE '暂不处理' END AS item_label,
			COUNT(DISTINCT b.id) AS item_count`).
			Group("b.status").Order("item_count DESC")
	case "source":
		query = query.Select(`b.source_type AS item_key,
			CASE b.source_type WHEN 'evaluation' THEN '评测发现' ELSE '业务登记' END AS item_label,
			COUNT(DISTINCT b.id) AS item_count`).
			Group("b.source_type").Order("item_count DESC")
	default:
		return nil, fmt.Errorf("unsupported distribution: %s", dimension)
	}
	if err := query.Scan(&items).Error; err != nil {
		return nil, err
	}
	return nonNilDistribution(items), nil
}

func (repository Repository) versionComparison(
	ctx context.Context,
	filters Filters,
) ([]VersionComparison, error) {
	if filters.DatasetID == nil {
		return []VersionComparison{}, nil
	}
	type row struct {
		VersionID id.UUID  `gorm:"column:version_id"`
		VersionNo uint32   `gorm:"column:version_no"`
		RunCount  int64    `gorm:"column:run_count"`
		Evaluated int64    `gorm:"column:evaluated_count"`
		Average   *float64 `gorm:"column:average_score"`
		Badcases  int64    `gorm:"column:badcase_count"`
	}
	var rows []row
	query := repository.applyEvaluationFilters(repository.evaluationBase(ctx), filters).
		Joins("LEFT JOIN case_results cr ON cr.evaluation_run_id = er.id").
		Joins("LEFT JOIN version_cases vc_cmp ON vc_cmp.id = cr.version_case_id").
		Select(`dv.id AS version_id, dv.version_no,
			COUNT(DISTINCT er.id) AS run_count,
			COALESCE(SUM(cr.status = 'evaluated'), 0) AS evaluated_count,
			AVG(CASE WHEN cr.status = 'evaluated' AND cr.score IS NOT NULL THEN cr.score END) AS average_score,
			COUNT(DISTINCT CASE WHEN cr.status = 'evaluated' AND bcmp.id IS NOT NULL
				AND bcmp.invalidated_at IS NULL THEN bcmp.id END) AS badcase_count`).
		Joins("LEFT JOIN badcases bcmp ON bcmp.case_result_id = cr.id").
		Group("dv.id, dv.version_no").Order("dv.version_no")
	query = repository.applyResultScenarioFilter(query, filters, "vc_cmp")
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]VersionComparison, 0, len(rows))
	for _, row := range rows {
		items = append(items, VersionComparison{
			VersionID: row.VersionID.String(), VersionNo: row.VersionNo,
			CompletedRunCount: row.RunCount, EvaluatedCaseCount: row.Evaluated,
			AverageScore: row.Average, EvaluationBadcaseCount: row.Badcases,
			EvaluationBadcaseRate: ratio(row.Badcases, row.Evaluated),
		})
	}
	return items, nil
}

func ratio(numerator, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := float64(numerator) / float64(denominator)
	return &value
}

func (repository Repository) evaluationBase(ctx context.Context) *gorm.DB {
	return repository.db.WithContext(ctx).Table("evaluation_runs er").
		Joins("JOIN dataset_versions dv ON dv.id = er.dataset_version_id").
		Joins("JOIN datasets d ON d.id = dv.dataset_id").
		Joins("JOIN evaluation_targets t ON t.id = d.evaluation_target_id").
		Joins("JOIN users evaluator ON evaluator.id = er.evaluator_id").
		Where("er.status = 'completed'")
}

func (repository Repository) applyEvaluationFilters(query *gorm.DB, filters Filters) *gorm.DB {
	if filters.TargetID != nil {
		query = query.Where("t.id = ?", *filters.TargetID)
	}
	if filters.ScenarioID != nil {
		query = query.Where(`EXISTS (
			SELECT 1 FROM case_results cr_filter
			JOIN version_cases vc_filter ON vc_filter.id = cr_filter.version_case_id
			WHERE cr_filter.evaluation_run_id = er.id AND vc_filter.scenario_id = ?
		)`, *filters.ScenarioID)
	}
	if filters.DatasetID != nil {
		query = query.Where("d.id = ?", *filters.DatasetID)
	}
	if filters.DatasetVersionID != nil {
		query = query.Where("dv.id = ?", *filters.DatasetVersionID)
	}
	if filters.EvaluatorID != nil {
		query = query.Where("er.evaluator_id = ?", *filters.EvaluatorID)
	}
	if filters.AgentVersion != "" {
		query = query.Where("er.agent_version = ?", filters.AgentVersion)
	}
	if filters.Environment != "" {
		query = query.Where("er.environment = ?", filters.Environment)
	}
	if filters.From != nil {
		query = query.Where("er.completed_at >= ?", *filters.From)
	}
	if filters.To != nil {
		query = query.Where("er.completed_at <= ?", *filters.To)
	}
	return query
}

func (repository Repository) applyResultScenarioFilter(
	query *gorm.DB,
	filters Filters,
	caseAlias string,
) *gorm.DB {
	if filters.ScenarioID == nil {
		return query
	}
	return query.Where(caseAlias+".scenario_id = ?", *filters.ScenarioID)
}

func (repository Repository) badcaseBase(ctx context.Context) *gorm.DB {
	return repository.db.WithContext(ctx).Table("badcases b").
		Joins("LEFT JOIN scenarios s ON s.id = b.scenario_id").
		Joins("JOIN evaluation_targets t ON t.id = b.evaluation_target_id").
		Joins("LEFT JOIN case_results crb ON crb.id = b.case_result_id").
		Joins("LEFT JOIN evaluation_runs er ON er.id = crb.evaluation_run_id").
		Joins("LEFT JOIN dataset_versions dv ON dv.id = er.dataset_version_id").
		Joins("LEFT JOIN datasets d ON d.id = dv.dataset_id").
		Where("b.invalidated_at IS NULL")
}

func (repository Repository) applyBadcaseFilters(query *gorm.DB, filters Filters) *gorm.DB {
	if filters.TargetID != nil {
		query = query.Where("t.id = ?", *filters.TargetID)
	}
	if filters.ScenarioID != nil {
		query = query.Where("s.id = ?", *filters.ScenarioID)
	}
	if filters.DatasetID != nil {
		query = query.Where("d.id = ?", *filters.DatasetID)
	}
	if filters.DatasetVersionID != nil {
		query = query.Where("dv.id = ?", *filters.DatasetVersionID)
	}
	if filters.EvaluatorID != nil {
		query = query.Where("er.evaluator_id = ?", *filters.EvaluatorID)
	}
	if filters.AgentVersion != "" {
		query = query.Where("b.agent_version = ?", filters.AgentVersion)
	}
	if filters.Environment != "" {
		query = query.Where("b.environment = ?", filters.Environment)
	}
	if filters.SourceType != "" {
		query = query.Where("b.source_type = ?", filters.SourceType)
	}
	if filters.BadcaseStatus != "" {
		query = query.Where("b.status = ?", filters.BadcaseStatus)
	}
	if filters.IssueTagID != nil {
		query = query.Where(`EXISTS (
			SELECT 1 FROM badcase_issue_tags bit_filter
			WHERE bit_filter.badcase_id = b.id AND bit_filter.issue_tag_id = ?
		)`, *filters.IssueTagID)
	}
	if filters.From != nil {
		query = query.Where("b.occurred_at >= ?", *filters.From)
	}
	if filters.To != nil {
		query = query.Where("b.occurred_at <= ?", *filters.To)
	}
	return query
}

func nonNilDistribution(items []DistributionItem) []DistributionItem {
	if items == nil {
		return []DistributionItem{}
	}
	return items
}

func (repository Repository) options(ctx context.Context) (DashboardOptions, error) {
	type row struct {
		ID       id.UUID  `gorm:"column:id"`
		Name     string   `gorm:"column:name"`
		ParentID *id.UUID `gorm:"column:parent_id"`
	}
	load := func(table, selectClause, where, order string) ([]Option, error) {
		var rows []row
		query := repository.db.WithContext(ctx).Table(table).Select(selectClause)
		if where != "" {
			query = query.Where(where)
		}
		if err := query.Order(order).Scan(&rows).Error; err != nil {
			return nil, err
		}
		items := make([]Option, 0, len(rows))
		for _, item := range rows {
			var parent *string
			if item.ParentID != nil {
				value := item.ParentID.String()
				parent = &value
			}
			items = append(items, Option{ID: item.ID.String(), Name: item.Name, ParentID: parent})
		}
		return items, nil
	}
	var result DashboardOptions
	var err error
	result.Targets, err = load("evaluation_targets", "id, name, NULL AS parent_id", "", "name")
	if err != nil {
		return DashboardOptions{}, err
	}
	result.Scenarios, err = load("scenarios", "id, name, evaluation_target_id AS parent_id", "", "name")
	if err != nil {
		return DashboardOptions{}, err
	}
	result.Datasets, err = load("datasets", "id, name, evaluation_target_id AS parent_id", "", "name")
	if err != nil {
		return DashboardOptions{}, err
	}
	result.Versions, err = load(
		"dataset_versions dv JOIN datasets d ON d.id = dv.dataset_id",
		"dv.id, CONCAT(d.name, ' V', dv.version_no) AS name, dv.dataset_id AS parent_id",
		"dv.version_no IS NOT NULL",
		"d.name, dv.version_no",
	)
	if err != nil {
		return DashboardOptions{}, err
	}
	result.Evaluators, err = load(
		"users", "id, display_name AS name, NULL AS parent_id", "",
		"display_name",
	)
	if err != nil {
		return DashboardOptions{}, err
	}
	result.IssueTags, err = load("issue_tags", "id, name, NULL AS parent_id", "", "sort_order, name")
	if err != nil {
		return DashboardOptions{}, err
	}
	if err := repository.db.WithContext(ctx).Table("evaluation_runs").
		Distinct("agent_version").Where("agent_version <> ''").
		Order("agent_version").Pluck("agent_version", &result.AgentVersions).Error; err != nil {
		return DashboardOptions{}, err
	}
	if result.AgentVersions == nil {
		result.AgentVersions = []string{}
	}
	return result, nil
}

func (repository Repository) Search(
	ctx context.Context,
	keyword string,
	types []string,
	page, pageSize int,
) (SearchResult, error) {
	type rawItem struct {
		Type     string   `gorm:"column:item_type"`
		ID       id.UUID  `gorm:"column:item_id"`
		ParentID *id.UUID `gorm:"column:parent_id"`
		Title    string   `gorm:"column:title"`
		Subtitle string   `gorm:"column:subtitle"`
		Snippet  string   `gorm:"column:snippet"`
	}
	like := "%" + escapeLike(keyword) + "%"
	parts := []string{}
	args := []any{}
	enabled := map[string]bool{}
	for _, value := range types {
		enabled[value] = true
	}
	if enabled["target"] {
		parts = append(parts, `SELECT 'target' AS item_type, t.id AS item_id, NULL AS parent_id,
			t.name AS title, '评测对象' AS subtitle, COALESCE(t.description, '') AS snippet
			FROM evaluation_targets t
			WHERE t.name LIKE ? OR t.description LIKE ?`)
		args = append(args, like, like)
	}
	if enabled["scenario"] {
		parts = append(parts, `SELECT 'scenario' AS item_type, s.id AS item_id, NULL AS parent_id,
			s.name AS title, t.name AS subtitle, COALESCE(s.description, '') AS snippet
			FROM scenarios s JOIN evaluation_targets t ON t.id = s.evaluation_target_id
			WHERE s.name LIKE ? OR s.description LIKE ? OR t.name LIKE ?`)
		args = append(args, like, like, like)
	}
	if enabled["dataset"] {
		parts = append(parts, `SELECT 'dataset' AS item_type, d.id AS item_id, NULL AS parent_id,
			d.name AS title, t.name AS subtitle,
			COALESCE(d.description, '') AS snippet
			FROM datasets d JOIN evaluation_targets t ON t.id = d.evaluation_target_id
			WHERE d.name LIKE ? OR d.description LIKE ? OR t.name LIKE ?`)
		args = append(args, like, like, like)
	}
	if enabled["case"] {
		parts = append(parts, `SELECT 'case' AS item_type, vc.id AS item_id, d.id AS parent_id,
			COALESCE(vc.name, LEFT(vc.user_prompt, 80)) AS title,
			CONCAT(d.name, IF(dv.version_no IS NULL, ' 草稿', CONCAT(' V', dv.version_no))) AS subtitle,
			vc.user_prompt AS snippet
			FROM version_cases vc JOIN dataset_versions dv ON dv.id = vc.dataset_version_id
			JOIN datasets d ON d.id = dv.dataset_id
			WHERE vc.name LIKE ? OR vc.user_prompt LIKE ? OR vc.expected_result LIKE ?
				OR vc.judging_guide LIKE ?`)
		args = append(args, like, like, like, like)
	}
	if enabled["badcase"] {
		parts = append(parts, `SELECT 'badcase' AS item_type, b.id AS item_id, NULL AS parent_id,
			b.title AS title, CONCAT(t.name, ' / ', COALESCE(s.name, '待归类')) AS subtitle,
			COALESCE(b.description, b.agent_response_text, b.business_reference, b.session_id, '') AS snippet
			FROM badcases b LEFT JOIN scenarios s ON s.id = b.scenario_id
			JOIN evaluation_targets t ON t.id = b.evaluation_target_id
			WHERE b.title LIKE ? OR b.description LIKE ? OR b.agent_response_text LIKE ?
				OR b.business_reference LIKE ? OR b.session_id LIKE ?
				OR EXISTS (SELECT 1 FROM badcase_activities ba
					WHERE ba.badcase_id = b.id AND ba.note LIKE ?)`)
		args = append(args, like, like, like, like, like, like)
	}
	if enabled["evaluation_result"] {
		parts = append(parts, `SELECT 'evaluation_result' AS item_type, cr.id AS item_id,
			er.id AS parent_id, COALESCE(vc.name, LEFT(vc.user_prompt, 80)) AS title,
			CONCAT(d.name, ' V', dv.version_no, ' / ', evaluator.display_name) AS subtitle,
			cr.answer_text AS snippet
			FROM case_results cr
			JOIN evaluation_runs er ON er.id = cr.evaluation_run_id
			JOIN users evaluator ON evaluator.id = er.evaluator_id
			JOIN version_cases vc ON vc.id = cr.version_case_id
			JOIN dataset_versions dv ON dv.id = er.dataset_version_id
			JOIN datasets d ON d.id = dv.dataset_id
			WHERE cr.status = 'evaluated' AND cr.answer_text IS NOT NULL
				AND cr.answer_text LIKE ?`)
		args = append(args, like)
	}
	if len(parts) == 0 {
		return SearchResult{Items: []SearchItem{}, Page: page, PageSize: pageSize}, nil
	}
	union := strings.Join(parts, " UNION ALL ")
	var total int64
	if err := repository.db.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM ("+union+") search_count", args...,
	).Scan(&total).Error; err != nil {
		return SearchResult{}, err
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	var rows []rawItem
	if err := repository.db.WithContext(ctx).Raw(
		"SELECT * FROM ("+union+") search_items ORDER BY item_type, title LIMIT ? OFFSET ?",
		queryArgs...,
	).Scan(&rows).Error; err != nil {
		return SearchResult{}, err
	}
	items := make([]SearchItem, 0, len(rows))
	for _, row := range rows {
		url := ""
		switch row.Type {
		case "target":
			url = "/dashboard?evaluation_target_id=" + row.ID.String()
		case "scenario":
			url = "/datasets?scenario_id=" + row.ID.String()
		case "dataset":
			url = "/datasets/" + row.ID.String()
		case "case":
			url = "/version-cases/" + row.ID.String()
		case "badcase":
			url = "/badcases/" + row.ID.String()
		case "evaluation_result":
			if row.ParentID != nil {
				url = "/evaluation-runs/" + row.ParentID.String() +
					"/result?result_id=" + row.ID.String()
			}
		}
		items = append(items, SearchItem{
			Type: row.Type, ID: row.ID.String(), Title: row.Title,
			Subtitle: row.Subtitle, Snippet: truncate(row.Snippet, 160), URL: url,
		})
	}
	return SearchResult{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func truncate(value string, size int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= size {
		return value
	}
	return string(runes[:size]) + "…"
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
