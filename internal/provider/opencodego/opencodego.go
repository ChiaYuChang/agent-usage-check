package opencodego

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"agent-usage-check/internal/provider"
)

const (
	// DefaultEndpoint is the official (undocumented) quota endpoint the web
	// console is built on: GET with `Authorization: Bearer <API key>`.
	DefaultEndpoint = "https://opencode.ai/zen/go/v1/usage"
)

// usageWindow is one quota window as reported by the usage endpoint.
// percent is the *used* percentage, resetsAt is an RFC3339 timestamp.
type usageWindow struct {
	Status   string   `json:"status"`
	Percent  *float64 `json:"percent"`
	ResetsAt string   `json:"resetsAt"`
}

type usageResponse struct {
	Usage struct {
		Rolling *usageWindow `json:"rolling"`
		Weekly  *usageWindow `json:"weekly"`
		Monthly *usageWindow `json:"monthly"`
	} `json:"usage"`
}

type Provider struct {
	// APIKey explicitly overrides env/auth-file resolution when non-empty.
	APIKey string
	// AuthPath overrides the auto-detected opencode auth.json location.
	AuthPath   string
	Endpoint   string
	HTTPClient *http.Client
}

func New(apiKey, authPath, endpoint string, httpClient *http.Client) *Provider {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Provider{
		APIKey:     strings.TrimSpace(apiKey),
		AuthPath:   authPath,
		Endpoint:   endpoint,
		HTTPClient: httpClient,
	}
}

func (p *Provider) Name() string {
	return "opencode-go"
}

// AuthFilePath returns the opencode auth.json location written by
// `opencode auth login`, honouring an explicit override first and
// $OPENCODE_AUTH_JSON second.
func AuthFilePath(override string) string {
	if strings.TrimSpace(override) != "" {
		return expandPath(strings.TrimSpace(override))
	}
	if v := strings.TrimSpace(os.Getenv("OPENCODE_AUTH_JSON")); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" && home != "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "opencode", "auth.json")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "opencode", "auth.json")
	default:
		base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
		if base == "" {
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "opencode", "auth.json")
	}
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// resolveAPIKey mirrors opencode's own resolution order:
// explicit option -> OPENCODE_API_KEY env -> auth.json
// (opencode-go entry, then legacy opencode entry).
func (p *Provider) resolveAPIKey() (string, error) {
	if p.APIKey != "" {
		return p.APIKey, nil
	}
	if v := strings.TrimSpace(os.Getenv("OPENCODE_API_KEY")); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("ZEN_API_KEY")); v != "" {
		return v, nil
	}
	if key, err := readAuthFileKey(AuthFilePath(p.AuthPath)); err == nil && key != "" {
		return key, nil
	}
	return "", fmt.Errorf("no OpenCode API key found (set --opencode-api-key, OPENCODE_API_KEY, or run `opencode auth login`)")
}

func readAuthFileKey(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var raw map[string]any
	if err := json.Unmarshal(contents, &raw); err != nil {
		return "", err
	}
	for _, entryName := range []string{"opencode-go", "opencode"} {
		entry, ok := raw[entryName]
		if !ok {
			continue
		}
		if s, ok := entry.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s), nil
		}
		obj, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, field := range []string{"key", "apiKey", "api_key", "token", "access_token"} {
			if v, ok := obj[field].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v), nil
			}
		}
	}
	return "", fmt.Errorf("no opencode-go/opencode key in %s", path)
}

func clampPercent(v float64) int {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	used := int(math.Round(v))
	if used < 0 {
		return 0
	}
	if used > 100 {
		return 100
	}
	return used
}

func convertWindow(w *usageWindow, now time.Time) *provider.Window {
	if w == nil {
		return nil
	}
	res := &provider.Window{}
	if w.Percent != nil {
		used := clampPercent(*w.Percent)
		rem := 100 - used
		res.UsedPercent = &used
		res.RemainingPercent = &rem
	}
	if w.ResetsAt != "" {
		var t time.Time
		var err error
		if t, err = time.Parse(time.RFC3339Nano, w.ResetsAt); err != nil {
			t, err = time.Parse(time.RFC3339, w.ResetsAt)
		}
		if err == nil {
			res.ResetTime = &t
			dur := t.Sub(now)
			if dur < 0 {
				dur = 0
			}
			res.ResetAfter = &dur
		}
	}
	return res
}

func (p *Provider) FetchUsage(ctx context.Context) (*provider.UsageStatus, error) {
	key, err := p.resolveAPIKey()
	if err != nil {
		return nil, err
	}

	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "agent-usage-check")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("OpenCode Go API key was rejected (HTTP 401)")
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("no OpenCode Go subscription on this key (HTTP 403)")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	var usage usageResponse
	if err := json.NewDecoder(resp.Body).Decode(&usage); err != nil {
		return nil, fmt.Errorf("failed to parse opencode-go usage: %w", err)
	}

	now := time.Now()
	rolling := convertWindow(usage.Usage.Rolling, now)
	weekly := convertWindow(usage.Usage.Weekly, now)
	monthly := convertWindow(usage.Usage.Monthly, now)

	if rolling == nil && weekly == nil && monthly == nil {
		return nil, fmt.Errorf("no usage windows found in opencode-go response")
	}

	windows := map[string]*provider.Window{}
	if rolling != nil {
		windows["5h"] = rolling
		windows["rolling"] = rolling
	}
	if weekly != nil {
		windows["1w"] = weekly
		windows["weekly"] = weekly
	}
	if monthly != nil {
		windows["monthly"] = monthly
		windows["1m"] = monthly
	}

	group := provider.Group{
		Name:      "OpenCode Go",
		Windows:   windows,
		Primary:   rolling,
		Secondary: weekly,
	}

	return &provider.UsageStatus{
		Provider:  p.Name(),
		UpdatedAt: now,
		Groups:    []provider.Group{group},
		Primary:   rolling,
		Secondary: weekly,
		Credits:   nil, // Go is a fixed subscription, no reload tokens
	}, nil
}
