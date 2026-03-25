package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultStorageRoot = "/data/openshare"

type Config struct {
	Server    ServerConfig    `json:"server"`
	Database  DatabaseConfig  `json:"database"`
	Storage   StorageConfig   `json:"storage"`
	Upload    UploadConfig    `json:"upload"`
	Session   SessionConfig   `json:"session"`
	RateLimit RateLimitConfig `json:"rate_limit"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type DatabaseConfig struct {
	Path      string      `json:"path"`
	LogLevel  string      `json:"log_level"`
	Pragmas   []SQLPragma `json:"pragmas"`
	EnableWAL bool        `json:"enable_wal"`
}

type SQLPragma struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type StorageConfig struct {
	Root    string `json:"root"`
	Staging string `json:"staging"`
	Trash   string `json:"trash"`
}

type UploadConfig struct {
	MaxFileSizeBytes       int64    `json:"max_file_size_bytes"`
	MaxBatchTotalSizeBytes int64    `json:"max_batch_total_size_bytes"`
	AllowedExtensions      []string `json:"allowed_extensions"`
	AllowedMIMETypes       []string `json:"allowed_mime_types"`
	MaxTitleLength         int      `json:"max_title_length"`
	MaxDescriptionLength   int      `json:"max_description_length"`
	ReceiptCodeLength      int      `json:"receipt_code_length"`
}

type SessionConfig struct {
	Name            string `json:"name"`
	Secret          string `json:"secret"`
	Path            string `json:"path"`
	MaxAgeSeconds   int    `json:"max_age_seconds"`
	Secure          bool   `json:"secure"`
	HTTPOnly        bool   `json:"http_only"`
	SameSite        string `json:"same_site"`
	RenewWindowSecs int    `json:"renew_window_seconds"`
}

type RateLimitConfig struct {
	Upload RateLimitRule `json:"upload"`
	Search RateLimitRule `json:"search"`
}

type RateLimitRule struct {
	Enabled bool `json:"enabled"`
	Limit   int  `json:"limit"`
	Window  int  `json:"window_seconds"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Path:      filepath.Join(defaultStorageRoot, "openshare.db"),
			LogLevel:  "warn",
			EnableWAL: true,
			Pragmas: []SQLPragma{
				{Name: "foreign_keys", Value: "ON"},
				{Name: "busy_timeout", Value: "5000"},
			},
		},
		Storage: StorageConfig{
			Root:    defaultStorageRoot,
			Staging: "staging",
			Trash:   "trash",
		},
		Upload: UploadConfig{
			MaxFileSizeBytes:       5 << 30,
			MaxBatchTotalSizeBytes: 20 << 30,
			AllowedExtensions:      []string{},
			AllowedMIMETypes:       []string{},
			MaxTitleLength:         200,
			MaxDescriptionLength:   4000,
			ReceiptCodeLength:      12,
		},
		Session: SessionConfig{
			Name:            "openshare_session",
			Secret:          "replace-this-in-local-config",
			Path:            "/",
			MaxAgeSeconds:   604800, // 7d
			Secure:          false,
			HTTPOnly:        true,
			SameSite:        "lax",
			RenewWindowSecs: 604800, // 7d sliding window
		},
		RateLimit: RateLimitConfig{
			Upload: RateLimitRule{Enabled: true, Limit: 10, Window: 60},
			Search: RateLimitRule{Enabled: true, Limit: 60, Window: 60},
		},
	}
}

func Load(defaultPath, localPath string) (Config, error) {
	cfg := Default()

	if err := mergeFromFile(&cfg, defaultPath, false); err != nil {
		return Config{}, err
	}

	if err := mergeFromFile(&cfg, localPath, true); err != nil {
		return Config{}, err
	}

	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}

	cfg.normalize()

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func mergeFromFile(cfg *Config, path string, optional bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if optional && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		if optional {
			return nil
		}
		return fmt.Errorf("config file %q is empty", path)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}

	return nil
}

