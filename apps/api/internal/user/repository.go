package user

import (
	"context"
	"errors"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return Repository{db: db}
}

func (repository Repository) DB() *gorm.DB {
	return repository.db
}

func (repository Repository) FindAccountByLogin(ctx context.Context, login string) (Account, error) {
	var account Account
	err := repository.accountQuery(repository.db.WithContext(ctx)).
		Where(
			"LOWER(user_identities.provider_subject) = ? OR LOWER(users.email) = ?",
			strings.ToLower(login),
			strings.ToLower(login),
		).
		Take(&account).Error
	return account, err
}

func (repository Repository) FindAccountByID(ctx context.Context, userID id.UUID) (Account, error) {
	var account Account
	err := repository.accountQuery(repository.db.WithContext(ctx)).
		Where("users.id = ?", userID).
		Take(&account).Error
	return account, err
}

func (repository Repository) List(
	ctx context.Context,
	page, pageSize int,
	status, role, keyword string,
) ([]Account, int64, error) {
	query := repository.accountQuery(repository.db.WithContext(ctx))
	countQuery := repository.db.WithContext(ctx).
		Table("users").
		Joins("JOIN user_identities ON user_identities.user_id = users.id AND user_identities.provider = 'local'")

	if status != "" {
		query = query.Where("users.status = ?", status)
		countQuery = countQuery.Where("users.status = ?", status)
	}
	if role != "" {
		query = query.Where("users.role = ?", role)
		countQuery = countQuery.Where("users.role = ?", role)
	}
	if keyword != "" {
		pattern := "%" + keyword + "%"
		filter := "(users.display_name LIKE ? OR users.email LIKE ? OR user_identities.provider_subject LIKE ?)"
		query = query.Where(filter, pattern, pattern, pattern)
		countQuery = countQuery.Where(filter, pattern, pattern, pattern)
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var accounts []Account
	err := query.
		Order("users.created_at DESC, users.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&accounts).Error
	return accounts, total, err
}

func (repository Repository) CreateAccount(
	ctx context.Context,
	account Account,
	identityID id.UUID,
	actorID *id.UUID,
) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userRecord := User{
			ID:          account.ID,
			DisplayName: account.DisplayName,
			Email:       account.Email,
			Role:        account.Role,
			Status:      StatusActive,
		}
		values := map[string]any{
			"id":           userRecord.ID,
			"display_name": userRecord.DisplayName,
			"email":        userRecord.Email,
			"role":         userRecord.Role,
			"status":       userRecord.Status,
			"created_by":   actorID,
			"updated_by":   actorID,
		}
		if err := tx.Table("users").Create(values).Error; err != nil {
			return err
		}

		passwordHash := account.PasswordHash
		identity := Identity{
			ID:              identityID,
			UserID:          account.ID,
			Provider:        "local",
			ProviderSubject: account.Username,
			PasswordHash:    &passwordHash,
			Status:          StatusActive,
		}
		return tx.Create(&identity).Error
	})
}

func (repository Repository) UpdateLastLogin(ctx context.Context, userID id.UUID) error {
	return repository.db.WithContext(ctx).
		Model(&Identity{}).
		Where("user_id = ? AND provider = 'local'", userID).
		Update("last_login_at", time.Now().UTC()).Error
}

func (repository Repository) UpdateProfile(
	ctx context.Context,
	userID, actorID id.UUID,
	displayName string,
	email *string,
	role string,
	expectedVersion uint32,
) error {
	result := repository.db.WithContext(ctx).
		Table("users").
		Where("id = ? AND lock_version = ?", userID, expectedVersion).
		Updates(map[string]any{
			"display_name": displayName,
			"email":        email,
			"role":         role,
			"updated_by":   actorID,
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

func (repository Repository) SetStatus(
	ctx context.Context,
	userID, actorID id.UUID,
	status string,
	expectedVersion uint32,
) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userUpdates := map[string]any{
			"status":       status,
			"updated_by":   actorID,
			"lock_version": gorm.Expr("lock_version + 1"),
		}
		identityStatus := status
		if status == StatusDisabled {
			now := time.Now().UTC()
			userUpdates["disabled_at"] = now
			userUpdates["disabled_by"] = actorID
		} else {
			userUpdates["disabled_at"] = nil
			userUpdates["disabled_by"] = nil
		}

		result := tx.Table("users").
			Where("id = ? AND lock_version = ?", userID, expectedVersion).
			Updates(userUpdates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrLockConflict
		}

		return tx.Model(&Identity{}).
			Where("user_id = ?", userID).
			Update("status", identityStatus).Error
	})
}

func (repository Repository) UpdatePassword(
	ctx context.Context,
	userID id.UUID,
	passwordHash string,
) error {
	result := repository.db.WithContext(ctx).
		Model(&Identity{}).
		Where("user_id = ? AND provider = 'local'", userID).
		Update("password_hash", passwordHash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (repository Repository) CountActiveAdmins(ctx context.Context) (int64, error) {
	var count int64
	err := repository.db.WithContext(ctx).
		Model(&User{}).
		Where("role = ? AND status = ?", RoleAdmin, StatusActive).
		Count(&count).Error
	return count, err
}

func (repository Repository) accountQuery(db *gorm.DB) *gorm.DB {
	return db.Table("users").
		Select(
			"users.id, user_identities.provider_subject AS username, users.display_name, users.email, " +
				"users.role, users.status, users.lock_version, users.created_at, users.updated_at, " +
				"user_identities.status AS identity_status, user_identities.password_hash, " +
				"user_identities.provider AS identity_provider",
		).
		Joins(
			"JOIN user_identities ON user_identities.user_id = users.id AND user_identities.provider = 'local'",
		)
}

var ErrLockConflict = errors.New("user lock version conflict")

func IsDuplicate(err error) bool {
	var mysqlError *mysqlDriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
