package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "account", "session.json")
	want := Session{Server: "localhost:50051", Token: "secret", UserID: "user", Name: "alice", ExpiresAt: time.Now().Add(time.Hour).UTC()}
	if err := SaveSession(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != want.Token || got.Server != want.Server {
		t.Fatal("session changed")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("token permissions: %v", info.Mode())
	}
	want.ExpiresAt = time.Now().Add(-time.Hour)
	if err := SaveSession(path, want); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSession(path); err == nil {
		t.Fatal("expired session accepted")
	}
}
func TestTerminalEscapesAreRemoved(t *testing.T) {
	if got := safe("hi\x1b[2J\r\n\u202eevil"); strings.ContainsAny(got, "\x1b\r\n\u202e") {
		t.Fatalf("unsafe output: %q", got)
	}
}
