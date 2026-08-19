package cmd

import (
	"Argus/capacity"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

var capacityCmd = &cobra.Command{
	Use:   "capacity",
	Short: "Show total monthly tokens and requests given your connected API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := OpenDefault()
		if err != nil {
			return err
		}

		keys := s.ListKeys("")
		if len(keys) == 0 {
			fmt.Println("No keys connected yet. Add one with: Argus keys add <provider>")
			return nil
		}

		// Build the UserKeys map expected by the capacity package.
		userKeys := make(capacity.UserKeys)
		for _, k := range keys {
			userKeys[k.Provider] = append(userKeys[k.Provider], k.Key)
		}

		results := capacity.CalculateProviderCapacity(userKeys, capacity.Catalog.ModelLimits)
		if len(results) == 0 {
			fmt.Println("No capacity data for your connected providers.")
			return nil
		}

		// Sort by provider name for stable output.
		sort.Slice(results, func(i, j int) bool {
			return results[i].Provider < results[j].Provider
		})

		fmt.Printf("%-14s %-6s %-20s %s\n", "PROVIDER", "KEYS", "REQUESTS/MONTH", "TOKENS/MONTH")

		totalRequests := 0
		totalTokens := 0
		for _, r := range results {
			fmt.Printf("%-14s %-6d %-20s %s\n",
				r.Provider, r.KeyCount,
				formatNumber(r.RequestsPerMonth),
				formatNumber(r.TokensPerMonth))
			totalRequests += r.RequestsPerMonth
			totalTokens += r.TokensPerMonth
		}

		fmt.Printf("\n%-14s %-6s %-20s %s\n",
			"TOTAL", "", formatNumber(totalRequests), formatNumber(totalTokens))

		// Show suggestion based on capacity level
		fmt.Println()
		suggestCapacity(totalRequests, totalTokens, len(keys))

		return nil
	},
}

// suggestCapacity prints a colored message suggesting to add more keys or providers
func suggestCapacity(totalRequests, totalTokens, keyCount int) {
	const (
		lowRequests   = 100_000
		lowTokens     = 500_000
		mediumRequests = 500_000
		mediumTokens  = 2_000_000
	)

	if totalRequests < lowRequests || totalTokens < lowTokens {
		// Low capacity - red message
		fmt.Printf("%s⚠ Low capacity detected! Consider adding more API keys or connecting additional providers to increase your monthly tokens and requests.%s\n",
			colorRed, colorReset)
		fmt.Printf("%sUse: Argus keys add <provider>%s\n", colorYellow, colorReset)
	} else if totalRequests < mediumRequests || totalTokens < mediumTokens {
		// Medium capacity - yellow message
		fmt.Printf("%s💡 Your capacity is moderate. Add more keys or providers to increase your monthly limits.%s\n",
			colorYellow, colorReset)
	} else {
		// Good capacity - green message
		fmt.Printf("%s✓ Your monthly capacity looks good!%s\n", colorGreen, colorReset)
	}
}

// formatNumber inserts commas into an integer for readability.
func formatNumber(n int) string {
	if n == 0 {
		return "0"
	}

	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	// Work from the right, inserting commas every 3 digits.
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

func init() {
	rootCmd.AddCommand(capacityCmd)
}
