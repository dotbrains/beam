package cmd

import (
	"fmt"
	"net/http"

	"github.com/dotbrains/beam/internal/beam"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Beam webhook API",
		RunE: func(cmd *cobra.Command, args []string) error {
			server := &http.Server{
				Addr:    addr,
				Handler: beam.Handler(beam.NewStore()),
			}
			cmd.ErrOrStderr().Write([]byte(fmt.Sprintf("beam listening on %s\n", addr)))
			return server.ListenAndServe()
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "listen address")
	return cmd
}
