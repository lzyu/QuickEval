package audit

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
)

type Entry struct {
	ID              id.UUID         `gorm:"column:id;type:binary(16);primaryKey"`
	ActorUserID     *id.UUID        `gorm:"column:actor_user_id;type:binary(16)"`
	ActorUsername   *string         `gorm:"column:actor_username"`
	Action          string          `gorm:"column:action"`
	EntityType      string          `gorm:"column:entity_type"`
	EntityID        id.UUID         `gorm:"column:entity_id;type:binary(16)"`
	SubjectUsername *string         `gorm:"column:subject_username"`
	BeforeData      json.RawMessage `gorm:"column:before_data"`
	AfterData       json.RawMessage `gorm:"column:after_data"`
	RequestID       string          `gorm:"column:request_id"`
	IPAddress       string          `gorm:"column:ip_address"`
	UserAgent       string          `gorm:"column:user_agent"`
	CreatedAt       time.Time       `gorm:"column:created_at"`
}

func (Entry) TableName() string {
	return "audit_logs"
}

type Recorder struct {
	db *gorm.DB
}

func NewRecorder(db *gorm.DB) Recorder {
	return Recorder{db: db}
}

func (recorder Recorder) Record(
	ctx context.Context,
	actorID *id.UUID,
	action, entityType string,
	entityID id.UUID,
	before, after any,
	requestID, ipAddress, userAgent string,
) error {
	beforeJSON, err := marshalOptional(before)
	if err != nil {
		return err
	}
	afterJSON, err := marshalOptional(after)
	if err != nil {
		return err
	}
	actorUsername, err := recorder.usernameFor(ctx, actorID)
	if err != nil {
		return err
	}
	var subjectUsername *string
	if entityType == "user" {
		subjectUsername, err = recorder.usernameFor(ctx, &entityID)
		if err != nil {
			return err
		}
	}
	return recorder.db.WithContext(ctx).Create(&Entry{
		ID:              id.MustNew(),
		ActorUserID:     actorID,
		ActorUsername:   actorUsername,
		Action:          action,
		EntityType:      entityType,
		EntityID:        entityID,
		SubjectUsername: subjectUsername,
		BeforeData:      beforeJSON,
		AfterData:       afterJSON,
		RequestID:       requestID,
		IPAddress:       ipAddress,
		UserAgent:       userAgent,
	}).Error
}

func (recorder Recorder) usernameFor(ctx context.Context, userID *id.UUID) (*string, error) {
	if userID == nil {
		return nil, nil
	}
	var username string
	err := recorder.db.WithContext(ctx).
		Table("user_identities").
		Select("provider_subject").
		Where("user_id = ? AND provider = 'local'", *userID).
		Take(&username).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &username, nil
}

func (recorder Recorder) List(
	ctx context.Context,
	page, pageSize int,
	actorID *id.UUID,
	action, entityType, entityID string,
) ([]Entry, int64, error) {
	query := recorder.db.WithContext(ctx).Model(&Entry{})
	if actorID != nil {
		query = query.Where("actor_user_id = ?", *actorID)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if entityType != "" {
		query = query.Where("entity_type = ?", entityType)
	}
	if entityID != "" {
		parsedID, err := id.Parse(entityID)
		if err != nil {
			return []Entry{}, 0, nil
		}
		query = query.Where("entity_id = ?", parsedID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var entries []Entry
	err := query.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&entries).Error
	return entries, total, err
}

func marshalOptional(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

type Public struct {
	ID              string          `json:"id"`
	ActorUserID     *string         `json:"actor_user_id"`
	ActorUsername   *string         `json:"actor_username"`
	Action          string          `json:"action"`
	EntityType      string          `json:"entity_type"`
	EntityID        string          `json:"entity_id"`
	SubjectUsername *string         `json:"subject_username"`
	BeforeData      json.RawMessage `json:"before_data"`
	AfterData       json.RawMessage `json:"after_data"`
	RequestID       string          `json:"request_id"`
	IPAddress       string          `json:"ip_address"`
	UserAgent       string          `json:"user_agent"`
	CreatedAt       time.Time       `json:"created_at"`
}

func (entry Entry) Public() Public {
	var actorID *string
	if entry.ActorUserID != nil {
		value := entry.ActorUserID.String()
		actorID = &value
	}
	return Public{
		ID:              entry.ID.String(),
		ActorUserID:     actorID,
		ActorUsername:   entry.ActorUsername,
		Action:          entry.Action,
		EntityType:      entry.EntityType,
		EntityID:        entry.EntityID.String(),
		SubjectUsername: entry.SubjectUsername,
		BeforeData:      entry.BeforeData,
		AfterData:       entry.AfterData,
		RequestID:       entry.RequestID,
		IPAddress:       entry.IPAddress,
		UserAgent:       entry.UserAgent,
		CreatedAt:       entry.CreatedAt,
	}
}
