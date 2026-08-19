package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

var (
	statsPeriod   string
	statsProvider string
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show request counts per day or per month",
	RunE: func(cmd *cobra.Command, args []string) error {
		var kind StatKind
		switch statsPeriod {
		case "day":
			kind = StatsDay
		case "month":
			kind = StatsMonth
		default:
			return fmt.Errorf("--period must be \"day\" or \"month\", got %q", statsPeriod)
		}

		s, err := Open(statsProvider)
		if err != nil {
			return err
		}

		counts := s.StatsFor(kind, statsProvider)
		if len(counts) == 0 {
			fmt.Println("No requests recorded for this period yet.")
			return nil
		}

		// Sort keys for stable, readable output.
		scopes := make([]string, 0, len(counts))
		for scope := range counts {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)

		fmt.Printf("Requests this %s:\n\n", statsPeriod)
		fmt.Printf("%-40s %s\n", "PROVIDER/MODEL", "REQUESTS")
		total := 0
		for _, scope := range scopes {
			fmt.Printf("%-40s %d\n", scope, counts[scope])
			total += counts[scope]
		}
		fmt.Printf("\nTotal: %d\n", total)
		return nil
	},
}

func init() {
	statsCmd.Flags().StringVar(&statsPeriod, "period", "day", "\"day\" or \"month\"")
	statsCmd.Flags().StringVar(&statsProvider, "provider", "", "filter by provider")
	rootCmd.AddCommand(statsCmd)
}
