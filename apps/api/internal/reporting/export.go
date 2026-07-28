package reporting

import (
	"context"
	"time"
)

type EvaluationExportRow struct {
	RunID         string     `gorm:"column:run_id"`
	DatasetName   string     `gorm:"column:dataset_name"`
	VersionNo     uint32     `gorm:"column:version_no"`
	TargetName    string     `gorm:"column:target_name"`
	ScenarioName  string     `gorm:"column:scenario_name"`
	EvaluatorName string     `gorm:"column:evaluator_name"`
	AgentVersion  string     `gorm:"column:agent_version"`
	Environment   string     `gorm:"column:environment"`
	CompletedAt   *time.Time `gorm:"column:completed_at"`
	ResultID      string     `gorm:"column:result_id"`
	CaseName      *string    `gorm:"column:case_name"`
	UserPrompt    string     `gorm:"column:user_prompt"`
	ResultStatus  string     `gorm:"column:result_status"`
	AnswerText    *string    `gorm:"column:answer_text"`
	Score         *uint8     `gorm:"column:score"`
	Comment       *string    `gorm:"column:comment"`
	SkipReason    *string    `gorm:"column:skip_reason"`
	HasBadcase    bool       `gorm:"column:has_badcase"`
	BadcaseID     *string    `gorm:"column:badcase_id"`
	BadcaseTitle  *string    `gorm:"column:badcase_title"`
	CaseTags      string     `gorm:"column:case_tags"`
	EvidenceURLs  string     `gorm:"column:evidence_urls"`
}

type BadcaseExportRow struct {
	ID                       string    `gorm:"column:id"`
	SourceType               string    `gorm:"column:source_type"`
	TargetName               string    `gorm:"column:target_name"`
	ScenarioName             string    `gorm:"column:scenario_name"`
	Title                    string    `gorm:"column:title"`
	Description              *string   `gorm:"column:description"`
	AgentResponseText        *string   `gorm:"column:agent_response_text"`
	AgentVersion             *string   `gorm:"column:agent_version"`
	Environment              string    `gorm:"column:environment"`
	OccurredAt               time.Time `gorm:"column:occurred_at"`
	Status                   string    `gorm:"column:status"`
	AssigneeName             *string   `gorm:"column:assignee_name"`
	CreatorName              string    `gorm:"column:creator_name"`
	BusinessReference        *string   `gorm:"column:business_reference"`
	SessionID                *string   `gorm:"column:session_id"`
	IssueTags                string    `gorm:"column:issue_tags"`
	OriginalEvidenceURLs     string    `gorm:"column:original_evidence_urls"`
	SupplementalEvidenceURLs string    `gorm:"column:supplemental_evidence_urls"`
	CreatedAt                time.Time `gorm:"column:created_at"`
	UpdatedAt                time.Time `gorm:"column:updated_at"`
}

func (repository Repository) EvaluationExportRows(
	ctx context.Context,
	filters Filters,
) ([]EvaluationExportRow, error) {
	query := repository.applyEvaluationFilters(repository.evaluationBase(ctx), filters).
		Joins("JOIN case_results cr ON cr.evaluation_run_id = er.id").
		Joins("JOIN version_cases vc ON vc.id = cr.version_case_id").
		Joins("LEFT JOIN badcases b ON b.case_result_id = cr.id AND b.invalidated_at IS NULL").
		Select(`BIN_TO_UUID(er.id) AS run_id, d.name AS dataset_name, dv.version_no,
			t.name AS target_name, s.name AS scenario_name, evaluator.display_name AS evaluator_name,
			er.agent_version, er.environment, er.completed_at,
			BIN_TO_UUID(cr.id) AS result_id, vc.name AS case_name, vc.user_prompt,
			cr.status AS result_status, cr.answer_text, cr.score, cr.comment, cr.skip_reason,
			b.id IS NOT NULL AS has_badcase, BIN_TO_UUID(b.id) AS badcase_id, b.title AS badcase_title,
			COALESCE((SELECT GROUP_CONCAT(vct.tag_name_snapshot ORDER BY vct.tag_name_snapshot SEPARATOR '；')
				FROM version_case_tags vct WHERE vct.version_case_id = vc.id), '') AS case_tags,
			COALESCE((SELECT GROUP_CONCAT(CONCAT('/api/v1/attachments/', BIN_TO_UUID(a.id), '/content')
				ORDER BY a.sort_order SEPARATOR '；') FROM attachments a
				WHERE a.case_result_id = cr.id), '') AS evidence_urls`).
		Order("er.completed_at DESC, er.id, cr.id").Limit(ExportLimit + 1)
	var rows []EvaluationExportRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (repository Repository) BadcaseExportRows(
	ctx context.Context,
	filters Filters,
) ([]BadcaseExportRow, error) {
	query := repository.applyBadcaseFilters(repository.badcaseBase(ctx), filters).
		Joins("JOIN users creator ON creator.id = b.created_by").
		Joins("LEFT JOIN users assignee ON assignee.id = b.assignee_id").
		Select(`BIN_TO_UUID(b.id) AS id, b.source_type, t.name AS target_name,
			s.name AS scenario_name, b.title, b.description, b.agent_response_text,
			b.agent_version, b.environment, b.occurred_at, b.status,
			assignee.display_name AS assignee_name, creator.display_name AS creator_name,
			b.business_reference, b.session_id, b.created_at, b.updated_at,
			COALESCE((SELECT GROUP_CONCAT(it.name ORDER BY it.sort_order, it.name SEPARATOR '；')
				FROM badcase_issue_tags bit JOIN issue_tags it ON it.id = bit.issue_tag_id
				WHERE bit.badcase_id = b.id), '') AS issue_tags,
			COALESCE((SELECT GROUP_CONCAT(CONCAT('/api/v1/attachments/', BIN_TO_UUID(a.id), '/content')
				ORDER BY a.sort_order SEPARATOR '；') FROM attachments a
				WHERE a.case_result_id = b.case_result_id), '') AS original_evidence_urls,
			COALESCE((SELECT GROUP_CONCAT(CONCAT('/api/v1/attachments/', BIN_TO_UUID(a.id), '/content')
				ORDER BY a.sort_order SEPARATOR '；') FROM attachments a
				WHERE a.badcase_id = b.id), '') AS supplemental_evidence_urls`).
		Order("b.occurred_at DESC, b.id").Limit(ExportLimit + 1)
	var rows []BadcaseExportRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (repository Repository) DistributionExportRows(
	ctx context.Context,
	filters Filters,
) ([]DistributionItem, error) {
	type dimension struct {
		name  string
		items []DistributionItem
	}
	dimensions := []dimension{}
	for _, name := range []string{"tag", "status", "source"} {
		items, err := repository.badcaseDistribution(ctx, filters, name)
		if err != nil {
			return nil, err
		}
		dimensions = append(dimensions, dimension{name: name, items: items})
	}
	rows := []DistributionItem{}
	for _, group := range dimensions {
		for _, item := range group.items {
			rows = append(rows, DistributionItem{
				Key:   group.name + ":" + item.Key,
				Label: item.Label,
				Count: item.Count,
			})
		}
	}
	return rows, nil
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
