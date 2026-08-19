package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage provider API keys",
}

// ---------- keys add ----------

var addKeyLabel string

var keysAddCmd = &cobra.Command{
	Use:   "add <provider> <key>",
	Short: "Add an API key for a provider (gemini, groq, cerebras, mistral, ...)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, key := args[0], args[1]

		s, err := Open(provider)
		if err != nil {
			return err
		}

		rec, err := s.AddKey(provider, key, addKeyLabel)
		if err != nil {
			return err
		}

		fmt.Printf("Added key %s for %s (added %s)\n", rec.Masked(), rec.Provider, rec.AddedAt.Format("2006-01-02 15:04"))
		fmt.Printf("Stored in %s\n", s.Path())
		return nil
	},
}

// ---------- keys list ----------

var listKeysProvider string

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List connected keys, when they were added, and how many per provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := Open(listKeysProvider)
		if err != nil {
			return err
		}

		keys := s.ListKeys(listKeysProvider)
		if len(keys) == 0 {
			fmt.Println("No keys connected yet. Add one with: hydra keys add <provider> <key>")
			return nil
		}

		countByProvider := map[string]int{}
		for _, k := range keys {
			countByProvider[k.Provider]++
		}

		fmt.Printf("%-12s %-10s %-20s %s\n", "PROVIDER", "KEY", "ADDED", "LABEL")
		for _, k := range keys {
			fmt.Printf("%-12s %-10s %-20s %s\n",
				k.Provider, k.Masked(), k.AddedAt.Format("2006-01-02 15:04"), k.Label)
		}

		fmt.Println()
		for provider, count := range countByProvider {
			fmt.Printf("%s: %d key(s)\n", provider, count)
		}
		return nil
	},
}

func init() {
	keysAddCmd.Flags().StringVar(&addKeyLabel, "label", "", "optional note, e.g. \"personal account\"")
	keysListCmd.Flags().StringVar(&listKeysProvider, "provider", "", "filter by provider")

	keysCmd.AddCommand(keysAddCmd, keysListCmd)
	rootCmd.AddCommand(keysCmd)
}
