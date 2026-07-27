package config

import (
	"path/filepath"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestMySQLDSNForcesUTCAndParseTime(t *testing.T) {
	cfg := Config{
		MySQL: MySQLConfig{
			Host:       "db.internal",
			Port:       3307,
			User:       "quickeval",
			Password:   "secret",
			Database:   "quickeval",
			Parameters: "charset=utf8mb4&parseTime=false",
		},
	}

	dsn := cfg.MySQLDSN()
	parsed, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse generated DSN: %v", err)
	}
	if !parsed.ParseTime {
		t.Fatal("ParseTime = false, want true")
	}
	if parsed.Loc.String() != "UTC" {
		t.Fatalf("Loc = %q, want UTC", parsed.Loc)
	}
	if parsed.MultiStatements {
		t.Fatal("application DSN unexpectedly enables multi statements")
	}
}

func TestMySQLMigrationDSNEnablesMultiStatements(t *testing.T) {
	cfg := Config{
		MySQL: MySQLConfig{
			Host:     "db.internal",
			Port:     3306,
			User:     "quickeval",
			Password: "secret",
			Database: "quickeval",
		},
	}

	parsed, err := mysqlDriver.ParseDSN(cfg.MySQLMigrationDSN())
	if err != nil {
		t.Fatalf("parse generated migration DSN: %v", err)
	}
	if !parsed.MultiStatements {
		t.Fatal("migration DSN does not enable multi statements")
	}
}

func TestResolvePathStaysUnderBaseForValidatedLocalPath(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "opt", "quickeval")
	got := ResolvePath(base, "uploads")
	want := filepath.Join(base, "uploads")
	if got != want {
		t.Fatalf("ResolvePath() = %q, want %q", got, want)
	}
}
