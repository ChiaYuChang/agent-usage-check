package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"agent-usage-check/internal/cache"
	"agent-usage-check/internal/config"
	"agent-usage-check/internal/formatter"
	"agent-usage-check/internal/provider"
	"agent-usage-check/internal/provider/agy"
	"agent-usage-check/internal/provider/openai"
	"agent-usage-check/internal/provider/opencodego"
)

// version is overridden at build time via:
// go build -ldflags "-X main.version=1.0.0" ./cmd
var version = "dev"

func getProvider(cfg config.Config, httpClient *http.Client) provider.Provider {
	switch strings.ToLower(cfg.Provider) {
	case "agy", "antigravity":
		return agy.New(cfg.AgyPath, cfg.AgyGroup)
	case "opencode-go", "opencode", "go":
		return opencodego.New(cfg.OpencodeAPIKey, cfg.OpencodeAuthPath, cfg.OpencodeEndpoint, httpClient)
	default:
		return openai.New(cfg.AuthPath, httpClient)
	}
}

func fetchAndSave(ctx context.Context, p provider.Provider, cachePath string) (*provider.UsageStatus, error) {
	st, err := p.FetchUsage(ctx)
	if err != nil {
		return nil, err
	}
	_ = cache.Save(cachePath, st)
	return st, nil
}

func main() {
	configPathFlag := flag.String("config", "", "path to YAML configuration file")
	providerFlag := flag.String("provider", "", "provider to use: openai, agy or opencode-go")
	details := flag.Bool("details", false, "print the detailed usage report")
	once := flag.Bool("once", false, "print one bar update and exit")
	modeFlag := flag.String("display", "", "bar value: remaining or used")
	refreshIntervalFlag := flag.Duration("interval", 0, "how often to refresh the bar")
	remainingCharFlag := flag.String("remaining-char", "", "progress-bar character for remaining usage")
	usedCharFlag := flag.String("used-char", "", "progress-bar character for used usage")
	iconFlag := flag.String("icon", "", "icon displayed before the bar summary")
	cachePathFlag := flag.String("cache-path", "", "path to cache file")
	authPathFlag := flag.String("auth-path", "", "path to auth file (openai)")
	agyPathFlag := flag.String("agy-path", "", "path to agy executable (agy)")
	agyGroupFlag := flag.String("agy-group", "", "filter target model group (agy)")
	opencodeAPIKeyFlag := flag.String("opencode-api-key", "", "API key for OpenCode Go (opencode-go)")
	opencodeAuthPathFlag := flag.String("opencode-auth-path", "", "path to opencode auth.json (opencode-go)")
	opencodeEndpointFlag := flag.String("opencode-endpoint", "", "usage endpoint URL (opencode-go)")
	jsonFlag := flag.Bool("json", false, "output status in structured JSON format")
	outputPathFlag := flag.String("output", "", "write status JSON to a file")
	watchFlag := flag.Bool("watch", false, "continuously watch and refresh status at configured interval")
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println("agent-usage-check", version)
		return
	}

	visitedFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		visitedFlags[f.Name] = true
	})

	cfg := config.DefaultConfig()

	// Determine config file path
	configFileToRead := config.DefaultConfigFile
	if visitedFlags["config"] {
		configFileToRead = *configPathFlag
	}

	// Attempt reading YAML config
	if err := config.LoadYAML(configFileToRead, &cfg); err != nil && visitedFlags["config"] {
		fmt.Fprintf(os.Stderr, "Unable to read config file at %s: %v\n", configFileToRead, err)
		return
	}

	// Override with CLI flags if explicitly set
	if visitedFlags["provider"] {
		cfg.Provider = *providerFlag
	}
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
		cfg.Icons = nil
	}
	if visitedFlags["cache-path"] {
		cfg.CachePath = *cachePathFlag
	}
	if visitedFlags["auth-path"] {
		cfg.AuthPath = *authPathFlag
	}
	if visitedFlags["agy-path"] {
		cfg.AgyPath = *agyPathFlag
	}
	if visitedFlags["agy-group"] {
		cfg.AgyGroup = *agyGroupFlag
	}
	if visitedFlags["opencode-api-key"] {
		cfg.OpencodeAPIKey = *opencodeAPIKeyFlag
	}
	if visitedFlags["opencode-auth-path"] {
		cfg.OpencodeAuthPath = *opencodeAuthPathFlag
	}
	if visitedFlags["opencode-endpoint"] {
		cfg.OpencodeEndpoint = *opencodeEndpointFlag
	}
	if visitedFlags["json"] {
		cfg.JSON = *jsonFlag
	}
	if visitedFlags["watch"] {
		cfg.Watch = *watchFlag
	}
	if visitedFlags["once"] && *once {
		cfg.Watch = false
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	httpClient := &http.Client{Timeout: 15 * time.Second}
	prov := getProvider(cfg, httpClient)

	options := formatter.DisplayOptions{
		Mode:          cfg.Display,
		RemainingChar: cfg.RemainingChar,
		UsedChar:      cfg.UsedChar,
		Icon:          cfg.GetIcon(prov.Name()),
		JSON:          cfg.JSON,
	}

	if *details {
		current, err := fetchAndSave(ctx, prov, cfg.CachePath)
		if err != nil {
			// Fallback to cache if available
			cachedSt, _, cacheErr := cache.Load(cfg.CachePath)
			if cacheErr == nil {
				if *outputPathFlag != "" {
					if writeErr := formatter.WriteStatusJSON(*outputPathFlag, cachedSt, nil, options); writeErr != nil {
						fmt.Fprintln(os.Stderr, "Unable to write status JSON:", writeErr)
					}
				}
				fmt.Print(formatter.Report(cachedSt, time.Now(), options))
				return
			}
			fmt.Fprintf(os.Stderr, "Unable to load %s usage: %v\n", prov.Name(), err)
			return
		}
		if *outputPathFlag != "" {
			if writeErr := formatter.WriteStatusJSON(*outputPathFlag, current, nil, options); writeErr != nil {
				fmt.Fprintln(os.Stderr, "Unable to write status JSON:", writeErr)
			}
		}
		fmt.Print(formatter.Report(current, time.Now(), options))
		return
	}

	update := func() {
		current, err := fetchAndSave(ctx, prov, cfg.CachePath)
		if err != nil {
			// If error, check if cache is available for fallback status display
			cachedSt, _, cacheErr := cache.Load(cfg.CachePath)
			if cacheErr == nil {
				formatter.EmitStatus(cachedSt, nil, options, *outputPathFlag)
				return
			}
		}
		formatter.EmitStatus(current, err, options, *outputPathFlag)
	}

	update()
	if !cfg.Watch {
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
