package dataset

import (
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

const (
	DatasetActive   = "active"
	DatasetArchived = "archived"

	VersionDraft     = "draft"
	VersionPublished = "published"
	VersionArchived  = "archived"
)

type Dataset struct {
	ID                    id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	ScenarioID            id.UUID   `gorm:"column:scenario_id;type:binary(16)"`
	ScenarioName          string    `gorm:"column:scenario_name;->"`
	TargetID              id.UUID   `gorm:"column:evaluation_target_id;type:binary(16);->"`
	TargetName            string    `gorm:"column:evaluation_target_name;->"`
	Name                  string    `gorm:"column:name"`
	Description           *string   `gorm:"column:description"`
	Status                string    `gorm:"column:status"`
	LockVersion           uint32    `gorm:"column:lock_version"`
	CreatedBy             id.UUID   `gorm:"column:created_by;type:binary(16)"`
	UpdatedBy             id.UUID   `gorm:"column:updated_by;type:binary(16)"`
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
	LatestVersionNo       *uint32   `gorm:"column:latest_version_no;->"`
	PublishedVersionCount int64     `gorm:"column:published_version_count;->"`
	DraftVersionID        *id.UUID  `gorm:"column:draft_version_id;type:binary(16);->"`
	DraftCaseCount        int64     `gorm:"column:draft_case_count;->"`
}

func (Dataset) TableName() string { return "datasets" }

type Version struct {
	ID            id.UUID    `gorm:"column:id;type:binary(16);primaryKey"`
	DatasetID     id.UUID    `gorm:"column:dataset_id;type:binary(16)"`
	BaseVersionID *id.UUID   `gorm:"column:base_version_id;type:binary(16)"`
	VersionNo     *uint32    `gorm:"column:version_no"`
	Status        string     `gorm:"column:status"`
	ReleaseNote   *string    `gorm:"column:release_note"`
	LockVersion   uint32     `gorm:"column:lock_version"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
	PublishedBy   *id.UUID   `gorm:"column:published_by;type:binary(16)"`
	ArchivedAt    *time.Time `gorm:"column:archived_at"`
	ArchivedBy    *id.UUID   `gorm:"column:archived_by;type:binary(16)"`
	CreatedBy     id.UUID    `gorm:"column:created_by;type:binary(16)"`
	UpdatedBy     id.UUID    `gorm:"column:updated_by;type:binary(16)"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
	CaseCount     int64      `gorm:"column:case_count;->"`
	EnabledCount  int64      `gorm:"column:enabled_count;->"`
}

func (Version) TableName() string { return "dataset_versions" }

type VersionCase struct {
	ID               id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	DatasetVersionID id.UUID   `gorm:"column:dataset_version_id;type:binary(16)"`
	CaseKey          id.UUID   `gorm:"column:case_key;type:binary(16)"`
	Name             *string   `gorm:"column:name"`
	UserPrompt       string    `gorm:"column:user_prompt"`
	Precondition     *string   `gorm:"column:precondition"`
	ExpectedResult   *string   `gorm:"column:expected_result"`
	JudgingGuide     *string   `gorm:"column:judging_guide"`
	SortOrder        uint32    `gorm:"column:sort_order"`
	IsEnabled        bool      `gorm:"column:is_enabled"`
	LockVersion      uint32    `gorm:"column:lock_version"`
	CreatedBy        id.UUID   `gorm:"column:created_by;type:binary(16)"`
	UpdatedBy        id.UUID   `gorm:"column:updated_by;type:binary(16)"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
	Tags             []CaseTag `gorm:"-"`
}

func (VersionCase) TableName() string { return "version_cases" }

type VersionCaseTag struct {
	ID              id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	VersionCaseID   id.UUID   `gorm:"column:version_case_id;type:binary(16)"`
	CaseTagID       id.UUID   `gorm:"column:case_tag_id;type:binary(16)"`
	TagNameSnapshot string    `gorm:"column:tag_name_snapshot"`
	CreatedBy       id.UUID   `gorm:"column:created_by;type:binary(16)"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (VersionCaseTag) TableName() string { return "version_case_tags" }

type CaseTag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DatasetPublic struct {
	ID                    string    `json:"id"`
	ScenarioID            string    `json:"scenario_id"`
	ScenarioName          string    `json:"scenario_name"`
	TargetID              string    `json:"evaluation_target_id"`
	TargetName            string    `json:"evaluation_target_name"`
	Name                  string    `json:"name"`
	Description           *string   `json:"description"`
	Status                string    `json:"status"`
	LockVersion           uint32    `json:"lock_version"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	LatestVersionNo       *uint32   `json:"latest_version_no"`
	PublishedVersionCount int64     `json:"published_version_count"`
	DraftVersionID        *string   `json:"draft_version_id"`
	DraftCaseCount        int64     `json:"draft_case_count"`
}

func (item Dataset) Public() DatasetPublic {
	var draftID *string
	if item.DraftVersionID != nil {
		value := item.DraftVersionID.String()
		draftID = &value
	}
	return DatasetPublic{
		ID: item.ID.String(), ScenarioID: item.ScenarioID.String(), ScenarioName: item.ScenarioName,
		TargetID: item.TargetID.String(), TargetName: item.TargetName, Name: item.Name,
		Description: item.Description, Status: item.Status, LockVersion: item.LockVersion,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		LatestVersionNo: item.LatestVersionNo, PublishedVersionCount: item.PublishedVersionCount,
		DraftVersionID: draftID, DraftCaseCount: item.DraftCaseCount,
	}
}

type VersionPublic struct {
	ID            string     `json:"id"`
	DatasetID     string     `json:"dataset_id"`
	BaseVersionID *string    `json:"base_version_id"`
	VersionNo     *uint32    `json:"version_no"`
	Status        string     `json:"status"`
	ReleaseNote   *string    `json:"release_note"`
	LockVersion   uint32     `json:"lock_version"`
	PublishedAt   *time.Time `json:"published_at"`
	ArchivedAt    *time.Time `json:"archived_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CaseCount     int64      `json:"case_count"`
	EnabledCount  int64      `json:"enabled_count"`
}

func (item Version) Public() VersionPublic {
	var baseID *string
	if item.BaseVersionID != nil {
		value := item.BaseVersionID.String()
		baseID = &value
	}
	return VersionPublic{
		ID: item.ID.String(), DatasetID: item.DatasetID.String(), BaseVersionID: baseID,
		VersionNo: item.VersionNo, Status: item.Status, ReleaseNote: item.ReleaseNote,
		LockVersion: item.LockVersion, PublishedAt: item.PublishedAt, ArchivedAt: item.ArchivedAt,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		CaseCount: item.CaseCount, EnabledCount: item.EnabledCount,
	}
}

type CasePublic struct {
	ID             string    `json:"id"`
	VersionID      string    `json:"dataset_version_id"`
	CaseKey        string    `json:"case_key"`
	Name           *string   `json:"name"`
	UserPrompt     string    `json:"user_prompt"`
	Precondition   *string   `json:"precondition"`
	ExpectedResult *string   `json:"expected_result"`
	JudgingGuide   *string   `json:"judging_guide"`
	SortOrder      uint32    `json:"sort_order"`
	IsEnabled      bool      `json:"is_enabled"`
	LockVersion    uint32    `json:"lock_version"`
	Tags           []CaseTag `json:"tags"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (item VersionCase) Public() CasePublic {
	return CasePublic{
		ID: item.ID.String(), VersionID: item.DatasetVersionID.String(), CaseKey: item.CaseKey.String(),
		Name: item.Name, UserPrompt: item.UserPrompt, Precondition: item.Precondition,
		ExpectedResult: item.ExpectedResult, JudgingGuide: item.JudgingGuide,
		SortOrder: item.SortOrder, IsEnabled: item.IsEnabled, LockVersion: item.LockVersion,
		Tags: item.Tags, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}
