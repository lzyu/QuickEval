package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	HTTP     HTTPConfig     `yaml:"http"`
	Log      LogConfig      `yaml:"log"`
	MySQL    MySQLConfig    `yaml:"mysql"`
	Redis    RedisConfig    `yaml:"redis"`
	Paths    PathsConfig    `yaml:"paths"`
	Upload   UploadConfig   `yaml:"upload"`
	Security SecurityConfig `yaml:"security"`
}

type AppConfig struct {
	Environment     string        `yaml:"environment"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type HTTPConfig struct {
	Address           string        `yaml:"address"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
}

type LogConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

type MySQLConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Database        string        `yaml:"database"`
	Parameters      string        `yaml:"parameters"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	Database int    `yaml:"database"`
}

type PathsConfig struct {
	Migrations string `yaml:"migrations"`
	Uploads    string `yaml:"uploads"`
}

type UploadConfig struct {
	MaxFileSize       int64    `yaml:"max_file_size"`
	MaxFilesPerOwner  int      `yaml:"max_files_per_owner"`
	AllowedMediaTypes []string `yaml:"allowed_media_types"`
}

type SecurityConfig struct {
	SessionSecret     string        `yaml:"session_secret"`
	SessionCookie     string        `yaml:"session_cookie"`
	SessionTTL        time.Duration `yaml:"session_ttl"`
	CookieSecure      bool          `yaml:"cookie_secure"`
	LoginMaxAttempts  int           `yaml:"login_max_attempts"`
	LoginWindow       time.Duration `yaml:"login_window"`
	PasswordMinLength int           `yaml:"password_min_length"`
}

type secrets struct {
	MySQL struct {
		Password string `yaml:"password"`
	} `yaml:"mysql"`
	Redis struct {
		Password string `yaml:"password"`
	} `yaml:"redis"`
	Security struct {
		SessionSecret string `yaml:"session_secret"`
	} `yaml:"security"`
}

func Load(baseDir string) (Config, error) {
	configPath := filepath.Join(baseDir, "config", "quickeval.yaml")
	secretsPath := filepath.Join(baseDir, "config", "secrets.yaml")

	var cfg Config
	if err := decodeYAML(configPath, &cfg); err != nil {
		return Config{}, err
	}

	var secretValues secrets
	if err := decodeYAML(secretsPath, &secretValues); err != nil {
		return Config{}, err
	}

	cfg.MySQL.Password = secretValues.MySQL.Password
	cfg.Redis.Password = secretValues.Redis.Password
	cfg.Security.SessionSecret = secretValues.Security.SessionSecret
	applyDefaults(&cfg)

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

func decodeYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.App.Environment == "" {
		cfg.App.Environment = "development"
	}
	if cfg.App.ShutdownTimeout == 0 {
		cfg.App.ShutdownTimeout = 10 * time.Second
	}
	if cfg.HTTP.Address == "" {
		cfg.HTTP.Address = "127.0.0.1:8080"
	}
	if cfg.HTTP.ReadHeaderTimeout == 0 {
		cfg.HTTP.ReadHeaderTimeout = 5 * time.Second
	}
	if cfg.HTTP.ReadTimeout == 0 {
		cfg.HTTP.ReadTimeout = 15 * time.Second
	}
	if cfg.HTTP.WriteTimeout == 0 {
		cfg.HTTP.WriteTimeout = 30 * time.Second
	}
	if cfg.HTTP.IdleTimeout == 0 {
		cfg.HTTP.IdleTimeout = 60 * time.Second
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.File == "" {
		cfg.Log.File = "logs/quickeval.log"
	}
	if cfg.MySQL.Port == 0 {
		cfg.MySQL.Port = 3306
	}
	if cfg.MySQL.Parameters == "" {
		cfg.MySQL.Parameters = "charset=utf8mb4&parseTime=true&loc=UTC"
	}
	if cfg.MySQL.MaxOpenConns == 0 {
		cfg.MySQL.MaxOpenConns = 20
	}
	if cfg.MySQL.MaxIdleConns == 0 {
		cfg.MySQL.MaxIdleConns = 5
	}
	if cfg.MySQL.ConnMaxLifetime == 0 {
		cfg.MySQL.ConnMaxLifetime = 30 * time.Minute
	}
	if cfg.Redis.Port == 0 {
		cfg.Redis.Port = 6379
	}
	if cfg.Paths.Migrations == "" {
		cfg.Paths.Migrations = "migrations"
	}
	if cfg.Paths.Uploads == "" {
		cfg.Paths.Uploads = "uploads"
	}
	if cfg.Upload.MaxFileSize == 0 {
		cfg.Upload.MaxFileSize = 10 * 1024 * 1024
	}
	if cfg.Upload.MaxFilesPerOwner == 0 {
		cfg.Upload.MaxFilesPerOwner = 10
	}
	if len(cfg.Upload.AllowedMediaTypes) == 0 {
		cfg.Upload.AllowedMediaTypes = []string{"image/png", "image/jpeg", "image/webp"}
	}
	if cfg.Security.SessionCookie == "" {
		cfg.Security.SessionCookie = "quickeval_session"
	}
	if cfg.Security.SessionTTL == 0 {
		cfg.Security.SessionTTL = 12 * time.Hour
	}
	if cfg.Security.LoginMaxAttempts == 0 {
		cfg.Security.LoginMaxAttempts = 5
	}
	if cfg.Security.LoginWindow == 0 {
		cfg.Security.LoginWindow = 15 * time.Minute
	}
	if cfg.Security.PasswordMinLength == 0 {
		cfg.Security.PasswordMinLength = 10
	}
}

func (cfg Config) Validate() error {
	var validationErrors []error
	if cfg.MySQL.Host == "" {
		validationErrors = append(validationErrors, errors.New("mysql.host is required"))
	}
	if cfg.MySQL.User == "" {
		validationErrors = append(validationErrors, errors.New("mysql.user is required"))
	}
	if cfg.MySQL.Database == "" {
		validationErrors = append(validationErrors, errors.New("mysql.database is required"))
	}
	if cfg.Redis.Host == "" {
		validationErrors = append(validationErrors, errors.New("redis.host is required"))
	}
	if len(cfg.Security.SessionSecret) < 32 {
		validationErrors = append(validationErrors, errors.New("security.session_secret must contain at least 32 characters"))
	}
	if cfg.Security.SessionTTL <= 0 {
		validationErrors = append(validationErrors, errors.New("security.session_ttl must be positive"))
	}
	if cfg.Security.LoginMaxAttempts <= 0 {
		validationErrors = append(validationErrors, errors.New("security.login_max_attempts must be positive"))
	}
	if cfg.Security.LoginWindow <= 0 {
		validationErrors = append(validationErrors, errors.New("security.login_window must be positive"))
	}
	if cfg.Security.PasswordMinLength < 8 {
		validationErrors = append(validationErrors, errors.New("security.password_min_length must be at least 8"))
	}
	if cfg.Upload.MaxFileSize <= 0 {
		validationErrors = append(validationErrors, errors.New("upload.max_file_size must be positive"))
	}
	if cfg.Upload.MaxFilesPerOwner <= 0 {
		validationErrors = append(validationErrors, errors.New("upload.max_files_per_owner must be positive"))
	}
	if !filepath.IsLocal(cfg.Log.File) {
		validationErrors = append(validationErrors, errors.New("log.file must be a relative local path"))
	}
	if !filepath.IsLocal(cfg.Paths.Migrations) {
		validationErrors = append(validationErrors, errors.New("paths.migrations must be a relative local path"))
	}
	if !filepath.IsLocal(cfg.Paths.Uploads) {
		validationErrors = append(validationErrors, errors.New("paths.uploads must be a relative local path"))
	}
	return errors.Join(validationErrors...)
}

func (cfg Config) MySQLDSN() string {
	return cfg.mysqlDSN(false)
}

func (cfg Config) MySQLMigrationDSN() string {
	return cfg.mysqlDSN(true)
}

func (cfg Config) mysqlDSN(multiStatements bool) string {
	parameters := map[string]string{}
	for _, pair := range strings.Split(cfg.MySQL.Parameters, "&") {
		key, value, found := strings.Cut(pair, "=")
		if found && key != "" && key != "parseTime" && key != "loc" && key != "multiStatements" {
			parameters[key] = value
		}
	}

	driverConfig := mysqlDriver.Config{
		User:            cfg.MySQL.User,
		Passwd:          cfg.MySQL.Password,
		Net:             "tcp",
		Addr:            net.JoinHostPort(cfg.MySQL.Host, strconv.Itoa(cfg.MySQL.Port)),
		DBName:          cfg.MySQL.Database,
		Params:          parameters,
		ParseTime:       true,
		Loc:             time.UTC,
		MultiStatements: multiStatements,
	}
	return driverConfig.FormatDSN()
}

func (cfg Config) RedisAddress() string {
	return net.JoinHostPort(cfg.Redis.Host, strconv.Itoa(cfg.Redis.Port))
}

func ResolvePath(baseDir, configuredPath string) string {
	clean := filepath.Clean(strings.TrimSpace(configuredPath))
	return filepath.Join(baseDir, clean)
}
