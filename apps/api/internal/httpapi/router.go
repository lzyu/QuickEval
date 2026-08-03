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
	"github.com/lzyu/QuickEval/apps/api/internal/reporting"
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
	Reporting      reporting.Handler
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

	authenticated := api.Group("")
	authenticated.Use(dependencies.AuthMiddleware.Required())
	authenticated.GET("/auth/session", dependencies.Auth.Session)

	authMutating := authenticated.Group("")
	authMutating.Use(dependencies.AuthMiddleware.CSRF())
	authMutating.DELETE("/auth/session", dependencies.Auth.Logout)
	authMutating.POST("/auth/change-password", dependencies.Auth.ChangePassword)

	protected := api.Group("")
	protected.Use(
		dependencies.AuthMiddleware.Required(),
		auth.RequirePasswordChangeComplete(),
	)

	mutating := protected.Group("")
	mutating.Use(dependencies.AuthMiddleware.CSRF())
	mutating.POST("/evaluation-runs", dependencies.Evaluations.CreateRun)
	mutating.PATCH("/evaluation-runs/:run_id", dependencies.Evaluations.UpdateRun)
	mutating.DELETE("/evaluation-runs/:run_id", dependencies.Evaluations.DeleteRun)
	mutating.POST("/evaluation-runs/:run_id/complete", dependencies.Evaluations.CompleteRun)
	mutating.POST("/evaluation-runs/:run_id/reopen", dependencies.Evaluations.ReopenRun)
	mutating.POST("/evaluation-runs/:run_id/void", dependencies.Evaluations.VoidRun)
	mutating.PATCH("/case-results/:result_id", dependencies.Evaluations.UpdateResult)
	mutating.POST("/case-results/:result_id/mark-badcase", dependencies.Badcases.MarkEvaluation)
	mutating.POST("/badcases", dependencies.Badcases.CreateBusiness)
	mutating.PATCH("/badcases/:badcase_id", dependencies.Badcases.UpdateBusiness)
	mutating.PUT("/badcases/:badcase_id/issue-tags", dependencies.Badcases.UpdateIssueTags)
	mutating.POST("/badcases/:badcase_id/assign", dependencies.Badcases.Assign)
	mutating.POST("/badcases/:badcase_id/unassign", dependencies.Badcases.Unassign)
	mutating.POST("/badcases/:badcase_id/start-processing", dependencies.Badcases.StartProcessing)
	mutating.POST("/badcases/:badcase_id/resolve", dependencies.Badcases.Resolve)
	mutating.POST("/badcases/:badcase_id/defer", dependencies.Badcases.Defer)
	mutating.POST("/badcases/:badcase_id/reopen", dependencies.Badcases.Reopen)
	mutating.POST("/badcases/:badcase_id/add-note", dependencies.Badcases.AddNote)
	mutating.POST("/badcases/:badcase_id/invalidate", dependencies.Badcases.Invalidate)
	mutating.POST("/badcases/:badcase_id/reactivate", dependencies.Badcases.Reactivate)
	mutating.POST("/case-results/:result_id/attachments", dependencies.Attachments.UploadResult)
	mutating.POST("/badcases/:badcase_id/attachments", dependencies.Attachments.UploadBadcase)
	mutating.DELETE("/attachments/:attachment_id", dependencies.Attachments.Delete)
	mutating.POST("/case-results/:result_id/attachments/reorder", dependencies.Attachments.ReorderResult)
	mutating.POST("/badcases/:badcase_id/attachments/reorder", dependencies.Attachments.ReorderBadcase)

	protected.GET("/evaluation-targets", dependencies.Catalog.ListTargets)
	protected.GET("/evaluation-targets/:target_id", dependencies.Catalog.GetTarget)
	protected.GET("/scenarios", dependencies.Catalog.ListScenarios)
	protected.GET("/scenarios/:scenario_id", dependencies.Catalog.GetScenario)
	protected.GET("/evaluation-targets/:target_id/case-tags", dependencies.Catalog.ListCaseTags)
	protected.GET("/evaluation-targets/:target_id/available-case-tags", dependencies.Catalog.ListAvailableCaseTags)
	protected.GET("/case-tags", dependencies.Catalog.ListManagedCaseTags)
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
	protected.GET("/badcase-options", dependencies.Badcases.Options)
	protected.GET("/badcases/:badcase_id", dependencies.Badcases.Get)
	protected.GET("/pages/badcases/:badcase_id", dependencies.Badcases.Page)
	protected.GET("/attachments/:attachment_id/content", dependencies.Attachments.Content)
	protected.GET("/pages/home", dependencies.Reporting.Home)
	protected.GET("/pages/dashboard", dependencies.Reporting.Dashboard)
	protected.GET("/pages/datasets/:dataset_id/version-comparison", dependencies.Reporting.VersionComparison)
	protected.GET("/evaluation-results", dependencies.Reporting.EvaluationResults)
	protected.GET("/search", dependencies.Reporting.Search)
	protected.GET("/exports/evaluation-results.csv", dependencies.Reporting.ExportEvaluationResults)
	protected.GET("/exports/badcases.csv", dependencies.Reporting.ExportBadcases)
	protected.GET("/exports/badcase-distribution.csv", dependencies.Reporting.ExportBadcaseDistribution)

	operationsRead := protected.Group("")
	operationsRead.Use(auth.RequireOperationsAdmin())
	operationsRead.GET("/audit-logs", dependencies.Audit.List)

	superAdminRead := protected.Group("")
	superAdminRead.Use(auth.RequireSuperAdmin())
	superAdminRead.GET("/users", dependencies.Users.List)
	superAdminRead.GET("/users/:user_id", dependencies.Users.Get)

	superAdmin := mutating.Group("")
	superAdmin.Use(auth.RequireSuperAdmin())
	superAdmin.POST("/users", dependencies.Users.Create)
	superAdmin.PUT("/users/:user_id", dependencies.Users.Update)
	superAdmin.POST("/users/:user_id/disable", dependencies.Users.Disable)
	superAdmin.POST("/users/:user_id/enable", dependencies.Users.Enable)
	superAdmin.POST("/users/:user_id/reset-password", dependencies.Users.ResetPassword)

	operationsAdmin := mutating.Group("")
	operationsAdmin.Use(auth.RequireOperationsAdmin())
	operationsAdmin.POST("/evaluation-targets", dependencies.Catalog.CreateTarget)
	operationsAdmin.PUT("/evaluation-targets/:target_id", dependencies.Catalog.UpdateTarget)
	operationsAdmin.POST("/evaluation-targets/:target_id/disable", dependencies.Catalog.DisableTarget)
	operationsAdmin.POST("/evaluation-targets/:target_id/enable", dependencies.Catalog.EnableTarget)
	operationsAdmin.POST("/scenarios", dependencies.Catalog.CreateScenario)
	operationsAdmin.PUT("/scenarios/:scenario_id", dependencies.Catalog.UpdateScenario)
	operationsAdmin.POST("/scenarios/:scenario_id/disable", dependencies.Catalog.DisableScenario)
	operationsAdmin.POST("/scenarios/:scenario_id/enable", dependencies.Catalog.EnableScenario)
	operationsAdmin.POST("/evaluation-targets/:target_id/case-tags", dependencies.Catalog.CreateCaseTag)
	operationsAdmin.POST("/case-tags", dependencies.Catalog.CreateManagedCaseTag)
	operationsAdmin.PUT("/case-tags/:tag_id", dependencies.Catalog.UpdateCaseTag)
	operationsAdmin.POST("/case-tags/:tag_id/disable", dependencies.Catalog.DisableCaseTag)
	operationsAdmin.POST("/case-tags/:tag_id/enable", dependencies.Catalog.EnableCaseTag)
	operationsAdmin.PUT("/evaluation-targets/:target_id/case-tags/reorder", dependencies.Catalog.ReorderCaseTags)
	operationsAdmin.POST("/issue-tags", dependencies.Catalog.CreateIssueTag)
	operationsAdmin.PUT("/issue-tags/:tag_id", dependencies.Catalog.UpdateIssueTag)
	operationsAdmin.POST("/issue-tags/:tag_id/disable", dependencies.Catalog.DisableIssueTag)
	operationsAdmin.POST("/issue-tags/:tag_id/enable", dependencies.Catalog.EnableIssueTag)
	operationsAdmin.PUT("/issue-tags/reorder", dependencies.Catalog.ReorderIssueTags)
	operationsAdmin.POST("/datasets", dependencies.Datasets.CreateDataset)
	operationsAdmin.PATCH("/datasets/:dataset_id", dependencies.Datasets.UpdateDataset)
	operationsAdmin.POST("/datasets/:dataset_id/archive", dependencies.Datasets.ArchiveDataset)
	operationsAdmin.POST("/datasets/:dataset_id/restore", dependencies.Datasets.RestoreDataset)
	operationsAdmin.POST("/datasets/:dataset_id/drafts", dependencies.Datasets.CreateDraft)
	operationsAdmin.DELETE("/dataset-versions/:version_id", dependencies.Datasets.DeleteDraft)
	operationsAdmin.POST("/dataset-versions/:version_id/publish", dependencies.Datasets.PublishVersion)
	operationsAdmin.POST("/dataset-versions/:version_id/archive", dependencies.Datasets.ArchiveVersion)
	operationsAdmin.POST("/dataset-versions/:version_id/cases", dependencies.Datasets.CreateCase)
	operationsAdmin.PATCH("/version-cases/:case_id", dependencies.Datasets.UpdateCase)
	operationsAdmin.DELETE("/version-cases/:case_id", dependencies.Datasets.DeleteCase)
	operationsAdmin.POST("/dataset-versions/:version_id/cases/reorder", dependencies.Datasets.ReorderCases)
	operationsAdmin.POST("/dataset-versions/:version_id/case-imports/preview", dependencies.Datasets.PreviewImport)
	operationsAdmin.POST("/dataset-versions/:version_id/case-imports/commit", dependencies.Datasets.CommitImport)

	router.NoRoute(response.NotFound)

	return router
}
