package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/lzyu/QuickEval/apps/api/internal/user"
)

const principalKey = "auth_principal"

type Principal struct {
	User    user.Account
	Session Session
	Token   string
}

func (principal Principal) ID() id.UUID {
	return principal.User.ID
}

func (principal Principal) Admin() bool {
	return principal.User.CanManageOperations()
}

func (principal Principal) SuperAdmin() bool {
	return principal.User.CanManageUsers()
}

func SetPrincipal(ctx *gin.Context, principal Principal) {
	ctx.Set(principalKey, principal)
}

func PrincipalFrom(ctx *gin.Context) (Principal, bool) {
	value, exists := ctx.Get(principalKey)
	if !exists {
		return Principal{}, false
	}
	principal, ok := value.(Principal)
	return principal, ok
}
