package cmd

import (
	"Argus/capacity"
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var priorityCmd = &cobra.Command{
	Use:   "priority [<provider> <model|-all> <priority>]",
	Short: "Show routing priorities, or set them with provider, model, priority",
	Long: `Show the routing priority of every model (lower number = tried first).

To set a priority for one model:
  Argus priority <provider> <model> <priority>

To set the priority of every model from a provider:
  Argus priority <provider> -all <priority>`,
	// No flags; parse args manually so "-all" isn't treated as a flag value.
	DisableFlagParsing: true,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
			return pflag.ErrHelp
		}
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

		if model == "-all" {
			return setAllPriorities(s, provider, priority)
		}

		if err := s.SetPriority(provider, model, priority); err != nil {
			return err
		}

		fmt.Printf("Set %s/%s priority to %d\n", provider, model, priority)
		return nil
	},
}

// setAllPriorities sets the priority of every model from a provider.
func setAllPriorities(s *Store, provider string, priority int) error {
	entries := allModelEntries(s)

	count := 0
	for _, e := range entries {
		if e.Provider != provider {
			continue
		}
		if err := s.SetPriority(e.Provider, e.Model, priority); err != nil {
			return err
		}
		count++
	}

	if count == 0 {
		return fmt.Errorf("no models found for provider %q", provider)
	}

	fmt.Printf("Set %d model(s) from %s to priority %d\n", count, provider, priority)
	return nil
}

// allModelEntries returns every known model: catalog plus custom ones.
func allModelEntries(s *Store) []priorityEntry {
	entries := make([]priorityEntry, 0, len(capacity.Catalog.ModelLimits))
	for _, m := range capacity.Catalog.ModelLimits {
		entries = append(entries, priorityEntry{Provider: m.Provider, Model: m.Model})
	}
	for _, cm := range s.ListCustomModels() {
		entries = append(entries, priorityEntry{Provider: cm.Provider, Model: cm.Model})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Provider != entries[j].Provider {
			return entries[i].Provider < entries[j].Provider
		}
		return entries[i].Model < entries[j].Model
	})
	return entries
}

// listPriorities prints every configured model with its routing priority.
func listPriorities(s *Store) error {
	models := allModelEntries(s)

	if len(models) == 0 {
		fmt.Println("No models configured.")
		return nil
	}

	fmt.Printf("%-14s %-30s %s\n", "PROVIDER", "MODEL", "PRIORITY")
	for _, m := range models {
		fmt.Printf("%-14s %-30s %d\n",
			m.Provider, m.Model, s.Priority(m.Provider, m.Model))
	}

	fmt.Println()
	fmt.Println("Lower priority numbers are tried first. Set a priority with:")
	fmt.Println("  Argus priority <provider> <model> <priority>")
	fmt.Println("  Argus priority <provider> -all <priority>")
	return nil
}

type priorityEntry struct {
	Provider string
	Model    string
}

func init() {
	rootCmd.AddCommand(priorityCmd)
}