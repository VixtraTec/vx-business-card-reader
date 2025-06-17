package config

import (
	"fmt"
	"os"
)

type Config struct {
	AWS struct {
		Region    string
		TableName string
	}
	Gemini struct {
		APIKey    string
		ModelName string
	}
}

func Load() (*Config, error) {
	cfg := &Config{}

	// AWS Configuration
	cfg.AWS.Region = getEnvOrDefault("AWS_REGION", "us-east-1")
	cfg.AWS.TableName = getEnvOrDefault("DYNAMODB_TABLE_NAME", "business-cards")

	// Gemini Configuration
	cfg.Gemini.APIKey = os.Getenv("GEMINI_API_KEY")
	if cfg.Gemini.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is required")
	}
	cfg.Gemini.ModelName = getEnvOrDefault("GEMINI_MODEL_NAME", "gemini-1.5-flash")

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
