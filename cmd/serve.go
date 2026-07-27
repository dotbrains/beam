package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dotbrains/beam/internal/beam"
	"github.com/dotbrains/beam/internal/config"
	"github.com/dotbrains/beam/internal/storage"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var addr, storageMode, dbPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Beam webhook API",
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, closeBackend, err := openBackend(cmd.Context(), storageMode, dbPath)
			if err != nil {
				return err
			}
			defer closeBackend()
			logger := slog.New(slog.NewJSONHandler(cmd.ErrOrStderr(), nil))
			server := &http.Server{
				Addr:    addr,
				Handler: accessLog(beam.Handler(backend), logger),
			}
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "beam listening on %s\n", addr); err != nil {
				return err
			}
			return server.ListenAndServe()
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "listen address")
	cmd.Flags().StringVar(&storageMode, "storage", "sqlite", "storage backend: sqlite or memory")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func accessLog(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Info("http_request",
			"method", r.Method,
			"path", redactRequestPath(r.URL.EscapedPath()),
			"status", recorder.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

func redactRequestPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[1] == "hooks" {
		parts[2] = ":token"
	}
	if len(parts) >= 5 && parts[1] == "api" && parts[2] == "auth" && parts[3] == "device" {
		parts[4] = ":deviceCode"
	}
	return strings.Join(parts, "/")
}

func openBackend(ctx context.Context, mode, dbPath string) (beam.Backend, func(), error) {
	switch mode {
	case "memory":
		return beam.NewStore(), func() {}, nil
	case "sqlite":
		if dbPath == "" {
			cfg, err := config.Load()
			if err != nil {
				return nil, nil, err
			}
			dbPath = cfg.DBPath
		}
		if dbPath == "" {
			var err error
			dbPath, err = config.DefaultDBPath()
			if err != nil {
				return nil, nil, err
			}
		}
		store, err := storage.OpenSQLite(ctx, dbPath)
		if err != nil {
			return nil, nil, err
		}
		return store, func() { _ = store.Close() }, nil
	default:
		return nil, nil, fmt.Errorf("unknown storage backend %q", mode)
	}
}
