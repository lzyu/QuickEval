package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/lzyu/QuickEval/apps/api/internal/attachment"
	"github.com/lzyu/QuickEval/apps/api/internal/audit"
	"github.com/lzyu/QuickEval/apps/api/internal/auth"
	"github.com/lzyu/QuickEval/apps/api/internal/badcase"
	"github.com/lzyu/QuickEval/apps/api/internal/catalog"
	"github.com/lzyu/QuickEval/apps/api/internal/config"
	"github.com/lzyu/QuickEval/apps/api/internal/dataset"
	"github.com/lzyu/QuickEval/apps/api/internal/evaluation"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/health"
	"github.com/lzyu/QuickEval/apps/api/internal/platform/cache"
	"github.com/lzyu/QuickEval/apps/api/internal/platform/database"
	"github.com/lzyu/QuickEval/apps/api/internal/platform/logging"
	"github.com/lzyu/QuickEval/apps/api/internal/runtimepath"
	"github.com/lzyu/QuickEval/apps/api/internal/user"
)

func main() {
	if err := run(); err != nil {
		log.Printf("quickeval server stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	baseDir, err := runtimepath.BaseDir()
	if err != nil {
		return err
	}

	cfg, err := config.Load(baseDir)
	if err != nil {
		return err
	}

	logger, logFile, err := logging.New(baseDir, cfg.Log.File, cfg.Log.Level)
	if err != nil {
		return err
	}
	defer logFile.Close()
	slog.SetDefault(logger)

	ctx := context.Background()
	mysqlDB, err := database.OpenMySQL(ctx, cfg)
	if err != nil {
		return err
	}
	defer database.CloseMySQL(mysqlDB)

	redisClient, err := cache.OpenRedis(ctx, cfg)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	sqlDB, err := mysqlDB.DB()
	if err != nil {
		return err
	}
	healthHandler := health.NewHandler(
		sqlDB.PingContext,
		func(ctx context.Context) error {
			return redisClient.Ping(ctx).Err()
		},
	)
	sessionStore := auth.NewSessionStore(
		redisClient,
		cfg.Security.SessionSecret,
		cfg.Security.SessionTTL,
	)
	userRepository := user.NewRepository(mysqlDB)
	passwordHasher := user.NewBcryptHasher(12)
	loginRateLimiter := auth.NewLoginRateLimiter(
		redisClient,
		cfg.Security.LoginMaxAttempts,
		cfg.Security.LoginWindow,
	)
	authService := auth.NewService(
		userRepository,
		auth.NewLocalIdentityProvider(
			userRepository,
			passwordHasher,
			cfg.Security.PasswordMinLength,
		),
		sessionStore,
		loginRateLimiter,
	)
	auditRecorder := audit.NewRecorder(mysqlDB)
	userService := user.NewService(
		userRepository,
		passwordHasher,
		sessionStore,
		cfg.Security.PasswordMinLength,
	)
	catalogRepository := catalog.NewRepository(mysqlDB)
	datasetRepository := dataset.NewRepository(mysqlDB)
	evaluationRepository := evaluation.NewRepository(mysqlDB)
	attachmentStorage, err := attachment.NewStorage(
		config.ResolvePath(baseDir, cfg.Paths.Uploads),
		cfg.Upload.MaxFileSize,
		cfg.Upload.AllowedMediaTypes,
	)
	if err != nil {
		return err
	}
	attachmentRepository := attachment.NewRepository(mysqlDB)
	badcaseRepository := badcase.NewRepository(mysqlDB)

	router := httpapi.NewRouter(httpapi.Dependencies{
		Logger:         logger,
		Health:         healthHandler,
		Auth:           auth.NewHandler(authService, cfg),
		AuthMiddleware: auth.NewMiddleware(cfg.Security.SessionCookie, sessionStore, userRepository),
		Users:          user.NewHandler(userService, userRepository, auditRecorder, logger),
		Catalog: catalog.NewHandler(
			catalog.NewService(catalogRepository),
			catalogRepository,
			auditRecorder,
			logger,
		),
		Datasets: dataset.NewHandler(
			dataset.NewService(datasetRepository),
			datasetRepository,
			dataset.NewImportPreviewStore(redisClient, cfg.CSV.ImportPreviewTTL),
			auditRecorder,
			logger,
		),
		Evaluations: evaluation.NewHandler(
			evaluation.NewService(evaluationRepository),
			evaluationRepository,
			evaluation.NewIdempotencyStore(redisClient),
			auditRecorder,
			logger,
		),
		Attachments: attachment.NewHandler(
			attachment.NewService(
				attachmentRepository,
				attachmentStorage,
				cfg.Upload.MaxFilesPerOwner,
			),
			attachment.NewIdempotencyStore(redisClient),
			logger,
			cfg.Upload.MaxFileSize,
			cfg.Upload.MaxFilesPerOwner,
		),
		Badcases: badcase.NewHandler(
			badcase.NewService(badcaseRepository, evaluationRepository),
			badcaseRepository,
			badcase.NewIdempotencyStore(redisClient),
			auditRecorder,
			logger,
		),
		Audit: audit.NewHandler(auditRecorder),
	})

	server := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server starting", "address", cfg.HTTP.Address, "environment", cfg.App.Environment)
		serverErrors <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case receivedSignal := <-signals:
		logger.Info("shutdown signal received", "signal", receivedSignal.String())
	case serverErr := <-serverErrors:
		if !errors.Is(serverErr, http.ErrServerClosed) {
			return serverErr
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("server stopped")
	return nil
}
