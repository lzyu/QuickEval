package user

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"gorm.io/gorm"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

const InitialPassword = "123456"

type SessionRevoker interface {
	RevokeUser(ctx context.Context, userID id.UUID) error
}

type Service struct {
	repository        Repository
	hasher            PasswordHasher
	sessions          SessionRevoker
	passwordMinLength int
}

func NewService(
	repository Repository,
	hasher PasswordHasher,
	sessions SessionRevoker,
	passwordMinLength int,
) Service {
	return Service{
		repository:        repository,
		hasher:            hasher,
		sessions:          sessions,
		passwordMinLength: passwordMinLength,
	}
}

type CreateInput struct {
	Username    string
	DisplayName string
	Email       *string
	Role        string
}

type BootstrapInput struct {
	Username    string
	DisplayName string
	Email       *string
	Password    string
}

func (service Service) Create(
	ctx context.Context,
	actorID id.UUID,
	input CreateInput,
) (Account, error) {
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = normalizeEmail(input.Email)

	if err := service.validateAccountInput(
		input.Username,
		input.DisplayName,
		input.Email,
		input.Role,
		"",
		false,
	); err != nil {
		return Account{}, err
	}
	hash, err := service.hasher.Hash(InitialPassword)
	if err != nil {
		return Account{}, err
	}
	account := Account{
		ID:                     id.MustNew(),
		Username:               input.Username,
		DisplayName:            input.DisplayName,
		Email:                  input.Email,
		Role:                   input.Role,
		Status:                 StatusActive,
		PasswordHash:           hash,
		PasswordChangeRequired: true,
	}
	if err := service.repository.CreateAccount(
		ctx,
		account,
		id.MustNew(),
		&actorID,
	); err != nil {
		if IsDuplicate(err) {
			return Account{}, apperror.Conflict("NAME_CONFLICT", "用户名或邮箱已存在")
		}
		return Account{}, err
	}
	return service.repository.FindAccountByID(ctx, account.ID)
}

func (service Service) BootstrapAdmin(
	ctx context.Context,
	input BootstrapInput,
) (Account, error) {
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = normalizeEmail(input.Email)
	if err := service.validateAccountInput(
		input.Username,
		input.DisplayName,
		input.Email,
		RoleAdmin,
		input.Password,
		true,
	); err != nil {
		return Account{}, err
	}
	count, err := service.repository.CountActiveAdmins(ctx)
	if err != nil {
		return Account{}, err
	}
	if count > 0 {
		return Account{}, apperror.Conflict("ADMIN_ALREADY_EXISTS", "系统中已经存在管理员")
	}
	hash, err := service.hasher.Hash(input.Password)
	if err != nil {
		return Account{}, err
	}
	account := Account{
		ID:                     id.MustNew(),
		Username:               input.Username,
		DisplayName:            input.DisplayName,
		Email:                  input.Email,
		Role:                   RoleAdmin,
		Status:                 StatusActive,
		PasswordHash:           hash,
		PasswordChangeRequired: false,
	}
	if err := service.repository.CreateAccount(ctx, account, id.MustNew(), nil); err != nil {
		if IsDuplicate(err) {
			return Account{}, apperror.Conflict("NAME_CONFLICT", "用户名或邮箱已存在")
		}
		return Account{}, err
	}
	return service.repository.FindAccountByID(ctx, account.ID)
}

type UpdateInput struct {
	DisplayName         string
	Email               *string
	Role                string
	ExpectedLockVersion uint32
}

