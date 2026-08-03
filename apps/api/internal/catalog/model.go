package catalog

import (
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"

	CaseTagScopeGlobal = "global"
	CaseTagScopeTarget = "target"
)

type EvaluationTarget struct {
	ID          id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	Name        string    `gorm:"column:name"`
	Description *string   `gorm:"column:description"`
	Status      string    `gorm:"column:status"`
	LockVersion uint32    `gorm:"column:lock_version"`
	CreatedBy   id.UUID   `gorm:"column:created_by;type:binary(16)"`
	UpdatedBy   id.UUID   `gorm:"column:updated_by;type:binary(16)"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (EvaluationTarget) TableName() string {
	return "evaluation_targets"
}

type Scenario struct {
	ID                 id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	EvaluationTargetID id.UUID   `gorm:"column:evaluation_target_id;type:binary(16)"`
	TargetName         string    `gorm:"column:target_name;->"`
	Name               string    `gorm:"column:name"`
	Description        *string   `gorm:"column:description"`
	Status             string    `gorm:"column:status"`
	LockVersion        uint32    `gorm:"column:lock_version"`
	CreatedBy          id.UUID   `gorm:"column:created_by;type:binary(16)"`
	UpdatedBy          id.UUID   `gorm:"column:updated_by;type:binary(16)"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (Scenario) TableName() string {
	return "scenarios"
}

type CaseTag struct {
	ID                 id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	Scope              string    `gorm:"column:scope"`
	EvaluationTargetID *id.UUID  `gorm:"column:evaluation_target_id;type:binary(16)"`
	TargetName         *string   `gorm:"column:target_name;->"`
	Name               string    `gorm:"column:name"`
	Description        *string   `gorm:"column:description"`
	Status             string    `gorm:"column:status"`
	SortOrder          uint32    `gorm:"column:sort_order"`
	LockVersion        uint32    `gorm:"column:lock_version"`
	CreatedBy          id.UUID   `gorm:"column:created_by;type:binary(16)"`
	UpdatedBy          id.UUID   `gorm:"column:updated_by;type:binary(16)"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (CaseTag) TableName() string {
	return "case_tags"
}

type IssueTag struct {
	ID          id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	Name        string    `gorm:"column:name"`
	Description *string   `gorm:"column:description"`
	Status      string    `gorm:"column:status"`
	SortOrder   uint32    `gorm:"column:sort_order"`
	LockVersion uint32    `gorm:"column:lock_version"`
	CreatedBy   id.UUID   `gorm:"column:created_by;type:binary(16)"`
	UpdatedBy   id.UUID   `gorm:"column:updated_by;type:binary(16)"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (IssueTag) TableName() string {
	return "issue_tags"
}

type TargetPublic struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Status      string    `json:"status"`
	LockVersion uint32    `json:"lock_version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (target EvaluationTarget) Public() TargetPublic {
	return TargetPublic{
		ID:          target.ID.String(),
		Name:        target.Name,
		Description: target.Description,
		Status:      target.Status,
		LockVersion: target.LockVersion,
		CreatedAt:   target.CreatedAt,
		UpdatedAt:   target.UpdatedAt,
	}
}

type ScenarioPublic struct {
	ID                 string    `json:"id"`
	EvaluationTargetID string    `json:"evaluation_target_id"`
	TargetName         string    `json:"evaluation_target_name"`
	Name               string    `json:"name"`
	Description        *string   `json:"description"`
	Status             string    `json:"status"`
	LockVersion        uint32    `json:"lock_version"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (scenario Scenario) Public() ScenarioPublic {
	return ScenarioPublic{
		ID:                 scenario.ID.String(),
		EvaluationTargetID: scenario.EvaluationTargetID.String(),
		TargetName:         scenario.TargetName,
		Name:               scenario.Name,
		Description:        scenario.Description,
		Status:             scenario.Status,
		LockVersion:        scenario.LockVersion,
		CreatedAt:          scenario.CreatedAt,
		UpdatedAt:          scenario.UpdatedAt,
	}
}

type TagPublic struct {
	ID                 string    `json:"id"`
	Scope              string    `json:"scope,omitempty"`
	EvaluationTargetID *string   `json:"evaluation_target_id,omitempty"`
	TargetName         *string   `json:"evaluation_target_name,omitempty"`
	Name               string    `json:"name"`
	Description        *string   `json:"description"`
	Status             string    `json:"status"`
	SortOrder          uint32    `json:"sort_order"`
	LockVersion        uint32    `json:"lock_version"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (tag CaseTag) Public() TagPublic {
	var targetID *string
	if tag.EvaluationTargetID != nil {
		value := tag.EvaluationTargetID.String()
		targetID = &value
	}
	return TagPublic{
		ID:                 tag.ID.String(),
		Scope:              tag.Scope,
		EvaluationTargetID: targetID,
		TargetName:         tag.TargetName,
		Name:               tag.Name,
		Description:        tag.Description,
		Status:             tag.Status,
		SortOrder:          tag.SortOrder,
		LockVersion:        tag.LockVersion,
		CreatedAt:          tag.CreatedAt,
		UpdatedAt:          tag.UpdatedAt,
	}
}

func (tag IssueTag) Public() TagPublic {
	return TagPublic{
		ID:          tag.ID.String(),
		Name:        tag.Name,
		Description: tag.Description,
		Status:      tag.Status,
		SortOrder:   tag.SortOrder,
		LockVersion: tag.LockVersion,
		CreatedAt:   tag.CreatedAt,
		UpdatedAt:   tag.UpdatedAt,
	}
}

type ReorderItem struct {
	ID                  id.UUID
	SortOrder           uint32
	ExpectedLockVersion uint32
}
