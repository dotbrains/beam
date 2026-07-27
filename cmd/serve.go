package cmd

import (
	"context"
	"fmt"
	"net/http"

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
			server := &http.Server{
				Addr:    addr,
				Handler: beam.Handler(backend),
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
