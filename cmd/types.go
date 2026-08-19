package cmd

import "time"

// ---------- Keys ----------

type AddKeyRequest struct {
	Provider string `json:"provider"` // "gemini", "groq", ...
	Key      string `json:"key"`
	Label    string `json:"label,omitempty"` // optional human-readable note, e.g. "personal account"
}

type KeyInfo struct {
	ID       string    `json:"id"`
	Provider string    `json:"provider"`
	Label    string    `json:"label,omitempty"`
	Masked   string    `json:"masked"` // e.g. "sk-...a91f" — never the raw key
	AddedAt  time.Time `json:"added_at"`
}

type ListKeysResponse struct {
	Keys []KeyInfo `json:"keys"`
}

// ---------- Stats ----------

type StatsPeriod string

const (
	PeriodDay   StatsPeriod = "day"
	PeriodMonth StatsPeriod = "month"
)

type StatsQuery struct {
	Provider string      `json:"provider,omitempty"` // empty = all providers
	Period   StatsPeriod `json:"period"`
}

type UsageStat struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Requests int    `json:"requests"`
}

type StatsResponse struct {
	Period StatsPeriod `json:"period"`
	Stats  []UsageStat `json:"stats"`
}

// ---------- Pause / resume ----------

type PauseRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"` // empty = pause the whole provider, not just one model
	Paused   bool   `json:"paused"`
}

// ---------- Priority ----------

// PriorityRequest sets routing priority for one model within a provider.
// Lower numbers are tried first — priority 0 is preferred over priority 5.
type PriorityRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Priority int    `json:"priority"`
}
