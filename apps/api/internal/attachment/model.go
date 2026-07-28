package attachment

import (
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

type Attachment struct {
	ID           id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	CaseResultID *id.UUID  `gorm:"column:case_result_id;type:binary(16)"`
	BadcaseID    *id.UUID  `gorm:"column:badcase_id;type:binary(16)"`
	StoragePath  string    `gorm:"column:storage_path"`
	OriginalName string    `gorm:"column:original_name"`
	MediaType    string    `gorm:"column:media_type"`
	FileSize     int64     `gorm:"column:file_size"`
	SHA256       []byte    `gorm:"column:sha256;type:binary(32)"`
	Width        *uint32   `gorm:"column:width"`
	Height       *uint32   `gorm:"column:height"`
	SortOrder    uint32    `gorm:"column:sort_order"`
	CreatedBy    id.UUID   `gorm:"column:created_by;type:binary(16)"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (Attachment) TableName() string { return "attachments" }

type Public struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	MediaType    string    `json:"media_type"`
	FileSize     int64     `json:"file_size"`
	Width        *uint32   `json:"width"`
	Height       *uint32   `json:"height"`
	SortOrder    uint32    `json:"sort_order"`
	ContentURL   string    `json:"content_url"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

func (item Attachment) Public() Public {
	return Public{
		ID: item.ID.String(), OriginalName: item.OriginalName,
		MediaType: item.MediaType, FileSize: item.FileSize,
		Width: item.Width, Height: item.Height, SortOrder: item.SortOrder,
		ContentURL: "/api/v1/attachments/" + item.ID.String() + "/content",
		CreatedBy:  item.CreatedBy.String(), CreatedAt: item.CreatedAt,
	}
}

type Owner struct {
	Kind        string
	ID          id.UUID
	LockVersion uint32
	CreatedBy   id.UUID
	Status      string
	AnswerText  *string
	RunID       *id.UUID
	EvaluatorID *id.UUID
	Invalidated bool
}

type Upload struct {
	TempPath     string
	OriginalName string
	MediaType    string
	Extension    string
	FileSize     int64
	SHA256       []byte
	Width        uint32
	Height       uint32
}
