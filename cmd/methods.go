package cmd

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	storeVersion = 1
	saltSize     = 16
)

type encryptedStore struct {
	Version    int    `json:"version"`
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// Store wraps a JSON file on disk with an in-process mutex. Open it once
// per command invocation, mutate, and it saves on every write — there's
// no separate explicit Save() the caller needs to remember to call.
type Store struct {
	path     string
	password string
	mu       sync.Mutex
	d        data
}

// OpenDefault opens the store used by all CLI commands.
func OpenDefault() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		password, err := createPassword()
		if err != nil {
			return nil, err
		}
		s := &Store{path: path, password: password, d: emptyData()}
		if err := s.saveWithPassword(password); err != nil {
			return nil, err
		}
		return s, nil
	} else if err != nil {
		return nil, fmt.Errorf("check %s: %w", path, err)
	}

	password, err := readPassword("Password: ")
	if err != nil {
		return nil, err
	}
	return Open(path, password)
}

func OpenWithPassword(password string) (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return Open(path, password)
}

// Open loads and decrypts the store from path.
func Open(path, password string) (*Store, error) {
	s := &Store{path: path, password: password, d: emptyData()}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var envelope encryptedStore
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Version != storeVersion || len(envelope.Salt) != saltSize {
		// Migrate the previous plaintext format after the user unlocks it.
		if err := json.Unmarshal(raw, &s.d); err != nil {
			return nil, fmt.Errorf("parse encrypted store: %w", err)
		}
		initializeData(&s.d)
		if err := s.save(); err != nil {
			return nil, fmt.Errorf("encrypt existing store: %w", err)
		}
		return s, nil
	}
	plaintext, err := decrypt(envelope, password)
	if err != nil {
		return nil, fmt.Errorf("unlock store: incorrect password or damaged store")
	}
	if err := json.Unmarshal(plaintext, &s.d); err != nil {
		return nil, fmt.Errorf("parse decrypted store: %w", err)
	}
	initializeData(&s.d)
	return s, nil
}

func initializeData(value *data) {
	if value.Paused == nil {
		value.Paused = make(map[string]bool)
	}
	if value.Priority == nil {
		value.Priority = make(map[string]int)
	}
	if value.Stats == nil {
		value.Stats = make(map[string]map[string]int)
	}
}

// save writes the store atomically: write to a temp file, then rename
// over the real path, so a crash mid-write never leaves a corrupt file.
func (s *Store) save() error {
	return s.saveWithPassword(s.password)
}

func (s *Store) saveWithPassword(password string) error {
	plaintext, err := json.Marshal(s.d)
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}

	envelope, err := encrypt(plaintext, password)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode encrypted store: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	defer os.Remove(tmp)
	if err := os.Chmod(tmp, 0600); err != nil {
		return fmt.Errorf("secure temp file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace store file: %w", err)
	}
	return nil
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
}

func encrypt(plaintext []byte, password string) (encryptedStore, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return encryptedStore{}, fmt.Errorf("create store salt: %w", err)
	}
	block, err := aes.NewCipher(deriveKey(password, salt))
	if err != nil {
		return encryptedStore{}, fmt.Errorf("create store cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return encryptedStore{}, fmt.Errorf("create store encryption: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return encryptedStore{}, fmt.Errorf("create store nonce: %w", err)
	}
	return encryptedStore{
		Version:    storeVersion,
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: gcm.Seal(nil, nonce, plaintext, nil),
	}, nil
}

func decrypt(envelope encryptedStore, password string) ([]byte, error) {
	if envelope.Version != storeVersion || len(envelope.Salt) != saltSize {
		return nil, fmt.Errorf("unsupported encrypted store")
	}
	block, err := aes.NewCipher(deriveKey(password, envelope.Salt))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(envelope.Nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid store nonce")
	}
	return gcm.Open(nil, envelope.Nonce, envelope.Ciphertext, nil)
}

func scopeKey(provider, model string) string {
	if model == "" {
		return provider
	}
	return provider + "/" + model
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- Keys ----------

func (s *Store) AddKey(provider, key, label string) (KeyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := KeyRecord{
		ID:       newID(),
		Provider: provider,
		Key:      key,
		Label:    label,
		AddedAt:  time.Now(),
	}
	s.d.Keys = append(s.d.Keys, rec)

	if err := s.save(); err != nil {
		return KeyRecord{}, err
	}
	return rec, nil
}

// ListKeys returns keys, optionally filtered by provider ("" = all).
func (s *Store) ListKeys(provider string) []KeyRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	if provider == "" {
		out := make([]KeyRecord, len(s.d.Keys))
		copy(out, s.d.Keys)
		return out
	}

	var out []KeyRecord
	for _, k := range s.d.Keys {
		if k.Provider == provider {
			out = append(out, k)
		}
	}
	return out
}

