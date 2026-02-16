package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sp3esu/mac-cleaner/internal/server"
)

var flagSocket string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "start the IPC server for Swift app integration",
	Long:  "starts a Unix domain socket server that accepts NDJSON requests for scan and cleanup operations",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle SIGINT/SIGTERM for graceful shutdown.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		srv := server.New(flagSocket, version)

		go func() {
			<-sigCh
			fmt.Fprintln(os.Stderr, "\nShutting down...")
			srv.Shutdown()
			cancel()
		}()

		fmt.Fprintf(os.Stderr, "Listening on %s\n", flagSocket)
		return srv.Serve(ctx)
	},
}

func init() {
	serveCmd.Flags().StringVar(&flagSocket, "socket", "/tmp/mac-cleaner.sock", "Unix domain socket path")
	rootCmd.AddCommand(serveCmd)
}
