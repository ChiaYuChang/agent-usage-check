package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	creditsEndpoint    = "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits"
	usageEndpoint      = "https://chatgpt.com/backend-api/wham/usage"
	defaultInterval    = 2 * time.Minute
	defaultConfigDir   = "~/.config/agent-usage-check"
	defaultConfigFile  = "~/.config/agent-usage-check/config.yaml"
	defaultCacheFile   = "~/.config/agent-usage-check/cache.json"
	defaultAuthFile    = "~/.codex/auth.json"
	notApplicable      = "N/A"
)

type authFile struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}

type window struct {
	UsedPercent      *int   `json:"used_percent"`
	RemainingPercent *int   `json:"remaining_percent"`
	ResetAfter       *int64 `json:"reset_after_seconds"`
}

type usageResponse struct {
	RateLimit struct {
		PrimaryWindow   *window `json:"primary_window"`
		SecondaryWindow *window `json:"secondary_window"`
	} `json:"rate_limit"`
	PrimaryWindow   *window `json:"primary_window"`
	SecondaryWindow *window `json:"secondary_window"`
}

type credit struct {
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at"`
}

type creditsResponse struct {
	AvailableCount        *int     `json:"available_count"`
	Credits               []credit `json:"credits"`
	RateLimitResetCredits []credit `json:"rate_limit_reset_credits"`
	Items                 []credit `json:"items"`
}

type status struct {
	Primary   *window  `json:"primary"`
	Secondary *window  `json:"secondary"`
	Credits   []credit `json:"credits"`
}

type cachedStatus struct {
	UpdatedAt time.Time `json:"updated_at"`
	Primary   *window   `json:"primary"`
	Secondary *window   `json:"secondary"`
	Credits   []credit  `json:"credits"`
}

type ashellOutput struct {
	Text string `json:"text"`
	Alt  string `json:"alt"`
}

type displayOptions struct {
	mode          string
	remainingChar string
	usedChar      string
	icon          string
}

type yamlConfig struct {
	Display          string `yaml:"display"`
	Interval         string `yaml:"interval"`
	RemainingChar    string `yaml:"remaining_char"`
	RemainingCharAlt string `yaml:"remaining-char"`
	UsedChar         string `yaml:"used_char"`
	UsedCharAlt      string `yaml:"used-char"`
	Icon             string `yaml:"icon"`
	CachePath        string `yaml:"cache_path"`
	CachePathAlt     string `yaml:"cache-path"`
	AuthPath         string `yaml:"auth_path"`
	AuthPathAlt      string `yaml:"auth-path"`
}

