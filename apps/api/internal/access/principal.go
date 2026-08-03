package access

import (
	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
)

const principalKey = "access_principal"

type Principal struct {
	UserID id.UUID
	Role   string
}

func (principal Principal) ID() id.UUID {
	return principal.UserID
}

func (principal Principal) Admin() bool {
	return principal.Role == "operator" || principal.Role == "super_admin"
}

func Set(ctx *gin.Context, principal Principal) {
	ctx.Set(principalKey, principal)
}

func From(ctx *gin.Context) (Principal, bool) {
	value, exists := ctx.Get(principalKey)
	if !exists {
		return Principal{}, false
	}
	principal, ok := value.(Principal)
	return principal, ok
}
