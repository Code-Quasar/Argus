package cmd

import (
	"Argus/capacity"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage provider API keys",
}

// ---------- keys add ----------

var addKeyLabel string
var addKeyCustom bool

var keysAddCmd = &cobra.Command{
	Use:   "add <provider>",
	Short: "Add an API key for a provider",
	Long: fmt.Sprintf("Add an API key for a provider.\n\nAvailable providers:\n  %s",
		strings.Join([]string{
			capacity.Gemini + "     - Google Gemini",
			capacity.Groq + "      - Groq",
			capacity.Mistral + "   - Mistral AI",
			capacity.Cerebras + "  - Cerebras",
			capacity.OpenRouter + " - OpenRouter",
			capacity.Zhipu + "     - Zhipu/GLM",
		}, "\n  ")),
	Args: func(cmd *cobra.Command, args []string) error {
			if addKeyCustom {
				// --custom is interactive, no args needed
				return cobra.NoArgs(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
	RunE: func(cmd *cobra.Command, args []string) error {
		if addKeyCustom {
			return runCustomKeyWizard()
		}

		provider := args[0]
		key, err := readPassword("Enter API key: ")
		if err != nil {
			return err
		}

		s, err := OpenDefault()
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

// ---------- keys add --custom interactive wizard ----------

// CustomModel represents a user-defined custom model
type CustomModel struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	Endpoint       string `json:"endpoint"`
	Style          string `json:"style"` // "openai" or "gemini"
	Key            string `json:"key"`
	TokensPerMonth int    `json:"tokens_per_month"`
	RPDMonth       int    `json:"rpd_month"` // requests per day * 30
}

func runCustomKeyWizard() error {
	reader := bufio.NewReader(os.Stdin)

	// Ask for provider name
	fmt.Print("Provider name: ")
	provider, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading provider: %w", err)
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	// Ask for endpoint
	fmt.Print("Endpoint URL (e.g., https://api.example.com/v1): ")
	endpoint, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading endpoint: %w", err)
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}
	endpoint = strings.TrimRight(endpoint, "/")

	// Ask for model name
	fmt.Print("Model name: ")
	modelName, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading model name: %w", err)
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return fmt.Errorf("model name cannot be empty")
	}

	// Ask for API key (hidden input)
	apiKey, err := readPassword("API key: ")
	if err != nil {
		return err
	}

	// Ask for API style
	fmt.Print("API style (openai/gemini) [openai]: ")
	style, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading style: %w", err)
	}
	style = strings.TrimSpace(style)
	if style == "" {
		style = "openai"
	}
	if style != "openai" && style != "gemini" {
		return fmt.Errorf("API style must be 'openai' or 'gemini', got '%s'", style)
	}

	// Ask for monthly tokens (optional)
	fmt.Print("Monthly token limit (press Enter for 0): ")
	tokensInput, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading tokens: %w", err)
	}
	tokensInput = strings.TrimSpace(tokensInput)
	tokensPerMonth := 0
	if tokensInput != "" {
		tokensPerMonth, err = strconv.Atoi(tokensInput)
		if err != nil {
			return fmt.Errorf("invalid token limit: %w", err)
		}
	}

	// Ask for requests per month (optional)
	fmt.Print("Monthly request limit (press Enter for 0): ")
	requestsInput, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading requests: %w", err)
	}
	requestsInput = strings.TrimSpace(requestsInput)
	requestsPerMonth := 0
	if requestsInput != "" {
		requestsPerMonth, err = strconv.Atoi(requestsInput)
		if err != nil {
			return fmt.Errorf("invalid request limit: %w", err)
		}
	}

	// Verify the endpoint works
	fmt.Printf("\nVerifying endpoint %s with model %s...\n", endpoint, modelName)
	if err := verifyEndpoint(endpoint, modelName, apiKey, style); err != nil {
		fmt.Printf("⚠ Endpoint verification failed: %v\n", err)
		fmt.Print("Continue anyway? (y/N): ")
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm != "y" && confirm != "yes" {
			return fmt.Errorf("cancelled by user")
		}
	} else {
		fmt.Printf("✓ Endpoint verification successful!\n\n")
	}

	// Save to store
	s, err := OpenDefault()
	if err != nil {
		return err
	}

	// Add the key
	rec, err := s.AddKey(provider, apiKey, fmt.Sprintf("custom: %s @ %s", modelName, endpoint))
	if err != nil {
		return err
	}

	// Store custom model metadata
	customModel := CustomModel{
		Provider:       provider,
		Model:          modelName,
		Endpoint:       endpoint,
		Style:          style,
		TokensPerMonth: tokensPerMonth,
		RPDMonth:       requestsPerMonth / 30,
	}

	if err := s.AddCustomModel(customModel); err != nil {
		return err
	}

	// New custom models get the lowest default priority so they are only
	// tried after every built-in model, until the user changes it.
	if err := s.SetPriority(provider, modelName, 99); err != nil {
		return err
	}

	fmt.Printf("\nAdded custom model:\n")
	fmt.Printf("  Provider: %s\n", provider)
	fmt.Printf("  Model:    %s\n", modelName)
	fmt.Printf("  Endpoint: %s\n", endpoint)
	fmt.Printf("  Style:    %s\n", style)
	fmt.Printf("  Key:      %s\n", rec.Masked())
	fmt.Printf("  Tokens/Month:  %s\n", formatNumber(tokensPerMonth))
	fmt.Printf("  Requests/Month: %s\n", formatNumber(requestsPerMonth))
	fmt.Printf("  Priority: %d (lowest — change with: Argus priority %s %s <number>)\n", 99, provider, modelName)
	fmt.Printf("\nStored in %s\n", s.Path())

	return nil
}

