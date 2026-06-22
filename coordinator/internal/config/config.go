// Package config loads Coordinator settings from the environment. All keys are
// prefixed CTV_ and have development-friendly defaults.
package config

import "os"

type Config struct {
	ListenAddr  string // CTV_LISTEN_ADDR
	DatabaseURL string // CTV_DATABASE_URL
	RedisURL    string // CTV_REDIS_URL
	Env         string // CTV_ENV: development | production
}

// Load reads configuration from the environment, applying defaults.
func Load() Config {
	return Config{
		ListenAddr:  getenv("CTV_LISTEN_ADDR", ":8080"),
		DatabaseURL: getenv("CTV_DATABASE_URL", "postgres://calltoverify:calltoverify@localhost:5432/calltoverify?sslmode=disable"),
		RedisURL:    getenv("CTV_REDIS_URL", "redis://localhost:6379"),
		Env:         getenv("CTV_ENV", "development"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
