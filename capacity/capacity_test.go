package capacity

import (
	"sort"
	"testing"
)

func TestCalculateProviderCapacity_NoKeys(t *testing.T) {
	results := CalculateProviderCapacity(UserKeys{}, Catalog.ModelLimits)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty keys, got %d", len(results))
	}
}

func TestCalculateProviderCapacity_SingleGeminiKey(t *testing.T) {
	keys := UserKeys{"gemini": {"fake-key-1"}}
	results := CalculateProviderCapacity(keys, Catalog.ModelLimits)

	if len(results) != 1 {
		t.Fatalf("expected 1 provider result, got %d", len(results))
	}
	r := results[0]
	if r.Provider != "gemini" {
		t.Fatalf("expected provider gemini, got %s", r.Provider)
	}
	if r.KeyCount != 1 {
		t.Fatalf("expected 1 key, got %d", r.KeyCount)
	}
	if r.RequestsPerMonth <= 0 {
		t.Fatalf("expected positive requests/month, got %d", r.RequestsPerMonth)
	}
	if r.TokensPerMonth <= 0 {
		t.Fatalf("expected positive tokens/month, got %d", r.TokensPerMonth)
	}
}

func TestCalculateProviderCapacity_MultipleKeys(t *testing.T) {
	keys := UserKeys{
		"gemini": {"key-1", "key-2"},
		"groq":   {"key-3"},
	}
	results := CalculateProviderCapacity(keys, Catalog.ModelLimits)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Provider < results[j].Provider
	})

	if len(results) != 2 {
		t.Fatalf("expected 2 provider results, got %d", len(results))
	}

	gemini := results[0]
	groq := results[1]

	if gemini.Provider != "gemini" || groq.Provider != "groq" {
		t.Fatalf("unexpected providers: %s, %s", gemini.Provider, groq.Provider)
	}

	if gemini.KeyCount != 2 {
		t.Fatalf("expected 2 gemini keys, got %d", gemini.KeyCount)
	}
	if groq.KeyCount != 1 {
		t.Fatalf("expected 1 groq key, got %d", groq.KeyCount)
	}
}

func TestCalculateProviderCapacity_ScalesWithKeys(t *testing.T) {
	oneKey := UserKeys{"gemini": {"key-1"}}
	twoKeys := UserKeys{"gemini": {"key-1", "key-2"}}

	r1 := CalculateProviderCapacity(oneKey, Catalog.ModelLimits)
	r2 := CalculateProviderCapacity(twoKeys, Catalog.ModelLimits)

	if len(r1) != 1 || len(r2) != 1 {
		t.Fatal("expected 1 result each")
	}

	if r2[0].RequestsPerMonth != r1[0].RequestsPerMonth*2 {
		t.Fatalf("requests didn't scale: 1 key = %d, 2 keys = %d (expected %d)",
			r1[0].RequestsPerMonth, r2[0].RequestsPerMonth, r1[0].RequestsPerMonth*2)
	}
	if r2[0].TokensPerMonth != r1[0].TokensPerMonth*2 {
		t.Fatalf("tokens didn't scale: 1 key = %d, 2 keys = %d (expected %d)",
			r1[0].TokensPerMonth, r2[0].TokensPerMonth, r1[0].TokensPerMonth*2)
	}
}

func TestCalculateProviderCapacity_UnknownProviderIgnored(t *testing.T) {
	keys := UserKeys{"nonexistent": {"key-1"}}
	results := CalculateProviderCapacity(keys, Catalog.ModelLimits)

	// The provider has keys but no catalog entries, so capacity will be 0/0.
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.RequestsPerMonth != 0 || r.TokensPerMonth != 0 {
		t.Fatalf("expected 0/0 for unknown provider, got %d/%d",
			r.RequestsPerMonth, r.TokensPerMonth)
	}
}
