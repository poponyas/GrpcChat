package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Session struct {
	Server    string    `json:"server"`
	TLS       bool      `json:"tls,omitempty"`
	CA        string    `json:"ca,omitempty"`
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"expires_at"`
}

func DefaultSessionPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "grpcchat", "session.json"), nil
}
func LoadSession(path string) (Session, error) {
	var s Session
	data, err := os.ReadFile(path)
	if err != nil {
		return s, fmt.Errorf("session: %w; run chat-cli register or chat-cli login", err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("invalid session file: %w", err)
	}
	if s.Token == "" || s.UserID == "" || s.Server == "" {
		return s, errors.New("incomplete session; log in again")
	}
	if !s.ExpiresAt.After(time.Now()) {
		return s, errors.New("session expired; log in again")
	}
	return s, nil
}
func SaveSession(path string, s Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".session-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err = file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
