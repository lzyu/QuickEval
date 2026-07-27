package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/audit"
	"github.com/lzyu/QuickEval/apps/api/internal/auth"
	"github.com/lzyu/QuickEval/apps/api/internal/catalog"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/health"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/middleware"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/requestid"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/response"
	"github.com/lzyu/QuickEval/apps/api/internal/user"
)

type Dependencies struct {
	Logger         *slog.Logger
	Health         health.Handler
	Auth           auth.Handler
	AuthMiddleware auth.Middleware
	Users          user.Handler
	Catalog        catalog.Handler
	Audit          audit.Handler
}

func NewRouter(dependencies Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(
		requestid.Middleware(),
		middleware.AccessLog(dependencies.Logger),
		middleware.Recovery(dependencies.Logger),
		middleware.SecurityHeaders(),
	)

	router.GET("/health/live", dependencies.Health.Live)
	router.GET("/health/ready", dependencies.Health.Ready)

	api := router.Group("/api/v1")
	api.POST("/auth/login", dependencies.Auth.Login)

	protected := api.Group("")
	protected.Use(dependencies.AuthMiddleware.Required())
	protected.GET("/auth/session", dependencies.Auth.Session)

	mutating := protected.Group("")
	mutating.Use(dependencies.AuthMiddleware.CSRF())
	mutating.DELETE("/auth/session", dependencies.Auth.Logout)
	mutating.POST("/auth/change-password", dependencies.Auth.ChangePassword)

	protected.GET("/evaluation-targets", dependencies.Catalog.ListTargets)
	protected.GET("/evaluation-targets/:target_id", dependencies.Catalog.GetTarget)
	protected.GET("/scenarios", dependencies.Catalog.ListScenarios)
	protected.GET("/scenarios/:scenario_id", dependencies.Catalog.GetScenario)
	protected.GET("/scenarios/:scenario_id/case-tags", dependencies.Catalog.ListCaseTags)
	protected.GET("/issue-tags", dependencies.Catalog.ListIssueTags)

	adminRead := protected.Group("")
	adminRead.Use(auth.RequireAdmin())
	adminRead.GET("/users", dependencies.Users.List)
	adminRead.GET("/users/:user_id", dependencies.Users.Get)
	adminRead.GET("/audit-logs", dependencies.Audit.List)

	admin := mutating.Group("")
	admin.Use(auth.RequireAdmin())
	admin.POST("/users", dependencies.Users.Create)
	admin.PUT("/users/:user_id", dependencies.Users.Update)
	admin.POST("/users/:user_id/disable", dependencies.Users.Disable)
	admin.POST("/users/:user_id/enable", dependencies.Users.Enable)
	admin.POST("/users/:user_id/reset-password", dependencies.Users.ResetPassword)

	admin.POST("/evaluation-targets", dependencies.Catalog.CreateTarget)
	admin.PUT("/evaluation-targets/:target_id", dependencies.Catalog.UpdateTarget)
	admin.POST("/evaluation-targets/:target_id/disable", dependencies.Catalog.DisableTarget)
	admin.POST("/evaluation-targets/:target_id/enable", dependencies.Catalog.EnableTarget)
	admin.POST("/scenarios", dependencies.Catalog.CreateScenario)
	admin.PUT("/scenarios/:scenario_id", dependencies.Catalog.UpdateScenario)
	admin.POST("/scenarios/:scenario_id/disable", dependencies.Catalog.DisableScenario)
	admin.POST("/scenarios/:scenario_id/enable", dependencies.Catalog.EnableScenario)
	admin.POST("/scenarios/:scenario_id/case-tags", dependencies.Catalog.CreateCaseTag)
	admin.PUT("/case-tags/:tag_id", dependencies.Catalog.UpdateCaseTag)
	admin.POST("/case-tags/:tag_id/disable", dependencies.Catalog.DisableCaseTag)
	admin.POST("/case-tags/:tag_id/enable", dependencies.Catalog.EnableCaseTag)
	admin.PUT("/scenarios/:scenario_id/case-tags/reorder", dependencies.Catalog.ReorderCaseTags)
	admin.POST("/issue-tags", dependencies.Catalog.CreateIssueTag)
	admin.PUT("/issue-tags/:tag_id", dependencies.Catalog.UpdateIssueTag)
	admin.POST("/issue-tags/:tag_id/disable", dependencies.Catalog.DisableIssueTag)
	admin.POST("/issue-tags/:tag_id/enable", dependencies.Catalog.EnableIssueTag)
	admin.PUT("/issue-tags/reorder", dependencies.Catalog.ReorderIssueTags)

	router.NoRoute(response.NotFound)

	return router
}
