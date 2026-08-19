package capacity

const (
	Gemini     = "gemini"
	Groq       = "groq"
	Mistral    = "mistral"
	Cerebras   = "cerebras"
	OpenRouter = "openrouter"
	Zhipu      = "zhipu" // GLM, served via z.ai
)

const (
	EndpointGemini     = "https://generativelanguage.googleapis.com/v1beta"
	EndpointGroq       = "https://api.groq.com/openai/v1"
	EndpointMistral    = "https://api.mistral.ai/v1"
	EndpointCerebras   = "https://api.cerebras.ai/v1"
	EndpointOpenRouter = "https://openrouter.ai/api/v1"
	EndpointZhipu      = "https://open.bigmodel.cn/api/paas/v4" // z.ai / bigmodel.cn, mainland vs intl differ — verify per account
)

type capacity struct {
	ModelLimits []ModelLimits
	UserKeys    UserKeys
}
type UserKeys map[string][]string

type ModelLimits struct {
	Provider string
	Model    string
	Tier     string
	RPM      int
	TPM      int
	RPD      int
}

type ProviderCapacity struct {
	Provider         string
	KeyCount         int
	Models           []string
	RequestsPerMonth int
	TokensPerMonth   int
}

var Catalog = capacity{ModelLimits: []ModelLimits{
	// Gemini — limits apply per Google Cloud project, not per key.
	// Pulled directly from a live AI Studio project console (RPM/TPM/RPD),
	// verified 2026-08-19 — this supersedes the third-party-sourced
	// figures used earlier, which were already stale/conflicting.
	// Gemini 2.5 Pro, Gemini 2 Flash, and Gemini 2 Flash Lite are omitted:
	// this project currently shows 0/0/0 free-tier access to all three.
	{Provider: Gemini, Model: "gemini-2.5-flash", Tier: "free", RPM: 5, TPM: 250_000, RPD: 20},
	{Provider: Gemini, Model: "gemini-2.5-flash-lite", Tier: "free", RPM: 10, TPM: 250_000, RPD: 20},
	{Provider: Gemini, Model: "gemini-3-flash", Tier: "free", RPM: 5, TPM: 250_000, RPD: 20},
	{Provider: Gemini, Model: "gemini-3.1-flash-lite", Tier: "free", RPM: 15, TPM: 250_000, RPD: 500},
	{Provider: Gemini, Model: "gemini-3.5-flash", Tier: "free", RPM: 5, TPM: 250_000, RPD: 20},
	{Provider: Gemini, Model: "gemini-3.5-flash-lite", Tier: "free", RPM: 15, TPM: 250_000, RPD: 500},
	{Provider: Gemini, Model: "gemini-3.6-flash", Tier: "free", RPM: 5, TPM: 250_000, RPD: 20},
	{Provider: Gemini, Model: "gemini-3.7-flash", Tier: "free", RPM: 5, TPM: 250_000, RPD: 20},

	// Groq — limits apply per organization, not per key. 8B kept as the
	// high-throughput fallback; gpt-oss-120b added as the stronger option
	// when quality matters more than raw request budget.
	{Provider: Groq, Model: "llama-3.1-8b-instant", Tier: "free", RPM: 30, TPM: 6_000, RPD: 14_400},
	{Provider: Groq, Model: "llama-3.3-70b-versatile", Tier: "free", RPM: 30, TPM: 6_000, RPD: 1_000},
	{Provider: Groq, Model: "openai/gpt-oss-120b", Tier: "free", RPM: 30, TPM: 8_000, RPD: 1_000},

	// Mistral — the free "Experiment" tier actually includes Mistral
	// Large, not just Small; upgraded here since Large is the better
	// reasoning model and costs the same $0 on this tier.
	{Provider: Mistral, Model: "mistral-large-latest", Tier: "free", RPM: 2, TPM: 500_000, RPD: 0},

	// Cerebras — catalog has been pruned hard in 2026 (down to ~2 models
	// from a dozen as of one May 2026 snapshot); gpt-oss-120b is the
	// confirmed-live, higher-capability survivor as of this writing.
	{Provider: Cerebras, Model: "gpt-oss-120b", Tier: "free", RPM: 30, TPM: 60_000, RPD: 14_400},

	// OpenRouter — the free catalog churns weekly (models get delisted
	// with no notice), so a wildcard is the only safe default. gpt-oss-20b
	// is included as a concrete, currently-live, stronger-than-tiny option
	// — but do not hardcode reliance on it; re-check openrouter.ai/models
	// filtered by :free before depending on any specific ID.
	{Provider: OpenRouter, Model: "*:free", Tier: "free", RPM: 20, TPM: 0, RPD: 50},
	{Provider: OpenRouter, Model: "openai/gpt-oss-20b:free", Tier: "free", RPM: 20, TPM: 0, RPD: 50},

	// Zhipu / GLM — free tier revised repeatedly; re-verify before relying
	// on this in production planning.
	{Provider: Zhipu, Model: "glm-4.7-flash", Tier: "free", RPM: 5, TPM: 0, RPD: 1_000},
}}

func CalculateProviderCapacity(keys UserKeys, catalog []ModelLimits) []ProviderCapacity {
	// First pass: total per-key capacity and model list, per provider.
	requestsPerKey := make(map[string]int)
	tokensPerKey := make(map[string]int)
	modelsByProvider := make(map[string][]string)

	for _, m := range catalog {
		requestsPerKey[m.Provider] += m.RPD * 30
		tokensPerKey[m.Provider] += m.TPM
		modelsByProvider[m.Provider] = append(modelsByProvider[m.Provider], m.Model)
	}

	// Second pass: scale by how many keys the user actually connected.
	var results []ProviderCapacity
	for provider, userKeysForProvider := range keys {
		keyCount := len(userKeysForProvider)
		if keyCount == 0 {
			continue
		}
		results = append(results, ProviderCapacity{
			Provider:         provider,
			KeyCount:         keyCount,
			Models:           modelsByProvider[provider],
			RequestsPerMonth: requestsPerKey[provider] * keyCount,
			TokensPerMonth:   tokensPerKey[provider] * keyCount,
		})
	}
	return results
}
