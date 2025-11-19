package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config stores runtime configuration loaded from environment variables.
type Config struct {
	OpenAIKey      string
	OpenAIEndpoint string
	OpenAIModel    string
	ZAIKey         string
	ZAIBaseURL     string
	ZAIModel       string
	Database       string
	UploadDir      string
}

// Load reads configuration from the environment, providing sensible defaults.
func Load() Config {
	// Load .env file if it exists (useful for development)
	_ = godotenv.Load()
	cfg := Config{
		OpenAIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenAIEndpoint: getEnv("OPENAI_API_ENDPOINT", "https://api.openai.com/v1"),
		OpenAIModel:    getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		ZAIKey:         os.Getenv("Z_AI_API_KEY"),
		ZAIBaseURL:     getEnv("Z_AI_BASE_URL", "https://open.bigmodel.cn/api/paas/v4/"),
		ZAIModel:       getEnv("Z_AI_VISION_MODEL", "glm-4.5v"),
		Database:       getEnv("DATABASE_PATH", "./data/flashcards.db"),
		UploadDir:      getEnv("UPLOAD_DIR", "./static/uploads"),
	}

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("failed to ensure upload dir %s: %v", cfg.UploadDir, err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Database), 0o755); err != nil {
		log.Fatalf("failed to ensure database dir %s: %v", cfg.Database, err)
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
