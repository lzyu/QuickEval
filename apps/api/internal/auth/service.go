package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/lzyu/QuickEval/apps/api/internal/apperror"
	"github.com/lzyu/QuickEval/apps/api/internal/config"
	"github.com/lzyu/QuickEval/apps/api/internal/user"
)

type Service struct {
	users       user.Repository
	identities  IdentityProvider
	sessions    SessionStore
	rateLimiter LoginRateLimiter
}

func NewService(
	users user.Repository,
	identities IdentityProvider,
	sessions SessionStore,
	rateLimiter LoginRateLimiter,
) Service {
	return Service{
		users:       users,
		identities:  identities,
		sessions:    sessions,
		rateLimiter: rateLimiter,
	}
}

func (service Service) Login(
	ctx context.Context,
	clientIP, username, password string,
) (string, Session, user.Account, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return "", Session{}, user.Account{}, apperror.Validation(
			apperror.FieldError{Field: "username", Message: "请输入用户名或邮箱"},
			apperror.FieldError{Field: "password", Message: "请输入密码"},
		)
	}

	blocked, retryAfter, err := service.rateLimiter.Blocked(ctx, clientIP, username)
	if err != nil {
		return "", Session{}, user.Account{}, err
	}
	if blocked {
		appError := apperror.New(http.StatusTooManyRequests, "LOGIN_RATE_LIMITED", "登录尝试过多，请稍后重试")
		appError.Details = map[string]any{"retry_after_seconds": max(int(retryAfter.Seconds()), 1)}
		return "", Session{}, user.Account{}, appError
	}

	account, err := service.identities.Authenticate(ctx, username, password)
	if err != nil {
		if errors.Is(err, ErrInvalidIdentity) {
			_ = service.rateLimiter.RecordFailure(ctx, clientIP, username)
			return "", Session{}, user.Account{}, invalidCredentials()
		}
		return "", Session{}, user.Account{}, err
	}

	if err := service.rateLimiter.Reset(ctx, clientIP, username); err != nil {
		return "", Session{}, user.Account{}, err
	}
	if err := service.users.UpdateLastLogin(ctx, account.ID); err != nil {
		return "", Session{}, user.Account{}, err
	}
	token, session, err := service.sessions.Create(ctx, account.ID)
	if err != nil {
		return "", Session{}, user.Account{}, err
	}
	return token, session, account, nil
}

func (service Service) Logout(ctx context.Context, token string) error {
	return service.sessions.Revoke(ctx, token)
}

func (service Service) ChangePassword(
	ctx context.Context,
	principal Principal,
	currentPassword, newPassword string,
) error {
	if err := service.identities.ChangePassword(
		ctx,
		principal.ID(),
		currentPassword,
		newPassword,
	); err != nil {
		return err
	}
	return service.sessions.RevokeUser(ctx, principal.ID())
}

func SessionPayload(
	account user.Account,
	session Session,
	cfg config.Config,
) map[string]any {
	admin := account.Role == user.RoleAdmin
	return map[string]any{
		"user": account.ToPublic(),
		"permissions": map[string]bool{
			"manage_users":    admin,
			"manage_catalog":  admin,
			"evaluate":        true,
			"manage_badcases": true,
			"view_audit_logs": admin,
		},
		"features": map[string]bool{
			"oa_login_enabled": false,
		},
		"upload_policy": map[string]any{
			"allowed_media_types": cfg.Upload.AllowedMediaTypes,
			"max_file_size":       cfg.Upload.MaxFileSize,
			"max_files_per_owner": cfg.Upload.MaxFilesPerOwner,
		},
		"csrf_token": session.CSRFToken,
	}
}

func invalidCredentials() *apperror.Error {
	return apperror.New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码不正确")
}
