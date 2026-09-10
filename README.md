# agent-usage-check

A token-optimized, highly customizable CLI and status bar integration tool written in Go for monitoring AI agent usage rate limits and reset tokens.

Supports **OpenAI (ChatGPT / Codex)** backend APIs, **Antigravity (`agy`)** CLI, and **OpenCode Go** (`https://opencode.ai/zen/go/v1/usage`) out-of-the-box, with an extensible modular provider architecture.

## Features

- **Multi-Provider Support**: Supports OpenAI Codex API, Antigravity (`agy`), and OpenCode Go with a modular Provider interface.
- **Status Bar Integration**: Emits JSON formatted for Linux desktop status bars (Ashell, Waybar, etc.).
- **Desktop Notifications**: `--details` mode generates detailed ASCII progress bar reports for `notify-send`.
- **Instant Response & Smart Caching**: Automatically updates `~/.config/agent-usage-check/cache.json` on fetch and falls back to cached state when offline.
- **YAML Configuration**: Supports `~/.config/agent-usage-check/config.yaml`.
- **First-Class Nix & Home-Manager Support**: Built with Nix Flakes and exports ready-to-use Home-Manager modules.

## Architecture

```
bin/
  agent-usage-check
cmd/
  main.go
internal/
  cache/
  config/
  formatter/
    provider/
      provider.go  <- interface & core models
      openai/      <- OpenAI / Codex implementation
      agy/         <- Antigravity (agy) implementation
      opencodego/  <- OpenCode Go implementation
```

## Installation & Configuration

### Home-Manager (Nix Flake)

Add `agent-usage-check` to your `flake.nix`:

```nix
{
  inputs = {
    agent-usage-check.url = "github:cychang/agent-usage-check"; # Or path:~/Projects/agent-usage-check
  };

  outputs = { home-manager, agent-usage-check, ... }: {
    homeConfigurations."user" = home-manager.lib.homeManagerConfiguration {
      modules = [
        agent-usage-check.homeManagerModules.default
        {
          programs.agent-usage-check = {
            enable = true;
            settings = {
              provider = "agy"; # "openai" or "agy"
              display = "remaining";
              interval = "15m";
              remaining_char = "■";
              used_char = "□";
              icon = "󰫣";
            };
          };
        }
      ];
    };
  };
}
```

### Manual Configuration (`~/.config/agent-usage-check/config.yaml`)

```yaml
provider: agy # openai, agy or opencode-go
display: remaining # remaining or used
interval: 15m
remaining_char: "■"
used_char: "□"
icon: "󰫣"
json: false # set true to output structured JSON
watch: false # set true to continuously stream status updates
cache_path: "~/.config/agent-usage-check/cache.json"
auth_path: "~/.codex/auth.json" # for openai provider
agy_path: "agy" # for agy provider
agy_group: "" # optional filter: "Gemini", "Claude", etc.
opencode_api_key: "" # for opencode-go provider (falls back to OPENCODE_API_KEY, then opencode auth.json)
opencode_auth_path: "" # optional override for opencode auth.json (opencode-go)
opencode_endpoint: "https://opencode.ai/zen/go/v1/usage" # usage endpoint (opencode-go)
```

## CLI Usage

```bash
# Print status bar JSON once and exit (default behavior)
agent-usage-check

# Check Antigravity (agy) usage
agent-usage-check --provider agy

# Check OpenCode Go usage (key via --opencode-api-key, OPENCODE_API_KEY, or `opencode auth login`)
agent-usage-check --provider opencode-go
agent-usage-check --provider opencode-go --details

# Print detailed usage report with progress bars
agent-usage-check --details
agent-usage-check --provider agy --details

# Continuously watch and stream status updates (for status bars / daemons)
agent-usage-check --watch --provider agy

# Print structured agent JSON output
agent-usage-check --provider agy --json
# Output: {"name":"agy","icon":"󰫣","usage":{"1w":64,"5h":10},"alt":"available","groups":[{"1w":64,"5h":10,"name":"Gemini Models"},{"1w":66,"5h":100,"name":"Claude and GPT models"}]}

# Write status JSON atomically to a file
agent-usage-check --provider agy --output ~/.cache/agent-usage-check/status.json --watch

# Print version and exit
agent-usage-check --version

# Print plain fixed-width text instead of JSON (for status bars / scripts)
agent-usage-check --provider opencode-go --text
# Output: "    5hr:  93% 1w:  87%" (icon takes %2s, each value 4 cells; N/A renders as "N/A ")

# Customize the text layout (verbs: icon, 5hr-value, 1w-value)
agent-usage-check --provider opencode-go --text --icon "󴃓"
agent-usage-check --provider opencode-go --text --text-format "%s|%s|%s"
```

## Acknowledgements

Inspired by [doggy8088/codex-reset-checker](https://github.com/doggy8088/codex-reset-checker).

## License

MIT
