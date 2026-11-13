package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/intarsia/intarsia/internal/server"
)

// dist is the compiled web interface, produced by `make build` from web/.
//
//go:embed all:dist
var dist embed.FS

const (
	// loopbackHost is not configurable: the application must not be reachable
	// from other machines while it has no login.
	loopbackHost = "127.0.0.1"
	defaultPort  = 8080

	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second
)

func newServeCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web application on 127.0.0.1",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			assets, err := fs.Sub(dist, "dist")
			if err != nil {
				return fmt.Errorf("load embedded web interface: %w", err)
			}

			addr := net.JoinHostPort(loopbackHost, strconv.Itoa(port))
			var lc net.ListenConfig
			listener, err := lc.Listen(cmd.Context(), "tcp", addr)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", addr, err)
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Intársia is running at http://%s\n", listener.Addr()); err != nil {
				return err
			}
			return serve(cmd.Context(), listener, server.New(assets))
		},
	}
	cmd.Flags().IntVar(&port, "port", defaultPort, "port to listen on")
	return cmd
}

// serve answers requests on listener until ctx is cancelled, then shuts down
// gracefully. It closes listener before returning.
func serve(ctx context.Context, listener net.Listener, handler http.Handler) error {
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: readHeaderTimeout}

	failed := make(chan error, 1)
	go func() { failed <- srv.Serve(listener) }()

	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down: %w", err)
		}
		if err := <-failed; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
