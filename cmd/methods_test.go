package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestEncryptedStoreRoundTrip(t *testing.T) {
	path := t.TempDir() + "/store.json"
	store := &Store{path: path, password: "correct horse battery staple", d: emptyData()}
	if _, err := store.AddKey("gemini", "secret-api-key", "test"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPriority("gemini", "gemini-test", 2); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("store permissions = %o, want 600", info.Mode().Perm())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-api-key") {
		t.Fatal("plaintext API key found in encrypted store")
	}

	unlocked, err := Open(path, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	keys := unlocked.ListKeys("")
	if len(keys) != 1 || keys[0].Key != "secret-api-key" {
		t.Fatalf("keys = %+v", keys)
	}
	if _, err := Open(path, "wrong password"); err == nil {
		t.Fatal("wrong password unexpectedly unlocked store")
	}
}
