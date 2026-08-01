package reporting

import (
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

const ExportLimit = 50000

type Filters struct {
	TargetID         *id.UUID
	ScenarioID       *id.UUID
	DatasetID        *id.UUID
	DatasetVersionID *id.UUID
	EvaluatorID      *id.UUID
	AgentVersion     string
	Environment      string
	SourceType       string
	BadcaseStatus    string
	IssueTagID       *id.UUID
	From             *time.Time
	To               *time.Time
}

type EvaluationResultFilters struct {
	Filters
	ResultID     *id.UUID
	ResultStatus string
	Score        *uint8
	SkipReason   string
	HasBadcase   *bool
	Scored       *bool
	Keyword      string
}

type HomeMetric struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value int64  `json:"value"`
	URL   string `json:"url"`
}

type HomeRun struct {
	ID             id.UUID   `gorm:"column:id"`
	DatasetName    string    `gorm:"column:dataset_name"`
	VersionNo      uint32    `gorm:"column:version_no"`
	TargetName     string    `gorm:"column:evaluation_target_name"`
	AgentVersion   string    `gorm:"column:agent_version"`
	Environment    string    `gorm:"column:environment"`
	EvaluatedCount int64     `gorm:"column:evaluated_count"`
	SkippedCount   int64     `gorm:"column:skipped_count"`
	TotalCount     int64     `gorm:"column:total_count"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

type HomeRunPublic struct {
	ID             string    `json:"id"`
	DatasetName    string    `json:"dataset_name"`
	VersionNo      uint32    `json:"version_no"`
	TargetName     string    `json:"evaluation_target_name"`
	AgentVersion   string    `json:"agent_version"`
	Environment    string    `json:"environment"`
	CompletedCount int64     `json:"completed_count"`
	TotalCount     int64     `json:"total_count"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (item HomeRun) Public() HomeRunPublic {
	return HomeRunPublic{
		ID: item.ID.String(), DatasetName: item.DatasetName, VersionNo: item.VersionNo,
		TargetName: item.TargetName, AgentVersion: item.AgentVersion,
		Environment: item.Environment, CompletedCount: item.EvaluatedCount + item.SkippedCount,
		TotalCount: item.TotalCount, UpdatedAt: item.UpdatedAt,
	}
}

type HomeBadcase struct {
	ID           id.UUID   `gorm:"column:id"`
	Title        string    `gorm:"column:title"`
	ScenarioName string    `gorm:"column:scenario_name"`
	Status       string    `gorm:"column:status"`
	SourceType   string    `gorm:"column:source_type"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

type HomeBadcasePublic struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	ScenarioName string    `json:"scenario_name"`
	Status       string    `json:"status"`
	SourceType   string    `json:"source_type"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (item HomeBadcase) Public() HomeBadcasePublic {
	return HomeBadcasePublic{
		ID: item.ID.String(), Title: item.Title, ScenarioName: item.ScenarioName,
		Status: item.Status, SourceType: item.SourceType, UpdatedAt: item.UpdatedAt,
	}
}

type RecentDataset struct {
	DatasetID   id.UUID   `gorm:"column:dataset_id"`
	VersionID   id.UUID   `gorm:"column:version_id"`
	DatasetName string    `gorm:"column:dataset_name"`
	TargetName  string    `gorm:"column:target_name"`
	VersionNo   uint32    `gorm:"column:version_no"`
	CaseCount   int64     `gorm:"column:case_count"`
	PublishedAt time.Time `gorm:"column:published_at"`
}

type RecentDatasetPublic struct {
	DatasetID   string    `json:"dataset_id"`
	VersionID   string    `json:"version_id"`
	DatasetName string    `json:"dataset_name"`
	TargetName  string    `json:"evaluation_target_name"`
	VersionNo   uint32    `json:"version_no"`
	CaseCount   int64     `json:"case_count"`
	PublishedAt time.Time `json:"published_at"`
}

func (item RecentDataset) Public() RecentDatasetPublic {
	return RecentDatasetPublic{
		DatasetID: item.DatasetID.String(), VersionID: item.VersionID.String(),
		DatasetName: item.DatasetName,
		TargetName:  item.TargetName, VersionNo: item.VersionNo,
		CaseCount: item.CaseCount, PublishedAt: item.PublishedAt,
	}
}

type Activity struct {
	ID           id.UUID   `gorm:"column:id"`
	BadcaseID    id.UUID   `gorm:"column:badcase_id"`
	BadcaseTitle string    `gorm:"column:badcase_title"`
	Type         string    `gorm:"column:activity_type"`
	Note         *string   `gorm:"column:note"`
	ActorName    string    `gorm:"column:actor_name"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

type ActivityPublic struct {
	ID           string    `json:"id"`
	BadcaseID    string    `json:"badcase_id"`
	BadcaseTitle string    `json:"badcase_title"`
	Type         string    `json:"activity_type"`
	Note         *string   `json:"note"`
	ActorName    string    `json:"actor_name"`
	CreatedAt    time.Time `json:"created_at"`
}

func (item Activity) Public() ActivityPublic {
	return ActivityPublic{
		ID: item.ID.String(), BadcaseID: item.BadcaseID.String(),
		BadcaseTitle: item.BadcaseTitle, Type: item.Type, Note: item.Note,
		ActorName: item.ActorName, CreatedAt: item.CreatedAt,
	}
}

type Home struct {
	Metrics          []HomeMetric          `json:"metrics"`
	ContinueRuns     []HomeRunPublic       `json:"continue_evaluations"`
	AssignedBadcases []HomeBadcasePublic   `json:"assigned_badcases"`
	RecentDatasets   []RecentDatasetPublic `json:"recent_datasets"`
	RecentActivities []ActivityPublic      `json:"recent_activities"`
}

type Metrics struct {
	CompletedRunCount      int64    `json:"completed_run_count"`
	EvaluatedCaseCount     int64    `json:"evaluated_case_count"`
	ScoredCaseCount        int64    `json:"scored_case_count"`
	AverageScore           *float64 `json:"average_score"`
	EvaluationBadcaseCount int64    `json:"evaluation_badcase_count"`
	EvaluationBadcaseRate  *float64 `json:"evaluation_badcase_rate"`
	ValidBadcaseCount      int64    `json:"valid_badcase_count"`
	SkippedCaseCount       int64    `json:"skipped_case_count"`
}

type DistributionItem struct {
	Key   string `json:"key" gorm:"column:item_key"`
	Label string `json:"label" gorm:"column:item_label"`
	Count int64  `json:"count" gorm:"column:item_count"`
}

type VersionComparison struct {
	VersionID              string   `json:"version_id"`
	VersionNo              uint32   `json:"version_no"`
	CompletedRunCount      int64    `json:"completed_run_count"`
	EvaluatedCaseCount     int64    `json:"evaluated_case_count"`
	AverageScore           *float64 `json:"average_score"`
	EvaluationBadcaseCount int64    `json:"evaluation_badcase_count"`
	EvaluationBadcaseRate  *float64 `json:"evaluation_badcase_rate"`
}

type Option struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id,omitempty"`
}

type DashboardOptions struct {
	Targets       []Option `json:"evaluation_targets"`
	Scenarios     []Option `json:"scenarios"`
	Datasets      []Option `json:"datasets"`
	Versions      []Option `json:"dataset_versions"`
	Evaluators    []Option `json:"evaluators"`
	AgentVersions []string `json:"agent_versions"`
	IssueTags     []Option `json:"issue_tags"`
}

type Dashboard struct {
	Metrics                Metrics             `json:"metrics"`
	ScoreDistribution      []DistributionItem  `json:"score_distribution"`
	IssueTagDistribution   []DistributionItem  `json:"issue_tag_distribution"`
	StatusDistribution     []DistributionItem  `json:"status_distribution"`
	SourceDistribution     []DistributionItem  `json:"source_distribution"`
	SkipReasonDistribution []DistributionItem  `json:"skip_reason_distribution"`
	VersionComparison      []VersionComparison `json:"version_comparison"`
	Options                DashboardOptions    `json:"options"`
}

type EvaluationResultDetail struct {
	RunID           string     `json:"evaluation_run_id" gorm:"column:evaluation_run_id"`
	ResultID        string     `json:"id" gorm:"column:id"`
	TargetName      string     `json:"evaluation_target_name" gorm:"column:evaluation_target_name"`
	ScenarioName    string     `json:"scenario_name" gorm:"column:scenario_name"`
	DatasetName     string     `json:"dataset_name" gorm:"column:dataset_name"`
	VersionNo       uint32     `json:"version_no" gorm:"column:version_no"`
	EvaluatorName   string     `json:"evaluator_name" gorm:"column:evaluator_name"`
	AgentVersion    string     `json:"agent_version" gorm:"column:agent_version"`
	Environment     string     `json:"environment" gorm:"column:environment"`
	CompletedAt     *time.Time `json:"completed_at" gorm:"column:completed_at"`
	CaseName        *string    `json:"case_name" gorm:"column:case_name"`
	UserPrompt      string     `json:"user_prompt" gorm:"column:user_prompt"`
	ResultStatus    string     `json:"result_status" gorm:"column:result_status"`
	AnswerText      *string    `json:"answer_text" gorm:"column:answer_text"`
	Score           *uint8     `json:"score" gorm:"column:score"`
	Comment         *string    `json:"comment" gorm:"column:comment"`
	SkipReason      *string    `json:"skip_reason" gorm:"column:skip_reason"`
	HasBadcase      bool       `json:"has_badcase" gorm:"column:has_badcase"`
	BadcaseID       *string    `json:"badcase_id" gorm:"column:badcase_id"`
	BadcaseTitle    *string    `json:"badcase_title" gorm:"column:badcase_title"`
	CaseTags        string     `json:"case_tags" gorm:"column:case_tags"`
	EvidenceCount   int64      `json:"evidence_count" gorm:"column:evidence_count"`
	ResultDetailURL string     `json:"result_detail_url" gorm:"-"`
}

type SearchItem struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Snippet  string `json:"snippet"`
	URL      string `json:"url"`
}

type SearchResult struct {
	Items    []SearchItem `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Total    int64        `json:"total"`
}
