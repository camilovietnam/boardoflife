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

	// GoogleBooksAPIKey is optional; without it Google Books is queried
	// anonymously (and is subject to a much tighter quota).
	GoogleBooksAPIKey string
}

func loadConfig() (config, error) {
	cfg := config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),

		GoogleBooksAPIKey: os.Getenv("GOOGLE_BOOKS_API_KEY"),
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
