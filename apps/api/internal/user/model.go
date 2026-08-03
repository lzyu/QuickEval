package user

import (
	"time"

	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

const (
	RoleMember     = "member"
	RoleOperator   = "operator"
	RoleSuperAdmin = "super_admin"

	StatusActive   = "active"
	StatusDisabled = "disabled"
)

type User struct {
	ID          id.UUID   `gorm:"column:id;type:binary(16);primaryKey"`
	DisplayName string    `gorm:"column:display_name"`
	Email       *string   `gorm:"column:email"`
	Role        string    `gorm:"column:role"`
	Status      string    `gorm:"column:status"`
	LockVersion uint32    `gorm:"column:lock_version"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (User) TableName() string {
	return "users"
}

type Identity struct {
	ID                     id.UUID    `gorm:"column:id;type:binary(16);primaryKey"`
	UserID                 id.UUID    `gorm:"column:user_id;type:binary(16)"`
	Provider               string     `gorm:"column:provider"`
	ProviderSubject        string     `gorm:"column:provider_subject"`
	PasswordHash           *string    `gorm:"column:password_hash"`
	PasswordChangeRequired bool       `gorm:"column:password_change_required"`
	Status                 string     `gorm:"column:status"`
	LastLoginAt            *time.Time `gorm:"column:last_login_at"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
	UpdatedAt              time.Time  `gorm:"column:updated_at"`
}

func (Identity) TableName() string {
	return "user_identities"
}

type Account struct {
	ID                     id.UUID
	Username               string
	DisplayName            string
	Email                  *string
	Role                   string
	Status                 string
	LockVersion            uint32
	IdentityStatus         string
	PasswordHash           string
	PasswordChangeRequired bool
	IdentityProvider       string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func (account Account) IsActive() bool {
	return account.Status == StatusActive && account.IdentityStatus == StatusActive
}

func (account Account) CanManageOperations() bool {
	return account.Role == RoleOperator || account.Role == RoleSuperAdmin
}

func (account Account) CanManageUsers() bool {
	return account.Role == RoleSuperAdmin
}

type Public struct {
	ID                     string     `json:"id"`
	Username               string     `json:"username"`
	DisplayName            string     `json:"display_name"`
	Email                  *string    `json:"email"`
	Role                   string     `json:"role"`
	Status                 string     `json:"status"`
	LockVersion            uint32     `json:"lock_version"`
	PasswordChangeRequired bool       `json:"password_change_required"`
	CreatedAt              *time.Time `json:"created_at,omitempty"`
	UpdatedAt              *time.Time `json:"updated_at,omitempty"`
}

func (account Account) ToPublic() Public {
	return Public{
		ID:                     account.ID.String(),
		Username:               account.Username,
		DisplayName:            account.DisplayName,
		Email:                  account.Email,
		Role:                   account.Role,
		Status:                 account.Status,
		LockVersion:            account.LockVersion,
		PasswordChangeRequired: account.PasswordChangeRequired,
		CreatedAt:              &account.CreatedAt,
		UpdatedAt:              &account.UpdatedAt,
	}
}
