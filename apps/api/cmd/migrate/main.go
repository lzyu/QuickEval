package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	mysqlMigration "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/lzyu/QuickEval/apps/api/internal/config"
	"github.com/lzyu/QuickEval/apps/api/internal/runtimepath"
)

func main() {
	if err := run(); err != nil {
		log.Printf("migration failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	direction := flag.String("direction", "up", "migration direction: up or down")
	steps := flag.Int("steps", 0, "number of migration steps; zero means all for up")
	forceVersion := flag.Int(
		"force-version",
		-2,
		"force migration version without running SQL; use -1 for an empty schema",
	)
	flag.Parse()

	baseDir, err := runtimepath.BaseDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(baseDir)
	if err != nil {
		return err
	}

	sqlDB, err := sql.Open("mysql", cfg.MySQLMigrationDSN())
	if err != nil {
		return fmt.Errorf("open mysql for migration: %w", err)
	}
	defer sqlDB.Close()

	driver, err := mysqlMigration.WithInstance(sqlDB, &mysqlMigration.Config{})
	if err != nil {
		return fmt.Errorf("create mysql migration driver: %w", err)
	}

	migrationsDir := config.ResolvePath(baseDir, cfg.Paths.Migrations)
	sourceURL := "file://" + filepath.ToSlash(migrationsDir)
	runner, err := migrate.NewWithDatabaseInstance(sourceURL, cfg.MySQL.Database, driver)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer runner.Close()

	if *forceVersion >= -1 {
		if err := runner.Force(*forceVersion); err != nil {
			return fmt.Errorf("force migration version: %w", err)
		}
		log.Printf("migration version forced to %d", *forceVersion)
		return nil
	}

	switch *direction {
	case "up":
		if *steps > 0 {
			err = runner.Steps(*steps)
		} else {
			err = runner.Up()
		}
	case "down":
		if *steps <= 0 {
			return errors.New("down migration requires a positive -steps value")
		}
		err = runner.Steps(-*steps)
	default:
		return fmt.Errorf("unsupported migration direction %q", *direction)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		log.Print("database schema is already up to date")
		return nil
	}
	if err != nil {
		return err
	}

	version, dirty, versionErr := runner.Version()
	if errors.Is(versionErr, migrate.ErrNilVersion) {
		log.Print("migration complete: schema is at the empty version")
	} else if versionErr == nil {
		log.Printf("migration complete: version=%d dirty=%t", version, dirty)
	}
	return nil
}
