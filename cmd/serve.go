package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var servePort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Hydra gateway server (the OpenAI-compatible endpoint + admin API)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Starting Hydra on :%d...\n", servePort)

		// TODO: this is the one piece not built yet in this conversation —
		// the actual HTTP server wiring chi router + convert.Registry +
		// providers.Catalog + the key-pool/router logic into a running
		// public endpoint (/v1/chat/completions). It should call
		// openStore() to read the same ~/.hydra/store.json this CLI
		// writes to — no separate admin API or database needed, since
		// they're already sharing one file on disk.
		//
		// return server.Run(servePort, dataPath)
		return fmt.Errorf("server not wired up yet — see TODO in cmd/hydra/serve.go")
	},
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "port to listen on")
	rootCmd.AddCommand(serveCmd)
}
