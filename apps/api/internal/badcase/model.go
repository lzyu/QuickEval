package badcase

import (
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/attachment"
	"github.com/lzyu/QuickEval/apps/api/internal/dataset"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

type Badcase struct {
	ID                id.UUID    `gorm:"column:id;type:binary(16);primaryKey"`
	SourceType        string     `gorm:"column:source_type"`
	TargetID          id.UUID    `gorm:"column:evaluation_target_id;type:binary(16)"`
	ScenarioID        *id.UUID   `gorm:"column:scenario_id;type:binary(16)"`
	AssignmentStatus  string     `gorm:"column:scenario_assignment_status"`
	CaseResultID      *id.UUID   `gorm:"column:case_result_id;type:binary(16)"`
	Title             string     `gorm:"column:title"`
	Description       *string    `gorm:"column:description"`
	AgentResponseText *string    `gorm:"column:agent_response_text"`
	AgentVersion      *string    `gorm:"column:agent_version"`
	Environment       string     `gorm:"column:environment"`
	OccurredAt        time.Time  `gorm:"column:occurred_at"`
	BusinessReference *string    `gorm:"column:business_reference"`
	SessionID         *string    `gorm:"column:session_id"`
	Status            string     `gorm:"column:status"`
	AssigneeID        *id.UUID   `gorm:"column:assignee_id;type:binary(16)"`
	ResolvedAt        *time.Time `gorm:"column:resolved_at"`
	InvalidatedAt     *time.Time `gorm:"column:invalidated_at"`
	InvalidatedBy     *id.UUID   `gorm:"column:invalidated_by;type:binary(16)"`
	InvalidReason     *string    `gorm:"column:invalid_reason"`
	LockVersion       uint32     `gorm:"column:lock_version"`
	CreatedBy         id.UUID    `gorm:"column:created_by;type:binary(16)"`
	UpdatedBy         id.UUID    `gorm:"column:updated_by;type:binary(16)"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`

	ScenarioName        *string             `gorm:"column:scenario_name;->"`
	TargetName          string              `gorm:"column:evaluation_target_name;->"`
	CreatorName         string              `gorm:"column:creator_name;->"`
	AssigneeName        string              `gorm:"column:assignee_name;->"`
	InvalidatorName     string              `gorm:"column:invalidator_name;->"`
	RunID               *id.UUID            `gorm:"column:evaluation_run_id;type:binary(16);->"`
	EvaluatorID         *id.UUID            `gorm:"column:evaluator_id;type:binary(16);->"`
	EvaluatorName       string              `gorm:"column:evaluator_name;->"`
	DatasetID           *id.UUID            `gorm:"column:dataset_id;type:binary(16);->"`
	DatasetName         string              `gorm:"column:dataset_name;->"`
	VersionID           *id.UUID            `gorm:"column:dataset_version_id;type:binary(16);->"`
	VersionNo           *uint32             `gorm:"column:version_no;->"`
	VersionCaseID       *id.UUID            `gorm:"column:version_case_id;type:binary(16);->"`
	CaseName            *string             `gorm:"column:case_name;->"`
	UserPrompt          *string             `gorm:"column:user_prompt;->"`
	Score               *uint8              `gorm:"column:score;->"`
	Comment             *string             `gorm:"column:comment;->"`
	IssueTags           []dataset.CaseTag   `gorm:"-"`
	OriginalAttachments []attachment.Public `gorm:"-"`
	Attachments         []attachment.Public `gorm:"-"`
	Activities          []ActivityPublic    `gorm:"-"`
}

func (Badcase) TableName() string { return "badcases" }

type BadcaseIssueTag struct {
	ID         id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	BadcaseID  id.UUID   `gorm:"column:badcase_id;type:binary(16)"`
	IssueTagID id.UUID   `gorm:"column:issue_tag_id;type:binary(16)"`
	CreatedBy  id.UUID   `gorm:"column:created_by;type:binary(16)"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (BadcaseIssueTag) TableName() string { return "badcase_issue_tags" }

type Activity struct {
	ID             id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	BadcaseID      id.UUID   `gorm:"column:badcase_id;type:binary(16)"`
	Type           string    `gorm:"column:activity_type"`
	FromStatus     *string   `gorm:"column:from_status"`
	ToStatus       *string   `gorm:"column:to_status"`
	FromAssigneeID *id.UUID  `gorm:"column:from_assignee_id;type:binary(16)"`
	ToAssigneeID   *id.UUID  `gorm:"column:to_assignee_id;type:binary(16)"`
	Note           *string   `gorm:"column:note"`
	ActorID        id.UUID   `gorm:"column:actor_id;type:binary(16)"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (Activity) TableName() string { return "badcase_activities" }

type ActivityPublic struct {
	ID               string    `json:"id"`
	Type             string    `json:"activity_type"`
	Note             *string   `json:"note"`
	ActorID          string    `json:"actor_id"`
	ActorName        string    `json:"actor_name"`
	FromStatus       *string   `json:"from_status"`
	ToStatus         *string   `json:"to_status"`
	FromAssigneeID   *string   `json:"from_assignee_id"`
	FromAssigneeName *string   `json:"from_assignee_name"`
	ToAssigneeID     *string   `json:"to_assignee_id"`
	ToAssigneeName   *string   `json:"to_assignee_name"`
	CreatedAt        time.Time `json:"created_at"`
}

type Public struct {
	ID                       string              `json:"id"`
	SourceType               string              `json:"source_type"`
	ScenarioID               *string             `json:"scenario_id"`
	ScenarioName             *string             `json:"scenario_name"`
	ScenarioAssignmentStatus string              `json:"scenario_assignment_status"`
	TargetID                 string              `json:"evaluation_target_id"`
	TargetName               string              `json:"evaluation_target_name"`
	Title                    string              `json:"title"`
	Description              *string             `json:"description"`
	AgentResponseText        *string             `json:"agent_response_text"`
	AgentVersion             *string             `json:"agent_version"`
	Environment              string              `json:"environment"`
	OccurredAt               time.Time           `json:"occurred_at"`
	Status                   string              `json:"status"`
	AssigneeID               *string             `json:"assignee_id"`
	AssigneeName             *string             `json:"assignee_name"`
	ResolvedAt               *time.Time          `json:"resolved_at"`
	BusinessReference        *string             `json:"business_reference"`
	SessionID                *string             `json:"session_id"`
	InvalidatedAt            *time.Time          `json:"invalidated_at"`
	InvalidatedBy            *string             `json:"invalidated_by"`
	InvalidatorName          *string             `json:"invalidator_name"`
	InvalidReason            *string             `json:"invalid_reason"`
	LockVersion              uint32              `json:"lock_version"`
	CreatedBy                string              `json:"created_by"`
	CreatorName              string              `json:"creator_name"`
	CreatedAt                time.Time           `json:"created_at"`
	UpdatedAt                time.Time           `json:"updated_at"`
	IssueTags                []dataset.CaseTag   `json:"issue_tags"`
	Evaluation               *EvaluationContext  `json:"evaluation"`
	OriginalAttachments      []attachment.Public `json:"original_attachments"`
	Attachments              []attachment.Public `json:"attachments"`
	Activities               []ActivityPublic    `json:"activities"`
}

type EvaluationContext struct {
	CaseResultID  string  `json:"case_result_id"`
	RunID         string  `json:"evaluation_run_id"`
	EvaluatorID   string  `json:"evaluator_id"`
	EvaluatorName string  `json:"evaluator_name"`
	DatasetID     string  `json:"dataset_id"`
	DatasetName   string  `json:"dataset_name"`
	VersionID     string  `json:"dataset_version_id"`
	VersionNo     uint32  `json:"version_no"`
	VersionCaseID string  `json:"version_case_id"`
	CaseName      *string `json:"case_name"`
	UserPrompt    *string `json:"user_prompt"`
	Score         *uint8  `json:"score"`
	Comment       *string `json:"comment"`
}

func (item Badcase) Public() Public {
	if item.IssueTags == nil {
		item.IssueTags = []dataset.CaseTag{}
	}
	if item.Attachments == nil {
		item.Attachments = []attachment.Public{}
	}
	if item.OriginalAttachments == nil {
		item.OriginalAttachments = []attachment.Public{}
	}
	if item.Activities == nil {
		item.Activities = []ActivityPublic{}
	}
	var evaluation *EvaluationContext
	if item.CaseResultID != nil && item.RunID != nil && item.EvaluatorID != nil &&
		item.DatasetID != nil && item.VersionID != nil && item.VersionCaseID != nil &&
		item.VersionNo != nil {
		evaluation = &EvaluationContext{
			CaseResultID: item.CaseResultID.String(), RunID: item.RunID.String(),
			EvaluatorID: item.EvaluatorID.String(), EvaluatorName: item.EvaluatorName,
			DatasetID: item.DatasetID.String(), DatasetName: item.DatasetName,
			VersionID: item.VersionID.String(), VersionNo: *item.VersionNo,
			VersionCaseID: item.VersionCaseID.String(), CaseName: item.CaseName,
			UserPrompt: item.UserPrompt, Score: item.Score, Comment: item.Comment,
		}
	}
	var assigneeID, assigneeName, invalidatedBy, invalidatorName *string
	if item.AssigneeID != nil {
		value := item.AssigneeID.String()
		assigneeID = &value
		name := item.AssigneeName
		assigneeName = &name
	}
	if item.InvalidatedBy != nil {
		value := item.InvalidatedBy.String()
		invalidatedBy = &value
		name := item.InvalidatorName
		invalidatorName = &name
	}
	var scenarioID *string
	if item.ScenarioID != nil {
		value := item.ScenarioID.String()
		scenarioID = &value
	}
	return Public{
		ID: item.ID.String(), SourceType: item.SourceType,
		ScenarioID: scenarioID, ScenarioName: item.ScenarioName,
		ScenarioAssignmentStatus: item.AssignmentStatus,
		TargetID:                 item.TargetID.String(), TargetName: item.TargetName,
		Title: item.Title, Description: item.Description,
		AgentResponseText: item.AgentResponseText, AgentVersion: item.AgentVersion,
		Environment: item.Environment, OccurredAt: item.OccurredAt,
		Status: item.Status, AssigneeID: assigneeID, AssigneeName: assigneeName,
		ResolvedAt: item.ResolvedAt, BusinessReference: item.BusinessReference,
		SessionID: item.SessionID, InvalidatedAt: item.InvalidatedAt,
		InvalidatedBy: invalidatedBy, InvalidatorName: invalidatorName,
		InvalidReason: item.InvalidReason, LockVersion: item.LockVersion,
		CreatedBy: item.CreatedBy.String(), CreatorName: item.CreatorName,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		IssueTags: item.IssueTags, Evaluation: evaluation,
		OriginalAttachments: item.OriginalAttachments,
		Attachments:         item.Attachments, Activities: item.Activities,
	}
}

type UserOption struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type PagePublic struct {
	Public
	CandidateAssignees []UserOption      `json:"candidate_assignees"`
	CandidateIssueTags []dataset.CaseTag `json:"candidate_issue_tags"`
	AllowedActions     []string          `json:"allowed_actions"`
}
