package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-usage-check/internal/provider"
)

const (
	DefaultCreditsEndpoint = "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits"
	DefaultUsageEndpoint   = "https://chatgpt.com/backend-api/wham/usage"
	DefaultAuthFile        = "~/.codex/auth.json"
)

type authFile struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}

type windowResponse struct {
	UsedPercent      *int   `json:"used_percent"`
	RemainingPercent *int   `json:"remaining_percent"`
	ResetAfter       *int64 `json:"reset_after_seconds"`
}

type usageResponse struct {
	RateLimit struct {
		PrimaryWindow   *windowResponse `json:"primary_window"`
		SecondaryWindow *windowResponse `json:"secondary_window"`
	} `json:"rate_limit"`
	PrimaryWindow   *windowResponse `json:"primary_window"`
	SecondaryWindow *windowResponse `json:"secondary_window"`
}

type creditItem struct {
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at"`
}

type creditsResponse struct {
	AvailableCount        *int         `json:"available_count"`
	Credits               []creditItem `json:"credits"`
	RateLimitResetCredits []creditItem `json:"rate_limit_reset_credits"`
	Items                 []creditItem `json:"items"`
}

type Provider struct {
	AuthPath        string
	CreditsEndpoint string
	UsageEndpoint   string
	HTTPClient      *http.Client
}

func New(authPath string, httpClient *http.Client) *Provider {
	if authPath == "" {
		authPath = DefaultAuthFile
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Provider{
		AuthPath:        authPath,
		CreditsEndpoint: DefaultCreditsEndpoint,
		UsageEndpoint:   DefaultUsageEndpoint,
		HTTPClient:      httpClient,
	}
}

func (p *Provider) Name() string {
	return "openai"
}

func expandPath(path string) string {
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

func readToken(path string) (string, error) {
	contents, err := os.ReadFile(expandPath(path))
	if err != nil {
		return "", err
	}

	var auth authFile
	if err := json.Unmarshal(contents, &auth); err != nil {
		return "", err
	}
	if auth.Tokens.AccessToken == "" {
		return "", errors.New("access token is missing")
	}
	return auth.Tokens.AccessToken, nil
}

func getJSON(ctx context.Context, client *http.Client, endpoint, token string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("User-Agent", "agent-usage-check")

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(destination)
}

func creditsFrom(response creditsResponse) []creditItem {
	if len(response.Credits) > 0 {
		return response.Credits
	}
	if len(response.RateLimitResetCredits) > 0 {
		return response.RateLimitResetCredits
	}
	return response.Items
}

func convertWindow(w *windowResponse) *provider.Window {
	if w == nil {
		return nil
	}
	res := &provider.Window{
		UsedPercent:      w.UsedPercent,
		RemainingPercent: w.RemainingPercent,
	}
	if w.ResetAfter != nil {
		dur := time.Duration(*w.ResetAfter) * time.Second
		res.ResetAfter = &dur
		t := time.Now().Add(dur)
		res.ResetTime = &t
	}
	return res
}

func (p *Provider) FetchUsage(ctx context.Context) (*provider.UsageStatus, error) {
	token, err := readToken(p.AuthPath)
	if err != nil {
		return nil, err
	}

	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	creditsEp := p.CreditsEndpoint
	if creditsEp == "" {
		creditsEp = DefaultCreditsEndpoint
	}
	usageEp := p.UsageEndpoint
	if usageEp == "" {
		usageEp = DefaultUsageEndpoint
	}

	var usage usageResponse
	if err := getJSON(ctx, client, usageEp, token, &usage); err != nil {
		return nil, err
	}
	var credits creditsResponse
	if err := getJSON(ctx, client, creditsEp, token, &credits); err != nil {
		return nil, err
	}

	primaryResp := usage.RateLimit.PrimaryWindow
	if primaryResp == nil {
		primaryResp = usage.PrimaryWindow
	}
	secondaryResp := usage.RateLimit.SecondaryWindow
	if secondaryResp == nil {
		secondaryResp = usage.SecondaryWindow
	}

	primaryWin := convertWindow(primaryResp)
	secondaryWin := convertWindow(secondaryResp)

	creditItems := creditsFrom(credits)
	var standardCredits []provider.Credit
	for _, c := range creditItems {
		var exp time.Time
		if c.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, c.ExpiresAt); err == nil {
				exp = t
			} else if t, err := time.Parse(time.RFC3339, c.ExpiresAt); err == nil {
				exp = t
			}
		}
		standardCredits = append(standardCredits, provider.Credit{
			Status:    c.Status,
			ExpiresAt: exp,
		})
	}

	group := provider.Group{
		Name:      "OpenAI (Codex)",
		Windows:   map[string]*provider.Window{},
		Primary:   primaryWin,
		Secondary: secondaryWin,
	}
	if primaryWin != nil {
		group.Windows["5h"] = primaryWin
	}
	if secondaryWin != nil {
		group.Windows["1w"] = secondaryWin
	}

	return &provider.UsageStatus{
		Provider:  p.Name(),
		UpdatedAt: time.Now(),
		Groups:    []provider.Group{group},
		Primary:   primaryWin,
		Secondary: secondaryWin,
		Credits:   standardCredits,
	}, nil
}
