package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var serverAddr string

var rootCmd = &cobra.Command{
	Use:   "Argus",
	Short: "Argus — a multi-provider LLM gateway",
	Long:  "Argus pools free-tier API keys across multiple LLM providers behind one OpenAI-compatible endpoint.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "http://localhost:8080", "Argus admin API address")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