func applyEnv(cfg *Config) error {
	var errs []error

	overrideString("OPENSHARE_SERVER_HOST", &cfg.Server.Host)
	overrideInt("OPENSHARE_SERVER_PORT", &cfg.Server.Port, &errs)
	overrideString("OPENSHARE_DATABASE_PATH", &cfg.Database.Path)
	overrideString("OPENSHARE_DATABASE_LOG_LEVEL", &cfg.Database.LogLevel)
	overrideBool("OPENSHARE_DATABASE_ENABLE_WAL", &cfg.Database.EnableWAL, &errs)
	overrideString("OPENSHARE_STORAGE_ROOT", &cfg.Storage.Root)
	overrideString("OPENSHARE_STORAGE_STAGING", &cfg.Storage.Staging)
	overrideString("OPENSHARE_STORAGE_TRASH", &cfg.Storage.Trash)
	overrideInt64("OPENSHARE_UPLOAD_MAX_FILE_SIZE_BYTES", &cfg.Upload.MaxFileSizeBytes, &errs)
	overrideInt64("OPENSHARE_UPLOAD_MAX_BATCH_TOTAL_SIZE_BYTES", &cfg.Upload.MaxBatchTotalSizeBytes, &errs)
	overrideInt("OPENSHARE_UPLOAD_MAX_TITLE_LENGTH", &cfg.Upload.MaxTitleLength, &errs)
	overrideInt("OPENSHARE_UPLOAD_MAX_DESCRIPTION_LENGTH", &cfg.Upload.MaxDescriptionLength, &errs)
	overrideInt("OPENSHARE_UPLOAD_RECEIPT_CODE_LENGTH", &cfg.Upload.ReceiptCodeLength, &errs)
	overrideString("OPENSHARE_SESSION_NAME", &cfg.Session.Name)
	overrideString("OPENSHARE_SESSION_SECRET", &cfg.Session.Secret)
	overrideString("OPENSHARE_SESSION_PATH", &cfg.Session.Path)
	overrideInt("OPENSHARE_SESSION_MAX_AGE_SECONDS", &cfg.Session.MaxAgeSeconds, &errs)
	overrideBool("OPENSHARE_SESSION_SECURE", &cfg.Session.Secure, &errs)
	overrideBool("OPENSHARE_SESSION_HTTP_ONLY", &cfg.Session.HTTPOnly, &errs)
	overrideString("OPENSHARE_SESSION_SAME_SITE", &cfg.Session.SameSite)
	overrideInt("OPENSHARE_SESSION_RENEW_WINDOW_SECONDS", &cfg.Session.RenewWindowSecs, &errs)
	overrideBool("OPENSHARE_RATE_LIMIT_UPLOAD_ENABLED", &cfg.RateLimit.Upload.Enabled, &errs)
	overrideInt("OPENSHARE_RATE_LIMIT_UPLOAD_LIMIT", &cfg.RateLimit.Upload.Limit, &errs)
	overrideInt("OPENSHARE_RATE_LIMIT_UPLOAD_WINDOW_SECONDS", &cfg.RateLimit.Upload.Window, &errs)
	overrideBool("OPENSHARE_RATE_LIMIT_SEARCH_ENABLED", &cfg.RateLimit.Search.Enabled, &errs)
	overrideInt("OPENSHARE_RATE_LIMIT_SEARCH_LIMIT", &cfg.RateLimit.Search.Limit, &errs)
	overrideInt("OPENSHARE_RATE_LIMIT_SEARCH_WINDOW_SECONDS", &cfg.RateLimit.Search.Window, &errs)

	return errors.Join(errs...)
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.Host) == "" {
		return errors.New("server.host must not be empty")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return errors.New("server.port must be between 1 and 65535")
	}

	if c.Database.Path == "" {
		return errors.New("database.path must not be empty")
	}
	switch c.Database.LogLevel {
	case "silent", "error", "warn", "info":
	default:
		return errors.New("database.log_level must be one of: silent, error, warn, info")
	}

	if c.Storage.Root == "" {
		return errors.New("storage.root must not be empty")
	}
	if c.Storage.Staging == "" {
		return errors.New("storage.staging must not be empty")
	}
	if c.Storage.Trash == "" {
		return errors.New("storage.trash must not be empty")
	}
	if c.Upload.MaxFileSizeBytes <= 0 {
		return errors.New("upload.max_file_size_bytes must be greater than 0")
	}
	if c.Upload.MaxBatchTotalSizeBytes < c.Upload.MaxFileSizeBytes {
		return errors.New("upload.max_batch_total_size_bytes must be greater than or equal to upload.max_file_size_bytes")
	}
	if c.Upload.MaxTitleLength <= 0 {
		return errors.New("upload.max_title_length must be greater than 0")
	}
	if c.Upload.MaxDescriptionLength <= 0 {
		return errors.New("upload.max_description_length must be greater than 0")
	}
	if c.Upload.ReceiptCodeLength < 6 || c.Upload.ReceiptCodeLength > 32 {
		return errors.New("upload.receipt_code_length must be between 6 and 32")
	}

	if c.Session.Name == "" {
		return errors.New("session.name must not be empty")
	}
	if c.Session.Secret == "" {
		return errors.New("session.secret must not be empty")
	}
	if c.Session.Secret == "replace-this-in-local-config" {
		return errors.New("session.secret must be overridden in local config or environment")
	}
	if c.Session.MaxAgeSeconds <= 0 {
		return errors.New("session.max_age_seconds must be greater than 0")
	}
	if c.Session.Path == "" || !strings.HasPrefix(c.Session.Path, "/") {
		return errors.New("session.path must start with '/'")
	}
	if c.Session.RenewWindowSecs < 0 || c.Session.RenewWindowSecs > c.Session.MaxAgeSeconds {
		return errors.New("session.renew_window_seconds must be between 0 and session.max_age_seconds")
	}
	if c.Session.SameSite == "none" && !c.Session.Secure {
		return errors.New("session.same_site 'none' requires session.secure=true")
	}

	switch c.Session.SameSite {
	case "lax", "strict", "none":
	default:
		return errors.New("session.same_site must be one of: lax, strict, none")
	}

	if err := validateRateLimit("rate_limit.upload", c.RateLimit.Upload); err != nil {
		return err
	}
	if err := validateRateLimit("rate_limit.search", c.RateLimit.Search); err != nil {
		return err
	}

	return nil
}

