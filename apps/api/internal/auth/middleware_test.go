package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/id"
	"github.com/lzyu/QuickEval/apps/api/internal/user"
)

func TestCSRFMiddlewareRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/", func(ctx *gin.Context) {
		SetPrincipal(ctx, Principal{
			Session: Session{CSRFToken: "expected"},
		})
	}, Middleware{}.CSRF(), func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}

func TestRequireAdminRejectsMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/", func(ctx *gin.Context) {
		SetPrincipal(ctx, Principal{User: accountWithRole("member")})
	}, RequireAdmin(), func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}

func TestRequirePasswordChangeCompleteRejectsInitialPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/", func(ctx *gin.Context) {
		SetPrincipal(ctx, Principal{User: user.Account{PasswordChangeRequired: true}})
	}, RequirePasswordChangeComplete(), func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}

func accountWithRole(role string) user.Account {
	return user.Account{ID: id.MustNew(), Role: role}
}
