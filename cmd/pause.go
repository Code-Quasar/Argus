package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pauseModel string

var pauseCmd = &cobra.Command{
	Use:   "pause <provider>",
	Short: "Pause a provider, or one model within it with --model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setPaused(args[0], pauseModel, true)
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume <provider>",
	Short: "Resume a paused provider, or one model within it with --model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setPaused(args[0], pauseModel, false)
	},
}

func setPaused(provider, model string, paused bool) error {
	s, err := OpenDefault()
	if err != nil {
		return err
	}

	if err := s.SetPaused(provider, model, paused); err != nil {
		return err
	}

	action := "Resumed"
	if paused {
		action = "Paused"
	}
	target := provider
	if model != "" {
		target = fmt.Sprintf("%s/%s", provider, model)
	}
	fmt.Printf("%s %s\n", action, target)
	return nil
}

func init() {
	pauseCmd.Flags().StringVar(&pauseModel, "model", "", "pause a specific model instead of the whole provider")
	resumeCmd.Flags().StringVar(&pauseModel, "model", "", "resume a specific model instead of the whole provider")

	rootCmd.AddCommand(pauseCmd, resumeCmd)
}