func (c ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c *Config) normalize() {
	c.Database.LogLevel = strings.ToLower(strings.TrimSpace(c.Database.LogLevel))
	c.Session.SameSite = strings.ToLower(strings.TrimSpace(c.Session.SameSite))
	c.Session.Path = strings.TrimSpace(c.Session.Path)

	c.Storage.Root = strings.TrimSpace(c.Storage.Root)
	c.Storage.Staging = strings.TrimSpace(c.Storage.Staging)
	c.Storage.Trash = strings.TrimSpace(c.Storage.Trash)
	c.Upload.AllowedExtensions = normalizeStringSlice(c.Upload.AllowedExtensions, true)
	c.Upload.AllowedMIMETypes = normalizeStringSlice(c.Upload.AllowedMIMETypes, true)
}

func validateRateLimit(prefix string, rule RateLimitRule) error {
	if !rule.Enabled {
		return nil
	}
	if rule.Limit <= 0 {
		return fmt.Errorf("%s.limit must be greater than 0", prefix)
	}
	if rule.Window <= 0 {
		return fmt.Errorf("%s.window_seconds must be greater than 0", prefix)
	}
	return nil
}

func overrideString(env string, target *string) {
	if value, ok := os.LookupEnv(env); ok {
		*target = value
	}
}

func overrideInt(env string, target *int, errs *[]error) {
	value, ok := os.LookupEnv(env)
	if !ok {
		return
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("parse env %s: %w", env, err))
		return
	}
	*target = parsed
}

func overrideInt64(env string, target *int64, errs *[]error) {
	value, ok := os.LookupEnv(env)
	if !ok {
		return
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("parse env %s: %w", env, err))
		return
	}
	*target = parsed
}

func overrideBool(env string, target *bool, errs *[]error) {
	value, ok := os.LookupEnv(env)
	if !ok {
		return
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("parse env %s: %w", env, err))
		return
	}
	*target = parsed
}

func normalizeStringSlice(values []string, lower bool) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	return normalized
}
