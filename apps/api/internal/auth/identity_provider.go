package auth

import (
	"context"
	"errors"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/lzyu/QuickEval/apps/api/internal/user"
	"gorm.io/gorm"
)

var ErrInvalidIdentity = errors.New("invalid identity credentials")

// IdentityProvider is the extension boundary for replacing local accounts with OA SSO.
type IdentityProvider interface {
	Authenticate(ctx context.Context, login, password string) (user.Account, error)
	ChangePassword(ctx context.Context, userID id.UUID, currentPassword, newPassword string) error
}

type LocalIdentityProvider struct {
	users             user.Repository
	hasher            user.PasswordHasher
	passwordMinLength int
}

func NewLocalIdentityProvider(
	users user.Repository,
	hasher user.PasswordHasher,
	passwordMinLength int,
) LocalIdentityProvider {
	return LocalIdentityProvider{
		users:             users,
		hasher:            hasher,
		passwordMinLength: passwordMinLength,
	}
}

func (provider LocalIdentityProvider) Authenticate(
	ctx context.Context,
	login, password string,
) (user.Account, error) {
	account, err := provider.users.FindAccountByLogin(ctx, login)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return user.Account{}, ErrInvalidIdentity
	}
	if err != nil {
		return user.Account{}, err
	}
	if !account.IsActive() || provider.hasher.Compare(account.PasswordHash, password) != nil {
		return user.Account{}, ErrInvalidIdentity
	}
	return account, nil
}

func (provider LocalIdentityProvider) ChangePassword(
	ctx context.Context,
	userID id.UUID,
	currentPassword, newPassword string,
) error {
	account, err := provider.users.FindAccountByID(ctx, userID)
	if err != nil {
		return err
	}
	if provider.hasher.Compare(account.PasswordHash, currentPassword) != nil {
		return apperror.Validation(
			apperror.FieldError{Field: "current_password", Message: "当前密码不正确"},
		)
	}
	if len(newPassword) < provider.passwordMinLength {
		return apperror.Validation(
			apperror.FieldError{Field: "new_password", Message: "新密码长度不足"},
		)
	}
	hash, err := provider.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	if err := provider.users.UpdatePassword(ctx, userID, hash); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.Conflict("IDENTITY_NOT_LOCAL", "当前账号不支持修改本地密码")
		}
		return err
	}
	return nil
}