// verifyEndpoint sends a test request to verify the endpoint works
func verifyEndpoint(endpoint, modelName, apiKey, style string) error {
	client := &http.Client{Timeout: 30 * time.Second}

	var req *http.Request
	var err error

	switch style {
	case "openai":
		req, err = createOpenAITestRequest(endpoint, modelName, apiKey)
	case "gemini":
		req, err = createGeminiTestRequest(endpoint, modelName, apiKey)
	default:
		return fmt.Errorf("unsupported style: %s", style)
	}

	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	// Try to parse as JSON to verify it's a valid response
	if style == "openai" {
		var openaiResp struct {
			ID      string `json:"id"`
			Model   string `json:"model"`
			Choices []struct {
				Index        int    `json:"index"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &openaiResp); err != nil {
			return fmt.Errorf("invalid OpenAI response format: %w", err)
		}
		if openaiResp.ID == "" || len(openaiResp.Choices) == 0 {
			return fmt.Errorf("incomplete OpenAI response")
		}
	} else {
		var geminiResp struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(body, &geminiResp); err != nil {
			return fmt.Errorf("invalid Gemini response format: %w", err)
		}
		if len(geminiResp.Candidates) == 0 {
			return fmt.Errorf("incomplete Gemini response")
		}
	}

	return nil
}

func createOpenAITestRequest(endpoint, modelName, apiKey string) (*http.Request, error) {
	url := endpoint + "/chat/completions"
	payload := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": "test"},
		},
		"max_tokens": 10,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	return req, nil
}

func createGeminiTestRequest(endpoint, modelName, apiKey string) (*http.Request, error) {
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", endpoint, modelName, apiKey)
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": "test"},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// ---------- keys list ----------

var listKeysProvider string

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List connected keys, when they were added, and how many per provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := OpenDefault()
		if err != nil {
			return err
		}

		keys := s.ListKeys(listKeysProvider)
		if len(keys) == 0 {
			fmt.Println("No keys connected yet. Add one with: Argus keys add <provider>")
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
	keysAddCmd.Flags().BoolVar(&addKeyCustom, "custom", false, "add a custom provider with interactive wizard (endpoint, model, key, limits)")
	keysListCmd.Flags().StringVar(&listKeysProvider, "provider", "", "filter by provider")

	keysCmd.AddCommand(keysAddCmd, keysListCmd)
	rootCmd.AddCommand(keysCmd)
}