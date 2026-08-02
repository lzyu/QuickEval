package evaluation

import (
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/attachment"
	"github.com/lzyu/QuickEval/apps/api/internal/dataset"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

const (
	RunInProgress = "in_progress"
	RunCompleted  = "completed"
	RunVoided     = "voided"

	ResultPending   = "pending"
	ResultEvaluated = "evaluated"
	ResultSkipped   = "skipped"
)

type Run struct {
	ID               id.UUID    `gorm:"column:id;type:binary(16);primaryKey"`
	DatasetVersionID id.UUID    `gorm:"column:dataset_version_id;type:binary(16)"`
	EvaluatorID      id.UUID    `gorm:"column:evaluator_id;type:binary(16)"`
	AgentVersion     string     `gorm:"column:agent_version"`
	Environment      string     `gorm:"column:environment"`
	PurposeNote      *string    `gorm:"column:purpose_note"`
	ConfigNote       *string    `gorm:"column:config_note"`
	Status           string     `gorm:"column:status"`
	LockVersion      uint32     `gorm:"column:lock_version"`
	FirstCompletedAt *time.Time `gorm:"column:first_completed_at"`
	CompletedAt      *time.Time `gorm:"column:completed_at"`
	VoidedAt         *time.Time `gorm:"column:voided_at"`
	VoidedBy         *id.UUID   `gorm:"column:voided_by;type:binary(16)"`
	VoidReason       *string    `gorm:"column:void_reason"`
	UpdatedBy        id.UUID    `gorm:"column:updated_by;type:binary(16)"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`

	DatasetID      id.UUID  `gorm:"column:dataset_id;type:binary(16);->"`
	DatasetName    string   `gorm:"column:dataset_name;->"`
	VersionNo      uint32   `gorm:"column:version_no;->"`
	TargetID       id.UUID  `gorm:"column:evaluation_target_id;type:binary(16);->"`
	TargetName     string   `gorm:"column:evaluation_target_name;->"`
	EvaluatorName  string   `gorm:"column:evaluator_name;->"`
	TotalCount     int64    `gorm:"column:total_count;->"`
	PendingCount   int64    `gorm:"column:pending_count;->"`
	EvaluatedCount int64    `gorm:"column:evaluated_count;->"`
	SkippedCount   int64    `gorm:"column:skipped_count;->"`
	ScoredCount    int64    `gorm:"column:scored_count;->"`
	BadcaseCount   int64    `gorm:"column:badcase_count;->"`
	AverageScore   *float64 `gorm:"column:average_score;->"`
}

func (Run) TableName() string { return "evaluation_runs" }

type Result struct {
	ID              id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	EvaluationRunID id.UUID   `gorm:"column:evaluation_run_id;type:binary(16)"`
	VersionCaseID   id.UUID   `gorm:"column:version_case_id;type:binary(16)"`
	Status          string    `gorm:"column:status"`
	AnswerText      *string   `gorm:"column:answer_text"`
	Score           *uint8    `gorm:"column:score"`
	Comment         *string   `gorm:"column:comment"`
	SkipReason      *string   `gorm:"column:skip_reason"`
	LockVersion     uint32    `gorm:"column:lock_version"`
	UpdatedBy       id.UUID   `gorm:"column:updated_by;type:binary(16)"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`

	CaseKey          id.UUID             `gorm:"column:case_key;type:binary(16);->"`
	Name             *string             `gorm:"column:case_name;->"`
	UserPrompt       string              `gorm:"column:user_prompt;->"`
	Precondition     *string             `gorm:"column:precondition;->"`
	ExpectedResult   *string             `gorm:"column:expected_result;->"`
	JudgingGuide     *string             `gorm:"column:judging_guide;->"`
	SortOrder        uint32              `gorm:"column:sort_order;->"`
	ScenarioID       *id.UUID            `gorm:"column:scenario_id;type:binary(16);->"`
	ScenarioName     *string             `gorm:"column:scenario_name;->"`
	AssignmentStatus string              `gorm:"column:scenario_assignment_status;->"`
	Tags             []dataset.CaseTag   `gorm:"-"`
	HasBadcase       bool                `gorm:"column:has_badcase;->"`
	Attachments      []attachment.Public `gorm:"-"`
	Badcase          *BadcaseSummary     `gorm:"-"`
}

func (Result) TableName() string { return "case_results" }

type RunPublic struct {
	ID               string     `json:"id"`
	DatasetVersionID string     `json:"dataset_version_id"`
	DatasetID        string     `json:"dataset_id"`
	DatasetName      string     `json:"dataset_name"`
	VersionNo        uint32     `json:"version_no"`
	TargetID         string     `json:"evaluation_target_id"`
	TargetName       string     `json:"evaluation_target_name"`
	EvaluatorID      string     `json:"evaluator_id"`
	EvaluatorName    string     `json:"evaluator_name"`
	AgentVersion     string     `json:"agent_version"`
	Environment      string     `json:"environment"`
	PurposeNote      *string    `json:"purpose_note"`
	ConfigNote       *string    `json:"config_note"`
	Status           string     `json:"status"`
	LockVersion      uint32     `json:"lock_version"`
	FirstCompletedAt *time.Time `json:"first_completed_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	VoidedAt         *time.Time `json:"voided_at"`
	VoidReason       *string    `json:"void_reason"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Progress         Progress   `json:"progress"`
}

type Progress struct {
	TotalCount     int64    `json:"total_count"`
	PendingCount   int64    `json:"pending_count"`
	EvaluatedCount int64    `json:"evaluated_count"`
	SkippedCount   int64    `json:"skipped_count"`
	ScoredCount    int64    `json:"scored_count"`
	BadcaseCount   int64    `json:"badcase_count"`
	AverageScore   *float64 `json:"average_score"`
	CompletionRate float64  `json:"completion_rate"`
}

func (item Run) Public() RunPublic {
	rate := float64(0)
	if item.TotalCount > 0 {
		rate = float64(item.EvaluatedCount+item.SkippedCount) / float64(item.TotalCount)
	}
	return RunPublic{
		ID: item.ID.String(), DatasetVersionID: item.DatasetVersionID.String(),
		DatasetID: item.DatasetID.String(), DatasetName: item.DatasetName, VersionNo: item.VersionNo,
		TargetID: item.TargetID.String(), TargetName: item.TargetName,
		EvaluatorID: item.EvaluatorID.String(), EvaluatorName: item.EvaluatorName,
		AgentVersion: item.AgentVersion, Environment: item.Environment,
		PurposeNote: item.PurposeNote, ConfigNote: item.ConfigNote,
		Status: item.Status, LockVersion: item.LockVersion,
		FirstCompletedAt: item.FirstCompletedAt, CompletedAt: item.CompletedAt,
		VoidedAt: item.VoidedAt, VoidReason: item.VoidReason,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		Progress: Progress{
			TotalCount: item.TotalCount, PendingCount: item.PendingCount,
			EvaluatedCount: item.EvaluatedCount, SkippedCount: item.SkippedCount,
			ScoredCount: item.ScoredCount, BadcaseCount: item.BadcaseCount,
			AverageScore: item.AverageScore, CompletionRate: rate,
		},
	}
}

type ResultPublic struct {
	ID               string              `json:"id"`
	EvaluationRunID  string              `json:"evaluation_run_id"`
	VersionCaseID    string              `json:"version_case_id"`
	CaseKey          string              `json:"case_key"`
	Name             *string             `json:"name"`
	UserPrompt       string              `json:"user_prompt"`
	Precondition     *string             `json:"precondition"`
	ExpectedResult   *string             `json:"expected_result"`
	JudgingGuide     *string             `json:"judging_guide"`
	SortOrder        uint32              `json:"sort_order"`
	ScenarioID       *string             `json:"scenario_id"`
	ScenarioName     *string             `json:"scenario_name"`
	AssignmentStatus string              `json:"scenario_assignment_status"`
	Tags             []dataset.CaseTag   `json:"tags"`
	Status           string              `json:"status"`
	AnswerText       *string             `json:"answer_text"`
	Score            *uint8              `json:"score"`
	Comment          *string             `json:"comment"`
	SkipReason       *string             `json:"skip_reason"`
	HasBadcase       bool                `json:"has_badcase"`
	Attachments      []attachment.Public `json:"attachments"`
	Badcase          *BadcaseSummary     `json:"badcase"`
	LockVersion      uint32              `json:"lock_version"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

func (item Result) Public() ResultPublic {
	if item.Tags == nil {
		item.Tags = []dataset.CaseTag{}
	}
	var scenarioID *string
	if item.ScenarioID != nil {
		value := item.ScenarioID.String()
		scenarioID = &value
	}
	return ResultPublic{
		ID: item.ID.String(), EvaluationRunID: item.EvaluationRunID.String(),
		VersionCaseID: item.VersionCaseID.String(), CaseKey: item.CaseKey.String(),
		Name: item.Name, UserPrompt: item.UserPrompt, Precondition: item.Precondition,
		ExpectedResult: item.ExpectedResult, JudgingGuide: item.JudgingGuide,
		SortOrder: item.SortOrder, ScenarioID: scenarioID, ScenarioName: item.ScenarioName,
		AssignmentStatus: item.AssignmentStatus, Tags: item.Tags, Status: item.Status,
		AnswerText: item.AnswerText, Score: item.Score, Comment: item.Comment,
		SkipReason: item.SkipReason, HasBadcase: item.HasBadcase,
		Attachments: item.Attachments, Badcase: item.Badcase,
		LockVersion: item.LockVersion, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

type BadcaseSummary struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description *string           `json:"description"`
	Status      string            `json:"status"`
	IssueTags   []dataset.CaseTag `json:"issue_tags"`
}