type appConfig struct {
	Display       string
	Interval      time.Duration
	RemainingChar string
	UsedChar      string
	Icon          string
	CachePath     string
	AuthPath      string
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

func creditsFrom(response creditsResponse) []credit {
	if len(response.Credits) > 0 {
		return response.Credits
	}
	if len(response.RateLimitResetCredits) > 0 {
		return response.RateLimitResetCredits
	}
	return response.Items
}

func fetchStatus(ctx context.Context, client *http.Client, authPath string) (status, error) {
	token, err := readToken(authPath)
	if err != nil {
		return status{}, err
	}

	var usage usageResponse
	if err := getJSON(ctx, client, usageEndpoint, token, &usage); err != nil {
		return status{}, err
	}
	var credits creditsResponse
	if err := getJSON(ctx, client, creditsEndpoint, token, &credits); err != nil {
		return status{}, err
	}

	primary := usage.RateLimit.PrimaryWindow
	if primary == nil {
		primary = usage.PrimaryWindow
	}
	secondary := usage.RateLimit.SecondaryWindow
	if secondary == nil {
		secondary = usage.SecondaryWindow
	}
	return status{Primary: primary, Secondary: secondary, Credits: creditsFrom(credits)}, nil
}

func saveCache(path string, st status) error {
	if path == "" {
		return nil
	}
	realPath := expandPath(path)
	if err := os.MkdirAll(filepath.Dir(realPath), 0755); err != nil {
		return err
	}
	c := cachedStatus{
		UpdatedAt: time.Now(),
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

func loadCache(path string) (status, time.Time, error) {
	if path == "" {
		return status{}, time.Time{}, errors.New("empty cache path")
	}
	realPath := expandPath(path)
	contents, err := os.ReadFile(realPath)
	if err != nil {
		return status{}, time.Time{}, err
	}
	var c cachedStatus
	if err := json.Unmarshal(contents, &c); err != nil {
		return status{}, time.Time{}, err
	}
	return status{
		Primary:   c.Primary,
		Secondary: c.Secondary,
		Credits:   c.Credits,
	}, c.UpdatedAt, nil
}

func fetchAndSave(ctx context.Context, client *http.Client, authPath, cachePath string) (status, error) {
	st, err := fetchStatus(ctx, client, authPath)
	if err != nil {
		return status{}, err
	}
	_ = saveCache(cachePath, st)
	return st, nil
}

func percentage(value *window) (int, bool) {
	if value == nil {
		return 0, false
	}
	if value.RemainingPercent != nil {
		return *value.RemainingPercent, true
	}
	if value.UsedPercent != nil {
		return 100 - *value.UsedPercent, true
	}
	return 0, false
}

func bar(value *window, width int, options displayOptions) string {
	remaining, ok := percentage(value)
	if !ok {
		return strings.Repeat("-", width)
	}
	filled := remaining * width / 100
	return strings.Repeat(options.remainingChar, filled) + strings.Repeat(options.usedChar, width-filled)
}

func windowDetails(label string, value *window, options displayOptions) string {
	remaining, ok := percentage(value)
	if !ok {
		return fmt.Sprintf("%s: %s\n[%s]", label, notApplicable, bar(value, 28, options))
	}
	used := 100 - remaining
	details := fmt.Sprintf("%s: %d%% remaining, %d%% used\n[%s]", label, remaining, used, bar(value, 28, options))
	if value.ResetAfter != nil {
		details += "\nReset in: " + durationString(time.Duration(*value.ResetAfter)*time.Second)
	}
	return details
}

func durationString(duration time.Duration) string {
	if duration < time.Minute {
		return "less than 1m"
	}
	minutes := int(duration.Round(time.Minute).Minutes())
	days := minutes / (24 * 60)
	minutes %= 24 * 60
	hours := minutes / 60
	minutes %= 60
	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	parts = append(parts, fmt.Sprintf("%dm", minutes))
	return strings.Join(parts, " ")
}

func report(current status, now time.Time, options displayOptions) string {
	var output strings.Builder
	output.WriteString("Codex usage\n\n")
	output.WriteString(windowDetails("5h", current.Primary, options))
	output.WriteString("\n\n")
	output.WriteString(windowDetails("1w", current.Secondary, options))
	output.WriteString("\n\nReload Token\n")

	index := 0
	for _, credit := range current.Credits {
		if credit.Status != "available" {
			continue
		}
		index++
		expiresAt, err := time.Parse(time.RFC3339Nano, credit.ExpiresAt)
		if err != nil {
			fmt.Fprintf(&output, "- #%03d: Expiry unavailable\n", index)
			continue
		}
		fmt.Fprintf(&output, "- #%03d: Expires in %s\n", index, durationString(expiresAt.Sub(now)))
	}
	if index == 0 {
		output.WriteString("- None available\n")
	}
	return output.String()
}

func printStatus(current status, err error, options displayOptions) {
	summary := fmt.Sprintf("5h: %s  1w: %s", notApplicable, notApplicable)
	if options.icon != "" {
		summary = options.icon + "  " + summary
	}
	output := ashellOutput{Text: summary, Alt: "error"}
	if err == nil {
		primary, primaryOK := percentage(current.Primary)
		secondary, secondaryOK := percentage(current.Secondary)
		primaryText := notApplicable
		secondaryText := notApplicable
		if primaryOK {
			if options.mode == "used" {
				primaryText = fmt.Sprintf("%d%%", 100-primary)
			} else {
				primaryText = fmt.Sprintf("%d%%", primary)
			}
		}
		if secondaryOK {
			if options.mode == "used" {
				secondaryText = fmt.Sprintf("%d%%", 100-secondary)
			} else {
				secondaryText = fmt.Sprintf("%d%%", secondary)
			}
		}
		summary = fmt.Sprintf("5h: %s  1w: %s", primaryText, secondaryText)
		if options.icon != "" {
			summary = options.icon + "  " + summary
		}
		output.Text = summary
		if (primaryOK && primary == 0) || (secondaryOK && secondary == 0) {
			output.Alt = "exhausted"
		} else {
			output.Alt = "available"
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(output)
}

func main() {
	configPathFlag := flag.String("config", "", "path to YAML configuration file")
	details := flag.Bool("details", false, "print the detailed usage report")
	once := flag.Bool("once", false, "print one bar update and exit")
	modeFlag := flag.String("display", "", "bar value: remaining or used")
	refreshIntervalFlag := flag.Duration("interval", 0, "how often to refresh the bar")
	remainingCharFlag := flag.String("remaining-char", "", "progress-bar character for remaining usage")
	usedCharFlag := flag.String("used-char", "", "progress-bar character for used usage")
	iconFlag := flag.String("icon", "", "icon displayed before the bar summary")
	cachePathFlag := flag.String("cache-path", "", "path to cache file")
	authPathFlag := flag.String("auth-path", "", "path to auth file")
	flag.Parse()

	visitedFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		visitedFlags[f.Name] = true
	})

	// Defaults
	cfg := appConfig{
		Display:       "remaining",
		Interval:      defaultInterval,
		RemainingChar: "■",
		UsedChar:      "□",
		Icon:          "󰫣",
		CachePath:     defaultCacheFile,
		AuthPath:      defaultAuthFile,
	}

	// Determine config file path
	configFileToRead := defaultConfigFile
	if visitedFlags["config"] {
		configFileToRead = *configPathFlag
	}

	// Attempt reading YAML config
	configPathResolved := expandPath(configFileToRead)
	data, err := os.ReadFile(configPathResolved)
	if err == nil {
		var yc yamlConfig
		if err := yaml.Unmarshal(data, &yc); err == nil {
			if yc.Display != "" {
				cfg.Display = yc.Display
			}
			if yc.Interval != "" {
				if d, err := time.ParseDuration(yc.Interval); err == nil {
					cfg.Interval = d
				}
			}
			if yc.RemainingChar != "" {
				cfg.RemainingChar = yc.RemainingChar
			} else if yc.RemainingCharAlt != "" {
				cfg.RemainingChar = yc.RemainingCharAlt
			}
			if yc.UsedChar != "" {
				cfg.UsedChar = yc.UsedChar
			} else if yc.UsedCharAlt != "" {
				cfg.UsedChar = yc.UsedCharAlt
			}
			if yc.Icon != "" {
				cfg.Icon = yc.Icon
			}
			if yc.CachePath != "" {
				cfg.CachePath = yc.CachePath
			} else if yc.CachePathAlt != "" {
				cfg.CachePath = yc.CachePathAlt
			}
			if yc.AuthPath != "" {
				cfg.AuthPath = yc.AuthPath
			} else if yc.AuthPathAlt != "" {
				cfg.AuthPath = yc.AuthPathAlt
			}
		}
	} else if visitedFlags["config"] {
		fmt.Fprintf(os.Stderr, "Unable to read config file at %s: %v\n", configFileToRead, err)
		return
	}

	// Override with CLI flags if explicitly set
	if visitedFlags["display"] {
		cfg.Display = *modeFlag
	}
	if visitedFlags["interval"] {
		cfg.Interval = *refreshIntervalFlag
	}
	if visitedFlags["remaining-char"] {
		cfg.RemainingChar = *remainingCharFlag
	}
	if visitedFlags["used-char"] {
		cfg.UsedChar = *usedCharFlag
	}
	if visitedFlags["icon"] {
		cfg.Icon = *iconFlag
	}
	if visitedFlags["cache-path"] {
		cfg.CachePath = *cachePathFlag
	}
	if visitedFlags["auth-path"] {
		cfg.AuthPath = *authPathFlag
	}

	// Validate options
	if cfg.Display != "remaining" && cfg.Display != "used" {
		fmt.Fprintln(os.Stderr, "--display / config display must be remaining or used")
		return
	}
	if cfg.Interval <= 0 {
		fmt.Fprintln(os.Stderr, "--interval / config interval must be greater than zero")
		return
	}
	if cfg.RemainingChar == "" || cfg.UsedChar == "" {
		fmt.Fprintln(os.Stderr, "progress-bar characters cannot be empty")
		return
	}

	options := displayOptions{
		mode:          cfg.Display,
		remainingChar: cfg.RemainingChar,
		usedChar:      cfg.UsedChar,
		icon:          cfg.Icon,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := &http.Client{Timeout: 15 * time.Second}

	if *details {
		current, err := fetchAndSave(ctx, client, cfg.AuthPath, cfg.CachePath)
		if err != nil {
			// Fallback to cache if available
			cachedSt, _, cacheErr := loadCache(cfg.CachePath)
			if cacheErr == nil {
				fmt.Print(report(cachedSt, time.Now(), options))
				return
			}
			fmt.Fprintln(os.Stderr, "Unable to load Codex usage:", err)
			return
		}
		fmt.Print(report(current, time.Now(), options))
		return
	}

	update := func() {
		current, err := fetchAndSave(ctx, client, cfg.AuthPath, cfg.CachePath)
		if err != nil {
			// If error, check if cache is available for fallback status display
			cachedSt, _, cacheErr := loadCache(cfg.CachePath)
			if cacheErr == nil {
				printStatus(cachedSt, nil, options)
				return
			}
		}
		printStatus(current, err, options)
	}

	update()
	if *once {
		return
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			update()
		}
	}
}
