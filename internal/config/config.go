package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	SQLServer   string
	SQLDatabase string
	SQLUser     string
	SQLPassword string

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3Region    string

	LogDir            string
	RetentionDays     int
	S3RetentionDays   int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		SQLServer:   os.Getenv("SQL_SERVER"),
		SQLDatabase: os.Getenv("SQL_DATABASE"),
		SQLUser:     os.Getenv("SQL_USER"),
		SQLPassword: os.Getenv("SQL_PASSWORD"),

		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3Bucket:    os.Getenv("S3_BUCKET"),
		S3Region:    os.Getenv("S3_REGION"),

		LogDir:          os.Getenv("LOG_DIR"),
		RetentionDays:   getEnvAsInt("RETENTION_DAYS", 30),
		S3RetentionDays: getEnvAsInt("S3_RETENTION_DAYS", 90),
	}

	if cfg.SQLServer == "" {
		return nil, fmt.Errorf("SQL_SERVER is required")
	}
	if cfg.SQLDatabase == "" {
		return nil, fmt.Errorf("SQL_DATABASE is required")
	}
	if cfg.SQLUser == "" {
		return nil, fmt.Errorf("SQL_USER is required")
	}
	if cfg.SQLPassword == "" {
		return nil, fmt.Errorf("SQL_PASSWORD is required")
	}
	if cfg.S3Endpoint == "" {
		return nil, fmt.Errorf("S3_ENDPOINT is required")
	}
	if cfg.S3AccessKey == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY is required")
	}
	if cfg.S3SecretKey == "" {
		return nil, fmt.Errorf("S3_SECRET_KEY is required")
	}
	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required")
	}
	if cfg.S3Region == "" {
		return nil, fmt.Errorf("S3_REGION is required")
	}
	if cfg.LogDir == "" {
		return nil, fmt.Errorf("LOG_DIR is required")
	}

	return cfg, nil
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultVal
}
