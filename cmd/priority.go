package cmd

import (
	"Argus/capacity"
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

var priorityCmd = &cobra.Command{
	Use:   "priority [<provider> <model> <priority>]",
	Short: "Show routing priorities, or set one with provider, model, priority",
	Long: `Show the routing priority of every model (lower number = tried first).

To set a priority for a model, pass three arguments:
  Argus priority <provider> <model> <priority>`,
	Args: func(cmd *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				return nil // show priorities
			case 3:
				return nil // set priority
			default:
				return fmt.Errorf("expected 0 or 3 arguments, got %d", len(args))
			}
		},
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := OpenDefault()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return listPriorities(s)
		}

		provider, model := args[0], args[1]
		priority, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("priority must be an integer, got %q", args[2])
		}

		if err := s.SetPriority(provider, model, priority); err != nil {
			return err
		}

		fmt.Printf("Set %s/%s priority to %d\n", provider, model, priority)
		return nil
	},
}

// listPriorities prints every configured model with its routing priority.
func listPriorities(s *Store) error {
	models := make([]priorityEntry, 0, len(capacity.Catalog.ModelLimits))
	for _, m := range capacity.Catalog.ModelLimits {
		models = append(models, priorityEntry{
			Provider: m.Provider,
			Model:    m.Model,
		})
	}

	// Include custom models registered by the user
	for _, cm := range s.ListCustomModels() {
		models = append(models, priorityEntry{Provider: cm.Provider, Model: cm.Model})
	}

	if len(models) == 0 {
		fmt.Println("No models configured.")
		return nil
	}

	sort.Slice(models, func(i, j int) bool {
		if models[i].Provider != models[j].Provider {
			return models[i].Provider < models[j].Provider
		}
		return models[i].Model < models[j].Model
	})

	fmt.Printf("%-14s %-30s %s\n", "PROVIDER", "MODEL", "PRIORITY")
	for _, m := range models {
		fmt.Printf("%-14s %-30s %d\n",
			m.Provider, m.Model, s.Priority(m.Provider, m.Model))
	}

	fmt.Println()
	fmt.Println("Lower priority numbers are tried first. Set a priority with:")
	fmt.Println("  Argus priority <provider> <model> <priority>")
	return nil
}

type priorityEntry struct {
	Provider string
	Model    string
}

func init() {
	rootCmd.AddCommand(priorityCmd)
}
