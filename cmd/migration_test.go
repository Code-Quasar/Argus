package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestOpenMigratesPlaintextStore(t *testing.T) {
	path := t.TempDir() + "/store.json"
	legacy := data{
		Keys:   []KeyRecord{{Provider: "groq", Key: "legacy-secret"}},
		Paused: map[string]bool{}, Priority: map[string]int{}, Stats: map[string]map[string]int{},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if len(store.ListKeys("")) != 1 {
		t.Fatal("legacy key was not loaded")
	}
	secured, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(secured), "legacy-secret") {
		t.Fatal("legacy store remained plaintext")
	}
}
