package main

import (
	"fmt"
	"os"
	"strconv"
)

// config is the process configuration, read from the environment.
type config struct {
	DatabaseURL   string
	CookieSecure  bool
	AdminUsername string
	AdminPassword string
}

func loadConfig() (config, error) {
	cfg := config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
	}
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		secure, err := strconv.ParseBool(v)
		if err != nil {
			return config{}, fmt.Errorf("COOKIE_SECURE: %w", err)
		}
		cfg.CookieSecure = secure
	}
	return cfg, nil
}
