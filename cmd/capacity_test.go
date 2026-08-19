package cmd

import (
	"Argus/capacity"
	"testing"
)

func TestCapacityCommand_Integration(t *testing.T) {
	// Set up an in-memory store with keys for two providers.
	path := t.TempDir() + "/store.json"
	store := &Store{path: path, password: "test-pw", d: emptyData()}

	if _, err := store.AddKey("gemini", "gemini-key-1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddKey("gemini", "gemini-key-2", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddKey("groq", "groq-key-1", ""); err != nil {
		t.Fatal(err)
	}

	// Build UserKeys the same way the CLI command does.
	keys := store.ListKeys("")
	userKeys := make(capacity.UserKeys)
	for _, k := range keys {
		userKeys[k.Provider] = append(userKeys[k.Provider], k.Key)
	}

	results := capacity.CalculateProviderCapacity(userKeys, capacity.Catalog.ModelLimits)

	if len(results) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(results))
	}

	totalRequests := 0
	totalTokens := 0
	for _, r := range results {
		totalRequests += r.RequestsPerMonth
		totalTokens += r.TokensPerMonth

		switch r.Provider {
		case "gemini":
			if r.KeyCount != 2 {
				t.Errorf("gemini: expected 2 keys, got %d", r.KeyCount)
			}
		case "groq":
			if r.KeyCount != 1 {
				t.Errorf("groq: expected 1 key, got %d", r.KeyCount)
			}
		default:
			t.Errorf("unexpected provider: %s", r.Provider)
		}
	}

	if totalRequests <= 0 {
		t.Error("total requests should be positive")
	}
	if totalTokens <= 0 {
		t.Error("total tokens should be positive")
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{12, "12"},
		{123, "123"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{1000000, "1,000,000"},
	}

	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
