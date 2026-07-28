package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/attachment"
	"github.com/lzyu/QuickEval/apps/api/internal/audit"
	"github.com/lzyu/QuickEval/apps/api/internal/auth"
	"github.com/lzyu/QuickEval/apps/api/internal/badcase"
	"github.com/lzyu/QuickEval/apps/api/internal/catalog"
	"github.com/lzyu/QuickEval/apps/api/internal/dataset"
	"github.com/lzyu/QuickEval/apps/api/internal/evaluation"
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
	Datasets       dataset.Handler
	Evaluations    evaluation.Handler
	Attachments    attachment.Handler
	Badcases       badcase.Handler
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
	mutating.POST("/evaluation-runs", dependencies.Evaluations.CreateRun)
	mutating.PATCH("/evaluation-runs/:run_id", dependencies.Evaluations.UpdateRun)
	mutating.DELETE("/evaluation-runs/:run_id", dependencies.Evaluations.DeleteRun)
	mutating.POST("/evaluation-runs/:run_id/complete", dependencies.Evaluations.CompleteRun)
	mutating.POST("/evaluation-runs/:run_id/reopen", dependencies.Evaluations.ReopenRun)
	mutating.POST("/evaluation-runs/:run_id/void", dependencies.Evaluations.VoidRun)
	mutating.PATCH("/case-results/:result_id", dependencies.Evaluations.UpdateResult)
	mutating.POST("/case-results/:result_id/mark-badcase", dependencies.Badcases.MarkEvaluation)
	mutating.POST("/case-results/:result_id/attachments", dependencies.Attachments.UploadResult)
	mutating.POST("/badcases/:badcase_id/attachments", dependencies.Attachments.UploadBadcase)
	mutating.DELETE("/attachments/:attachment_id", dependencies.Attachments.Delete)
	mutating.POST("/case-results/:result_id/attachments/reorder", dependencies.Attachments.ReorderResult)
	mutating.POST("/badcases/:badcase_id/attachments/reorder", dependencies.Attachments.ReorderBadcase)

	protected.GET("/evaluation-targets", dependencies.Catalog.ListTargets)
	protected.GET("/evaluation-targets/:target_id", dependencies.Catalog.GetTarget)
	protected.GET("/scenarios", dependencies.Catalog.ListScenarios)
	protected.GET("/scenarios/:scenario_id", dependencies.Catalog.GetScenario)
	protected.GET("/scenarios/:scenario_id/case-tags", dependencies.Catalog.ListCaseTags)
	protected.GET("/issue-tags", dependencies.Catalog.ListIssueTags)
	protected.GET("/datasets", dependencies.Datasets.ListDatasets)
	protected.GET("/datasets/:dataset_id", dependencies.Datasets.GetDataset)
	protected.GET("/datasets/:dataset_id/versions", dependencies.Datasets.ListVersions)
	protected.GET("/dataset-versions/:version_id", dependencies.Datasets.GetVersion)
	protected.GET("/dataset-versions/:version_id/cases", dependencies.Datasets.ListCases)
	protected.GET("/version-cases/:case_id", dependencies.Datasets.GetCase)
	protected.GET("/case-import-template.csv", dependencies.Datasets.ImportTemplate)
	protected.GET("/dataset-versions/:version_id/cases.csv", dependencies.Datasets.ExportCases)
	protected.GET("/evaluation-runs", dependencies.Evaluations.ListRuns)
	protected.GET("/evaluation-runs/:run_id", dependencies.Evaluations.GetRun)
	protected.GET("/evaluation-runs/:run_id/case-results", dependencies.Evaluations.ListResults)
	protected.GET("/pages/evaluation-runs/:run_id/workbench", dependencies.Evaluations.Workbench)
	protected.GET("/case-results/:result_id", dependencies.Evaluations.GetResult)
	protected.GET("/badcases", dependencies.Badcases.List)
	protected.GET("/badcases/:badcase_id", dependencies.Badcases.Get)
	protected.GET("/pages/badcases/:badcase_id", dependencies.Badcases.Get)
	protected.GET("/attachments/:attachment_id/content", dependencies.Attachments.Content)

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
	admin.POST("/datasets", dependencies.Datasets.CreateDataset)
	admin.PATCH("/datasets/:dataset_id", dependencies.Datasets.UpdateDataset)
	admin.POST("/datasets/:dataset_id/archive", dependencies.Datasets.ArchiveDataset)
	admin.POST("/datasets/:dataset_id/restore", dependencies.Datasets.RestoreDataset)
	admin.POST("/datasets/:dataset_id/drafts", dependencies.Datasets.CreateDraft)
	admin.DELETE("/dataset-versions/:version_id", dependencies.Datasets.DeleteDraft)
	admin.POST("/dataset-versions/:version_id/publish", dependencies.Datasets.PublishVersion)
	admin.POST("/dataset-versions/:version_id/archive", dependencies.Datasets.ArchiveVersion)
	admin.POST("/dataset-versions/:version_id/cases", dependencies.Datasets.CreateCase)
	admin.PATCH("/version-cases/:case_id", dependencies.Datasets.UpdateCase)
	admin.DELETE("/version-cases/:case_id", dependencies.Datasets.DeleteCase)
	admin.POST("/dataset-versions/:version_id/cases/reorder", dependencies.Datasets.ReorderCases)
	admin.POST("/dataset-versions/:version_id/case-imports/preview", dependencies.Datasets.PreviewImport)
	admin.POST("/dataset-versions/:version_id/case-imports/commit", dependencies.Datasets.CommitImport)

	router.NoRoute(response.NotFound)

	return router
}
