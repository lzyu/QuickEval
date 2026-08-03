package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/lzyu/QuickEval/apps/api/internal/audit"
	"github.com/lzyu/QuickEval/apps/api/internal/config"
	"github.com/lzyu/QuickEval/apps/api/internal/platform/database"
	"github.com/lzyu/QuickEval/apps/api/internal/runtimepath"
	"github.com/lzyu/QuickEval/apps/api/internal/user"
)

func main() {
	if err := run(); err != nil {
		log.Printf("bootstrap admin failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	username := flag.String("username", "admin", "local login username")
	displayName := flag.String("display-name", "系统管理员", "display name")
	email := flag.String("email", "", "optional email address")
	flag.Parse()

	password := os.Getenv("QUICKEVAL_BOOTSTRAP_PASSWORD")
	if password == "" {
		return fmt.Errorf("QUICKEVAL_BOOTSTRAP_PASSWORD is required")
	}
	baseDir, err := runtimepath.BaseDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(baseDir)
	if err != nil {
		return err
	}
	db, err := database.OpenMySQL(context.Background(), cfg)
	if err != nil {
		return err
	}
	defer database.CloseMySQL(db)

	var emailValue *string
	if *email != "" {
		emailValue = email
	}
	service := user.NewService(
		user.NewRepository(db),
		user.NewBcryptHasher(12),
		nil,
		cfg.Security.PasswordMinLength,
	)
	account, err := service.BootstrapAdmin(context.Background(), user.BootstrapInput{
		Username:    *username,
		DisplayName: *displayName,
		Email:       emailValue,
		Password:    password,
	})
	if err != nil {
		return err
	}
	recorder := audit.NewRecorder(db)
	if err := recorder.Record(
		context.Background(),
		nil,
		"user.bootstrap_admin",
		"user",
		account.ID,
		nil,
		account.ToPublic(),
		"bootstrap-admin",
		"local",
		"quickeval-cli",
	); err != nil {
		return err
	}
	fmt.Printf("administrator created: %s (%s)\n", account.Username, account.ID.String())
	return nil
}
