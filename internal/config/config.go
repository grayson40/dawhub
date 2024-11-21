package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Server ServerConfig
	DB     DBConfig
	Minio  MinioConfig
}

type ServerConfig struct {
	Port          string
	SessionSecret string
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	return &Config{
		Server: ServerConfig{
			Port:          getEnv("APP_PORT", "8080"),
			SessionSecret: getEnv("SESSION_SECRET", "default-secret-key"),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "db"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", ""),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", ""),
		},
		Minio: MinioConfig{
			Endpoint:  getEnv("MINIO_ENDPOINT", ""),
			AccessKey: getEnv("MINIO_ACCESS_KEY", ""),
			SecretKey: getEnv("MINIO_SECRET_KEY", ""),
			Bucket:    getEnv("MINIO_BUCKET", ""),
			UseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
		},
	}, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