// ---------- Pause / resume ----------

func (s *Store) SetPaused(provider, model string, paused bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := scopeKey(provider, model)
	if paused {
		s.d.Paused[key] = true
	} else {
		delete(s.d.Paused, key) // absence means "not paused" — keeps the file tidy
	}
	return s.save()
}

// IsPaused checks both the specific model and the whole-provider pause,
// since pausing a provider should silence every model under it.
func (s *Store) IsPaused(provider, model string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.d.Paused[provider] {
		return true
	}
	return s.d.Paused[scopeKey(provider, model)]
}

// ---------- Priority ----------

func (s *Store) SetPriority(provider, model string, priority int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.d.Priority[scopeKey(provider, model)] = priority
	return s.save()
}

// Priority returns the configured priority for a model, defaulting to 0
// (highest preference) if it was never explicitly set.
func (s *Store) Priority(provider, model string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.d.Priority[scopeKey(provider, model)]
}

// ---------- Stats ----------

// RecordRequest increments today's counter for a provider/model. The
// router should call this once per completed request.
func (s *Store) RecordRequest(provider, model string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if s.d.Stats[today] == nil {
		s.d.Stats[today] = make(map[string]int)
	}
	s.d.Stats[today][scopeKey(provider, model)]++
	return s.save()
}

// StatKind selects how StatsFor aggregates.
type StatKind string

const (
	StatsDay   StatKind = "day"
	StatsMonth StatKind = "month"
)

// StatsFor returns request counts keyed by "provider/model", summed over
// today (StatsDay) or the current calendar month (StatsMonth).
// Optionally filtered to one provider.
func (s *Store) StatsFor(kind StatKind, provider string) map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	out := make(map[string]int)

	for dateStr, counts := range s.d.Stats {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue // skip malformed entries rather than fail the whole read
		}

		matches := false
		switch kind {
		case StatsDay:
			matches = date.Format("2006-01-02") == now.Format("2006-01-02")
		case StatsMonth:
			matches = date.Year() == now.Year() && date.Month() == now.Month()
		}
		if !matches {
			continue
		}

		for scope, count := range counts {
			if provider != "" {
				// scope is "provider" or "provider/model" — only keep
				// entries belonging to the requested provider.
				if scope != provider && !hasPrefix(scope, provider+"/") {
					continue
				}
			}
			out[scope] += count
		}
	}
	return out
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// Path returns the file path this store is backed by, mainly for CLI
// output like "stored at ~/.argus/store.json".
func (s *Store) Path() string {
	return s.path
}

// AddCustomModel stores a custom model configuration
func (s *Store) AddCustomModel(model CustomModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if this provider+model combination already exists
	for i, existing := range s.d.CustomModels {
		if existing.Provider == model.Provider && existing.Model == model.Model {
			// Update existing entry
			s.d.CustomModels[i] = model
			return s.save()
		}
	}

	// Add new entry
	s.d.CustomModels = append(s.d.CustomModels, model)
	return s.save()
}

// ListCustomModels returns all configured custom models
func (s *Store) ListCustomModels() []CustomModel {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]CustomModel, len(s.d.CustomModels))
	copy(result, s.d.CustomModels)
	return result
}

// RemoveCustomModel removes a custom model by provider and model name
func (s *Store) RemoveCustomModel(provider, modelName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.d.CustomModels {
		if existing.Provider == provider && existing.Model == modelName {
			// Remove this entry
			s.d.CustomModels = append(s.d.CustomModels[:i], s.d.CustomModels[i+1:]...)
			return s.save()
		}
	}

	return fmt.Errorf("custom model %s/%s not found", provider, modelName)
}

// GetCustomModel retrieves a specific custom model
func (s *Store) GetCustomModel(provider, modelName string) (*CustomModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.d.CustomModels {
		if existing.Provider == provider && existing.Model == modelName {
			return &existing, nil
		}
	}

	return nil, fmt.Errorf("custom model %s/%s not found", provider, modelName)
}

