package cmd

import (
	"Argus/capacity"
	"fmt"

	"github.com/spf13/cobra"
)

var setupProviders = []string{
	capacity.Gemini,
	capacity.Groq,
	capacity.Mistral,
	capacity.Cerebras,
	capacity.OpenRouter,
	capacity.Zhipu,
}

var fullSetupCmd = &cobra.Command{
	Use:   "full-setup",
	Short: "Configure API keys for all supported providers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := OpenDefault()
		if err != nil {
			return err
		}

		fmt.Println("Configure provider API keys.")
		fmt.Println("Leave a key empty to skip that provider.")
		fmt.Println()

		added := 0
		for _, provider := range setupProviders {
			key, err := readHidden("Enter API key for " + provider + " (leave empty to skip): ")
			if err != nil {
				return err
			}
			if key == "" {
				continue
			}
			if _, err := store.AddKey(provider, key, ""); err != nil {
				return fmt.Errorf("save %s API key: %w", provider, err)
			}
			added++
			fmt.Printf("Added %s key.\n", provider)
		}

		fmt.Printf("Setup complete: %d provider key(s) added.\n", added)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fullSetupCmd)
}