func (service Service) Update(
	ctx context.Context,
	actorID, userID id.UUID,
	input UpdateInput,
) (Account, Account, error) {
	before, err := service.repository.FindAccountByID(ctx, userID)
	if err != nil {
		return Account{}, Account{}, mapNotFound(err)
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = normalizeEmail(input.Email)
	if err := service.validateAccountInput(
		before.Username,
		input.DisplayName,
		input.Email,
		input.Role,
		"",
		false,
	); err != nil {
		return Account{}, Account{}, err
	}
	if actorID == userID && input.Role != RoleAdmin {
		return Account{}, Account{}, apperror.Conflict(
			"SELF_ADMIN_ROLE_CHANGE_FORBIDDEN",
			"不能移除当前登录账号的管理员角色",
		)
	}
	if err := service.repository.UpdateProfile(
		ctx,
		userID,
		actorID,
		input.DisplayName,
		input.Email,
		input.Role,
		input.ExpectedLockVersion,
	); err != nil {
		return Account{}, Account{}, mapWriteError(err)
	}
	after, err := service.repository.FindAccountByID(ctx, userID)
	return before, after, err
}

func (service Service) SetStatus(
	ctx context.Context,
	actorID, userID id.UUID,
	status string,
	expectedVersion uint32,
) (Account, Account, error) {
	if status != StatusActive && status != StatusDisabled {
		return Account{}, Account{}, apperror.Validation()
	}
	if status == StatusDisabled && actorID == userID {
		return Account{}, Account{}, apperror.Conflict(
			"SELF_DISABLE_FORBIDDEN",
			"不能停用当前登录账号",
		)
	}
	before, err := service.repository.FindAccountByID(ctx, userID)
	if err != nil {
		return Account{}, Account{}, mapNotFound(err)
	}
	if before.Status == status {
		return Account{}, Account{}, apperror.Conflict(
			"INVALID_STATE_TRANSITION",
			"用户已经处于目标状态",
		)
	}
	if before.Role == RoleAdmin && status == StatusDisabled {
		activeAdmins, err := service.repository.CountActiveAdmins(ctx)
		if err != nil {
			return Account{}, Account{}, err
		}
		if activeAdmins <= 1 {
			return Account{}, Account{}, apperror.Conflict(
				"LAST_ADMIN_DISABLE_FORBIDDEN",
				"不能停用最后一个可用管理员",
			)
		}
	}
	if err := service.repository.SetStatus(
		ctx,
		userID,
		actorID,
		status,
		expectedVersion,
	); err != nil {
		return Account{}, Account{}, mapWriteError(err)
	}
	if status == StatusDisabled {
		if err := service.sessions.RevokeUser(ctx, userID); err != nil {
			return Account{}, Account{}, err
		}
	}
	after, err := service.repository.FindAccountByID(ctx, userID)
	return before, after, err
}

func (service Service) ResetPassword(
	ctx context.Context,
	userID id.UUID,
) error {
	hash, err := service.hasher.Hash(InitialPassword)
	if err != nil {
		return err
	}
	if err := service.repository.UpdatePassword(ctx, userID, hash, true); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.Conflict("IDENTITY_NOT_LOCAL", "该账号不支持重置本地密码")
		}
		return err
	}
	return service.sessions.RevokeUser(ctx, userID)
}

func (service Service) validateAccountInput(
	username, displayName string,
	email *string,
	role, password string,
	validatePassword bool,
) error {
	var fields []apperror.FieldError
	if !usernamePattern.MatchString(username) {
		fields = append(fields, apperror.FieldError{
			Field:   "username",
			Message: "用户名需为 3～64 位小写字母、数字、点、横线或下划线",
		})
	}
	if displayName == "" || len([]rune(displayName)) > 100 {
		fields = append(fields, apperror.FieldError{
			Field:   "display_name",
			Message: "姓名不能为空且不能超过 100 个字符",
		})
	}
	if email != nil {
		parsed, err := mail.ParseAddress(*email)
		if err != nil || parsed.Address != *email {
			fields = append(fields, apperror.FieldError{
				Field:   "email",
				Message: "邮箱格式不正确",
			})
		}
	}
	if role != RoleMember && role != RoleAdmin {
		fields = append(fields, apperror.FieldError{
			Field:   "role",
			Message: "角色必须是普通成员或管理员",
		})
	}
	if validatePassword && len(password) < service.passwordMinLength {
		fields = append(fields, apperror.FieldError{
			Field:   "password",
			Message: "密码长度不足",
		})
	}
	if len(fields) > 0 {
		return apperror.Validation(fields...)
	}
	return nil
}

func normalizeEmail(email *string) *string {
	if email == nil {
		return nil
	}
	value := strings.ToLower(strings.TrimSpace(*email))
	if value == "" {
		return nil
	}
	return &value
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound()
	}
	return err
}

func mapWriteError(err error) error {
	if errors.Is(err, ErrLockConflict) {
		return apperror.Conflict("LOCK_VERSION_CONFLICT", "数据已被其他用户更新，请刷新后重试")
	}
	if IsDuplicate(err) {
		return apperror.Conflict("NAME_CONFLICT", "邮箱已存在")
	}
	return err
}
