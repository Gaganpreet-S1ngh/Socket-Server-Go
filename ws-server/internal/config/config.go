package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	Port           string
	JWTSecret      string
	JWTIssuer      string
	AllowedOrigins []string
	Logger         *zap.Logger
}

func LoadConfig() *Config {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(logger)
	}

	if os.Getenv("ENV") != "prod" {
		if err := godotenv.Load(); err != nil {
			logger.Warn("failed to load .env", zap.Error(err))
		}
	}

	return &Config{
		Port:          os.Getenv("PORT"),
		JWTSecret: os.Getenv("JWT_SECRET"),
		JWTIssuer: os.Getenv("JWT_ISSUER"),
		AllowedOrigins:     getList("WS_ALLOWED_ORIGINS", []string{}),

	}
}


//==========================================//
//             PRIVATE FUNCTIONS            //
//==========================================//

func (c *Config) validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("WS_JWT_SECRET is required and must match the Node.js backend's signing secret")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("WS_JWT_SECRET should be at least 32 bytes for HMAC-SHA256 security")
	}

	if len(c.AllowedOrigins) == 0 {
		return fmt.Errorf("WS_ALLOWED_ORIGINS is required (comma-separated list, e.g. https://yourapp.com) - refusing to run wide open in production")
	}
	return nil
}


func getList(key string, def []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}