package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var priorityCmd = &cobra.Command{
	Use:   "priority <provider> <model> <priority>",
	Short: "Set routing priority for a model (lower number = tried first)",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, model := args[0], args[1]
		priority, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("priority must be an integer, got %q", args[2])
		}

		s, err := OpenDefault()
		if err != nil {
			return err
		}

		if err := s.SetPriority(provider, model, priority); err != nil {
			return err
		}

		fmt.Printf("Set %s/%s priority to %d\n", provider, model, priority)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(priorityCmd)
}
