package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"boardoflife/backend/internal/auth"
	"boardoflife/backend/internal/books"
	"boardoflife/backend/internal/booksearch"
	"boardoflife/backend/internal/healthbars"
	"boardoflife/backend/internal/httpx"
	"boardoflife/backend/internal/ratelimit"
)

const (
	sessionTTL    = 7 * 24 * time.Hour
	purgeInterval = time.Hour

	lookupsPerMinute = 30
)

func main() {
	if err := run(); err != nil {
		slog.Error("backend stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	db, err := openDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	authSvc, err := auth.NewService(auth.NewSQLStore(db), sessionTTL)
	if err != nil {
		return err
	}
	bootstrapAdmin(ctx, authSvc, cfg)
	go purgeExpiredSessions(ctx, authSvc)

	httpClient := booksearch.NewHTTPClient()
	searcher := booksearch.NewService(
		booksearch.NewGoogle(httpClient, booksearch.GoogleBooksURL, cfg.GoogleBooksAPIKey),
		booksearch.NewOpenLibrary(httpClient, booksearch.OpenLibraryURL),
	)
	limiter := ratelimit.New[int64](lookupsPerMinute, time.Minute)
	booksHandler := books.NewHandler(books.NewSQLStore(db), authSvc.RequireAuth, searcher, limiter)

	healthbarsHandler := healthbars.NewHandler(healthbars.NewSQLStore(db), authSvc.RequireAuth)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           newRouter(auth.NewHandler(authSvc, cfg.CookieSecure), booksHandler, healthbarsHandler),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("backend listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func newRouter(authHandler *auth.Handler, booksHandler *books.Handler, healthbarsHandler *healthbars.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer, accessLog)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, "not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", health)
		authHandler.Routes(r)
		booksHandler.Routes(r)
		healthbarsHandler.Routes(r)
	})
	return r
}

func health(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// accessLog logs method, path, status and duration. Headers, cookies and
// bodies are deliberately never logged.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method, "path", r.URL.Path,
			"status", ww.Status(), "duration", time.Since(start))
	})
}

func bootstrapAdmin(ctx context.Context, svc *auth.Service, cfg config) {
	if cfg.AdminUsername == "" || cfg.AdminPassword == "" {
		slog.Info("admin bootstrap skipped: ADMIN_USERNAME / ADMIN_PASSWORD not set")
		return
	}
	created, err := svc.BootstrapAdmin(ctx, cfg.AdminUsername, cfg.AdminPassword)
	switch {
	case err != nil:
		slog.Error("admin bootstrap failed", "err", err)
	case created:
		slog.Info("admin bootstrap: created initial admin", "username", cfg.AdminUsername)
	default:
		slog.Info("admin bootstrap skipped: users already exist")
	}
}

func purgeExpiredSessions(ctx context.Context, svc *auth.Service) {
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()
	for {
		if n, err := svc.PurgeExpired(ctx); err != nil {
			slog.Error("purge expired sessions", "err", err)
		} else if n > 0 {
			slog.Info("purged expired sessions", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
