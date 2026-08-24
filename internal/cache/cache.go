package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-usage-check/internal/provider"
)

type CachedStatus struct {
	UpdatedAt time.Time             `json:"updated_at"`
	Provider  string                `json:"provider"`
	Groups    []provider.Group      `json:"groups,omitempty"`
	Primary   *provider.Window      `json:"primary,omitempty"`
	Secondary *provider.Window      `json:"secondary,omitempty"`
	Credits   []provider.Credit     `json:"credits,omitempty"`
}

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func Save(path string, st *provider.UsageStatus) error {
	if path == "" || st == nil {
		return nil
	}
	realPath := ExpandPath(path)
	if err := os.MkdirAll(filepath.Dir(realPath), 0755); err != nil {
		return err
	}
	c := CachedStatus{
		UpdatedAt: time.Now(),
		Provider:  st.Provider,
		Groups:    st.Groups,
		Primary:   st.Primary,
		Secondary: st.Secondary,
		Credits:   st.Credits,
	}
	bytes, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(realPath, bytes, 0644)
}

func Load(path string) (*provider.UsageStatus, time.Time, error) {
	if path == "" {
		return nil, time.Time{}, errors.New("empty cache path")
	}
	realPath := ExpandPath(path)
	contents, err := os.ReadFile(realPath)
	if err != nil {
		return nil, time.Time{}, err
	}
	var c CachedStatus
	if err := json.Unmarshal(contents, &c); err != nil {
		return nil, time.Time{}, err
	}
	st := &provider.UsageStatus{
		Provider:  c.Provider,
		UpdatedAt: c.UpdatedAt,
		Groups:    c.Groups,
		Primary:   c.Primary,
		Secondary: c.Secondary,
		Credits:   c.Credits,
	}
	return st, c.UpdatedAt, nil
}
